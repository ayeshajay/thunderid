// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package server builds every service this product has and runs one plane of it. Which plane is
// decided by the binary that calls Run, not by configuration: see Plane.
package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/mcp"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/revocationcache"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// shutdownTimeout defines the timeout duration for graceful shutdown.
const shutdownTimeout = 5 * time.Second

var (
	netListen = net.Listen
	tlsListen = tls.Listen
)

// Run starts one plane of the server and blocks until it is asked to shut down.
//
// Everything up to the plane's own wiring is the same on both: the same configuration, the same
// services, the same configuration management APIs. What differs is what the plane mounts, and a
// binary calls this with the one plane it was built to be.
func Run(p Plane) {
	// Server bootstrap/shutdown logging has no request scope, so context.Background() is used.
	ctx := context.Background()
	startupStartedAt := time.Now()
	logger := log.GetLogger()

	flag.String("resources", "", "Path to declarative resources YAML file")
	serverHome := getThunderHome(ctx, logger)

	cfg := initThunderConfigurations(ctx, logger, serverHome)
	if cfg == nil {
		logger.Fatal(ctx, "Failed to initialize configurations")
	}

	// Apply the configured log level from deployment.yaml, now that config is loaded.
	if cfg.Log.Level != "" {
		if err := logger.SetLevel(cfg.Log.Level); err != nil {
			logger.Fatal(ctx, "Invalid log level in configuration", log.Error(err))
		}
	}

	// Apply the configured log output (console and/or rotating file), now that the
	// server home is known and the file path can be resolved.
	if err := logger.Configure(cfg.Log.BuildOutputOptions(serverHome)); err != nil {
		logger.Fatal(ctx, "Failed to configure log output", log.Error(err))
	}

	// Initialize the cache manager.
	cacheManager := cache.Initialize(cfg.Cache, cfg.Server.Identifier)

	// Initialize system permission strings before any service or middleware uses them.
	security.InitSystemPermissions(cfg.Server.SecurityConfig.SystemPermissionPrefix)

	// Create a new HTTP multiplexer.
	mux := http.NewServeMux()
	if mux == nil {
		logger.Fatal(ctx, "Failed to initialize multiplexer")
	}

	// A plane supplies its own inputs to the shared build before that build runs.
	var deps Dependencies
	if p.Provide != nil {
		provided, err := p.Provide(ctx, mux)
		if err != nil {
			logger.Fatal(ctx, "Failed to build the plane's own services",
				log.String("plane", p.Name), log.Error(err))
		}
		deps = provided
	}

	// Build every service, and register the surfaces that belong to no single plane.
	svcs := registerServices(mux, cacheManager, deps)

	// When invoked as the bootstrap one-shot (`thunderid bootstrap`), create the
	// default resources in-process and exit without starting the HTTP server.
	if isBootstrapInvocation() {
		if err := runBootstrap(ctx, logger, serverHome, svcs.ImportService, cacheManager); err != nil {
			logger.Error(ctx, "In-process bootstrap failed; exiting", log.Error(err))
			os.Exit(1)
		}
		logger.Info(ctx, "In-process bootstrap finished successfully")
		return
	}

	// Mount the surfaces that are this plane's own. A path no plane mounts is not registered on the
	// multiplexer at all, so it is answered with a not-found by net/http rather than refused by a
	// guard: on this plane the path genuinely does not exist.
	if err := p.Wire(ctx, svcs); err != nil {
		logger.Fatal(ctx, "Failed to wire the plane", log.String("plane", p.Name), log.Error(err))
	}

	// Initialize the Resource Server token-revocation cache. The initial deny-list snapshot is loaded
	// synchronously so enforcement is live before the first request; if that load fails the server
	// still starts and the syncer repopulates the cache on its next tick.
	revocationEnforcer, revocationSyncer := initRevocationCache(ctx, logger, cfg)
	revocationSyncer.Start(ctx)

	// Mount the MCP server's routes now that the revocation enforcer exists — DefaultGuard uses it
	// to authenticate MCP requests with the same verification and revocation logic as the REST gate.
	mcpGuard, mcpResourceMeta := mcp.DefaultGuard(svcs.JWTService, revocationEnforcer)
	mcp.Initialize(mux, svcs.MCPServer, mcpGuard, mcpResourceMeta)

	// Register static file handlers for frontend applications.
	registerStaticFileHandlers(ctx, logger, mux, serverHome, p.StaticApps)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create the HTTP server.
	server := createHTTPServer(ctx, logger, cfg, mux, svcs.JWTService, revocationEnforcer)
	var ln net.Listener
	if cfg.Server.HTTPOnly {
		logger.Info(ctx, "TLS is not enabled, starting server without TLS")
		ln = createListener(ctx, logger, server)
	} else {
		tlsConfigProvider, ok := svcs.RuntimeCryptoSvc.(common.TLSConfigProvider)
		if !ok {
			logger.Fatal(ctx, "Runtime crypto provider does not support TLS material retrieval")
		}
		tlsConfig := loadCertConfig(ctx, logger, tlsConfigProvider)
		ln = createTLSListener(ctx, logger, server, tlsConfig)
	}

	serverURL := config.GetServerURL(&cfg.Server)
	consoleURL := fmt.Sprintf("%s/console", strings.TrimSuffix(serverURL, "/"))
	logger.Info(ctx, "ThunderID plane", log.String("plane", p.Name))
	logger.Info(ctx, "ThunderID Server URL", log.String("url", serverURL))
	logger.Info(ctx, "ThunderID Console URL", log.String("url", consoleURL))

	// Start server in a goroutine
	go func() {
		startupDuration := time.Since(startupStartedAt)
		logger.Info(ctx, "ThunderID Server started", log.String("startup_time", startupDuration.String()))
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "Failed to serve requests", log.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info(ctx, "Shutting down server...")
	gracefulShutdown(ctx, logger, server, cacheManager, revocationSyncer, p)
}

// initRevocationCache builds the Resource Server token-revocation enforcer and its background syncer
// from the server security configuration. An unsupported source configuration fails startup; a
// failed initial deny-list load does not — the server starts and the syncer populates the cache later.
func initRevocationCache(ctx context.Context, logger *log.Logger,
	cfg *config.Config) (revocationcache.EnforcerInterface, revocationcache.Syncer) {
	rc := cfg.Server.SecurityConfig.TokenRevocation
	enforcer, syncer, err := revocationcache.Initialize(revocationcache.Config{
		Enabled:      rc.IsEnabled(),
		Source:       rc.Source,
		SyncInterval: time.Duration(rc.SyncIntervalSeconds) * time.Second,
	})
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize token revocation cache", log.Error(err))
	}
	return enforcer, syncer
}

// getThunderHome retrieves and return the home directory.
func getThunderHome(ctx context.Context, logger *log.Logger) string {
	// Parse project directory from command line arguments.
	projectHome := ""
	projectHomeFlag := flag.String("serverHome", "", "Path to ThunderID home directory")
	flag.Parse()

	if *projectHomeFlag != "" {
		logger.Info(ctx, "Using serverHome from command line argument",
			log.String("serverHome", *projectHomeFlag))
		projectHome = *projectHomeFlag
	} else {
		// If no command line argument is provided, use the current working directory.
		dir, dirErr := os.Getwd()
		if dirErr != nil {
			logger.Fatal(ctx, "Failed to get current working directory", log.Error(dirErr))
		}
		projectHome = dir
	}

	return projectHome
}

// initThunderConfigurations initializes the configurations.
func initThunderConfigurations(ctx context.Context, logger *log.Logger, serverHome string) *config.Config {
	// Load the configurations.
	configFilePath := path.Join(serverHome, "deployment.yaml")
	defaultConfigPath := path.Join(serverHome, "config/default.json")
	cfg, err := config.LoadConfig(configFilePath, defaultConfigPath, serverHome)
	if err != nil {
		logger.Fatal(ctx, "Failed to load configurations", log.Error(err))
	}

	// Initialize runtime configurations.
	if err := config.InitializeServerRuntime(serverHome, cfg); err != nil {
		logger.Fatal(ctx, "Failed to initialize server runtime", log.Error(err))
	}

	return cfg
}

// loadCertConfig loads the TLS material via the runtime crypto provider.
func loadCertConfig(ctx context.Context, logger *log.Logger, runtimeSvc common.TLSConfigProvider) *tls.Config {
	mat, err := runtimeSvc.GetTLSMaterial(ctx)
	if err != nil {
		logger.Fatal(ctx, "Failed to load TLS material", log.Error(err))
	}
	// #nosec G402 -- MinVersion is set to TLS 1.2 or higher by GetTLSMaterial
	return &tls.Config{
		Certificates: []tls.Certificate{mat.Certificate},
		MinVersion:   mat.MinVersion,
	}
}

// accessLogExcludePaths returns the path prefixes excluded from the access log: the always-excluded
// Gate and Console frontend paths plus any configured extras. Empty and root ("/") entries are
// dropped because they match every request and would silently disable all access logging.
func accessLogExcludePaths(configured []string) []string {
	paths := []string{"/gate/", "/console/"}
	for _, prefix := range configured {
		if prefix != "" && prefix != "/" {
			paths = append(paths, prefix)
		}
	}
	return paths
}

// createHTTPServer creates and configures an HTTP server with common settings.
func createHTTPServer(ctx context.Context, logger *log.Logger, cfg *config.Config, mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface, revocationEnforcer revocationcache.EnforcerInterface) *http.Server {
	securityMiddleware := createSecurityMiddleware(ctx, logger, mux, jwtService, revocationEnforcer)

	// Build the middleware chain with proper execution order.
	// Request flow: CorrelationID (outermost) -> DeploymentID -> SecurityHeaders -> AccessLog ->
	// Security -> Route Handler (innermost)
	// Note: Middlewares are wrapped in reverse order - the last added will execute first.
	// The Gate and Console frontend paths are always excluded from the access log to keep it
	// focused on API traffic. Additional prefixes can be excluded via log.access.exclude_paths.
	handler := log.AccessLogHandler(logger, accessLogExcludePaths(cfg.Log.Access.ExcludePaths), securityMiddleware)
	handler = middleware.SecurityHeadersMiddleware()(handler)
	// Outside the security layer, so that every request carries the deployment id it acts for by the
	// time any store is reached.
	handler = middleware.DeploymentIDMiddleware(handler)
	handler = middleware.CorrelationIDMiddleware(handler)

	// Build the server address using hostname and port from the configurations.
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Hostname, cfg.Server.Port)

	server := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Mitigate Slowloris attacks
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          log.NewServerErrorLog(logger),
	}

	return server
}

// createListener creates and returns a listener for the HTTP server.
func createListener(ctx context.Context, logger *log.Logger, server *http.Server) net.Listener {
	ln, err := netListen("tcp", server.Addr)
	if err != nil {
		logger.Fatal(ctx, "Failed to start HTTP listener", log.Error(err))
	}
	return ln
}

// createTLSListener creates and returns a TLS listener for the HTTPS server.
func createTLSListener(ctx context.Context, logger *log.Logger, server *http.Server,
	tlsConfig *tls.Config) net.Listener {
	ln, err := tlsListen("tcp", server.Addr, tlsConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to start TLS listener", log.Error(err))
	}
	return ln
}

func createSecurityMiddleware(ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface, revocationEnforcer revocationcache.EnforcerInterface) http.Handler {
	middlewareFunc, err := security.Initialize(jwtService, revocationEnforcer)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize security middleware", log.Error(err))
	}
	return middlewareFunc(mux)
}

// gracefulShutdown handles the graceful shutdown of all components.
func gracefulShutdown(
	ctx context.Context,
	logger *log.Logger,
	server *http.Server,
	cacheManager cache.CacheManagerInterface,
	revocationSyncer revocationcache.Syncer,
	p Plane,
) {
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Error during server shutdown", log.Error(err))
	} else {
		logger.Debug(ctx, "HTTP server shutdown completed")
	}

	// Stop the token-revocation cache syncer.
	revocationSyncer.Stop()

	// Let the plane release what it started, before the services it depends on are torn down.
	if p.Stop != nil {
		p.Stop(ctx)
	}

	// Shutdown services
	unregisterServices()

	// Close database connections
	dbCloser := provider.GetDBProviderCloser()
	if err := dbCloser.Close(); err != nil {
		logger.Error(ctx, "Error closing database connections", log.Error(err))
	} else {
		logger.Debug(ctx, "Database connections closed successfully")
	}

	if cacheManager != nil {
		cacheManager.Close()
		logger.Debug(ctx, "Cache manager closed successfully")
	}

	// Close the log file writer before the final shutdown log line.
	if err := logger.Close(); err != nil {
		logger.Error(ctx, "Error closing log file", log.Error(err))
	}

	logger.Info(ctx, "Server shutdown completed")
}

// registerStaticFileHandlers registers static file handlers for the frontend applications this
// plane serves. An application this plane has no use for is not mounted, and its directory is not
// looked for.
func registerStaticFileHandlers(ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	serverHome string, apps []string) {
	// Override the OS-level MIME mapping so .js/.mjs files are served as
	// application/javascript. Most proxies (Envoy, NGINX, Cloudflare) only
	// compress application/javascript in their default allowlists, not
	// text/javascript, which is Go's default on some systems.
	_ = mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "application/javascript; charset=utf-8")

	for _, app := range apps {
		route := "/" + app + "/"
		dir := path.Join(serverHome, "apps", app)
		handler, err := createStaticFileHandler(route, dir, logger)
		if err != nil {
			logger.Warn(ctx, "Frontend application not registered", log.String("application", app),
				log.String("directory", dir), log.Error(err))
			continue
		}
		logger.Debug(ctx, "Registering static file handler for frontend application",
			log.String("application", app), log.String("path", route), log.String("directory", dir))
		mux.Handle(route, handler)
	}
}

// createStaticFileHandler creates a handler for serving static files with SPA fallback.
//
// All filesystem access is performed through an os.Root anchored at `directory`, which
// confines every lookup below that directory: any request path that would escape it fails
// with a path-escape error rather than reaching the filesystem.
func createStaticFileHandler(routePrefix, directory string, logger *log.Logger) (http.Handler, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	rootFS := root.FS()
	fileServer := http.FileServerFS(rootFS)

	// serveIndex serves index.html with no-cache headers, substituting the request's CSP nonce
	// (see SecurityHeadersMiddleware) for the placeholder in its <meta property="csp-nonce"> tag, so
	// the frontend can apply the same nonce to its own inline <style> tags. It reports whether
	// index.html existed and was served.
	serveIndex := func(w http.ResponseWriter, r *http.Request) bool {
		content, err := fs.ReadFile(rootFS, "index.html")
		if err != nil {
			return false
		}
		nonce := sysContext.GetCSPNonce(r.Context())
		content = bytes.ReplaceAll(content, []byte(constants.CSPNoncePlaceholder), []byte(nonce))

		w.Header().Set(constants.CacheControlHeaderName, constants.CacheControlNoCacheComposite)
		w.Header().Set(constants.PragmaHeaderName, constants.PragmaNoCache)
		w.Header().Set(constants.ExpiresHeaderName, constants.ExpiresZero)
		w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeHTML)
		_, _ = w.Write(content)
		return true
	}

	return http.StripPrefix(routePrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle the application root by explicitly serving index.html.
		if r.URL.Path == "/" || r.URL.Path == "" {
			if serveIndex(w, r) {
				return
			}
		}

		// Resolve the request against the served directory.
		name := strings.TrimPrefix(r.URL.Path, "/")
		isIndexHTML := name == "index.html"

		if name != "" {
			if _, err := root.Stat(name); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					// For SPA routing, serve index.html for non-existent in-bounds paths.
					logger.Debug(r.Context(), "Serving index.html for SPA routing",
						log.String("requested_path", r.URL.Path),
						log.String("route_prefix", routePrefix))
					if serveIndex(w, r) {
						return
					}
				} else {
					// The path escapes the served directory (or is otherwise invalid).
					logger.Warn(r.Context(), "Rejected request with out-of-bounds path",
						log.String("requested_path", r.URL.Path),
						log.String("route_prefix", routePrefix))
					http.NotFound(w, r)
					return
				}
			}
		}

		// Serve index.html directly with no-cache headers when requested.
		if isIndexHTML {
			if serveIndex(w, r) {
				return
			}
		}

		// Serve the requested file or directory listing.
		fileServer.ServeHTTP(w, r)
	})), nil
}
