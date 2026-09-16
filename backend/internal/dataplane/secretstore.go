// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dataplane

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/secretstore"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// initSecretResolver installs the process-wide secret resolver and loads its cache.
//
// A load failure is logged rather than fatal: the resolver fetches a name on demand, so the server can
// start and recover once the provider is reachable. A deployment with no provider configured installs a
// disabled resolver, which leaves every stored value untouched.
func initSecretResolver(ctx context.Context, logger *log.Logger, cfg engineconfig.SecretProviderConfig,
	local *secretstore.Store) {
	resolverCfg := secretresolver.Config{
		BaseURL: cfg.Service.URL,
		Token:   cfg.Service.Token,
		Timeout: time.Duration(cfg.Service.TimeoutSeconds) * time.Second,
	}
	// This server serves the store itself, so it reads it directly. Going back out over HTTP would
	// mean presenting a token for its own management API, which it has no way to mint.
	//
	// The store is consulted per reference rather than copied into the resolver. It is already an
	// in-memory cache that a control plane's writes land in and that reloads from a shared key vault
	// on its own schedule, so a second copy here would keep serving a credential that has since been
	// regenerated, and every login against it would fail until this process restarted.
	if local != nil {
		resolverCfg.Local = func(ctx context.Context, name string) (secretresolver.LocalSecret, bool, error) {
			secret, found := local.Get(ctx, name)
			if !found {
				return secretresolver.LocalSecret{}, false, nil
			}
			return secretresolver.LocalSecret{
				Kind:        string(secret.Kind),
				Value:       secret.Value,
				Algorithm:   secret.Algorithm,
				Salt:        secret.Parameters.Salt,
				Iterations:  secret.Parameters.Iterations,
				KeySize:     secret.Parameters.KeySize,
				Memory:      secret.Parameters.Memory,
				Parallelism: secret.Parameters.Parallelism,
			}, true, nil
		}
	}
	resolver := secretresolver.New(resolverCfg)
	secretresolver.SetDefault(resolver)

	if !resolver.Enabled() {
		return
	}
	// A resolver reading this server's own store preloads nothing: it reads that store per reference,
	// which is what keeps a regenerated credential from going stale here.
	if local != nil {
		return
	}
	if err := resolver.LoadAll(ctx); err != nil {
		logger.Warn(ctx, "Failed to load secrets from the secret provider; they will be fetched on demand",
			log.Error(err))
		return
	}
	// Count only: logging a secret name or value would defeat the point of the provider.
	logger.Info(ctx, "Loaded secrets from the secret provider", log.Int("count", resolver.Count()))
}

// initSecretStore serves the secret store from this process, backed by whatever the configured mode
// asks for. A mode that keeps no store here (an empty mode, or reading from the standalone provider
// service) returns nil, and this server serves no store.
//
// A misconfiguration is logged rather than fatal: the deployment may also have a standalone provider,
// and a server that cannot serve its own store is still able to read from that one.
func initSecretStore(ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	cfg engineconfig.SecretProviderConfig, configCrypto kmprovider.ConfigCryptoProvider,
	deploymentID string) *secretstore.Store {
	store, err := secretstore.Initialize(ctx, mux, secretStoreConfig(cfg, configCrypto, deploymentID))
	if store == nil {
		if err != nil {
			logger.Error(ctx, "Failed to start the secret store", log.Error(err))
		}
		return nil
	}
	// The store exists but its backing could not be read. It retries, so this is a warning rather
	// than a reason to serve nothing: a key vault that is briefly unreachable at startup should not
	// leave the server permanently without its secrets.
	if err != nil {
		logger.Warn(ctx, "The secret store could not be read yet; it will be retried",
			log.String("backend", store.Backend()), log.Error(err))
	}
	logger.Info(ctx, "Serving the secret store from this server",
		log.String("backend", store.Backend()), log.Int("count", store.Count()))
	return store
}

// secretStoreConfig maps the deployment configuration onto the store's own, so the store package
// depends on no configuration types.
func secretStoreConfig(cfg engineconfig.SecretProviderConfig,
	configCrypto kmprovider.ConfigCryptoProvider, deploymentID string) secretstore.Config {
	// A data plane holds the credentials the configuration applied to it refers to, so it keeps a
	// store whether or not one was configured. The database is the default because every instance of
	// the deployment shares it. This is decided here rather than in the store package because it is a
	// per-plane decision: a control plane keeps no store at all.
	mode := secretstore.Mode(cfg.Mode)
	if strings.TrimSpace(string(mode)) == "" {
		mode = secretstore.ModeDB
	}
	return secretstore.Config{
		Mode:     mode,
		FilePath: cfg.File.Path,
		DB: secretstore.DBConfig{
			Provider:     dbprovider.GetDBProvider(),
			Sealer:       configSecretSealer{crypto: configCrypto},
			DeploymentID: deploymentID,
		},
		KV: secretstore.KVConfig{
			Type:       cfg.KV.Type,
			Address:    cfg.KV.Address,
			Mount:      cfg.KV.Mount,
			PathPrefix: cfg.KV.PathPrefix,
			Namespace:  cfg.KV.Namespace,
			Token:      cfg.KV.Token,
			CAFile:     cfg.KV.CAFile,
			Timeout:    time.Duration(cfg.KV.TimeoutSeconds) * time.Second,
			CacheTTL:   time.Duration(cfg.KV.CacheTTLSeconds) * time.Second,
		},
	}
}

// configSecretSealer adapts the server's configuration crypto to what the secret store needs. A stored
// credential is encrypted with the same key the rest of the server's configuration secrets use, so
// there is one key to manage rather than one per subsystem.
type configSecretSealer struct {
	crypto kmprovider.ConfigCryptoProvider
}

// Seal encrypts a credential for storage.
func (s configSecretSealer) Seal(ctx context.Context, plaintext []byte) ([]byte, error) {
	return s.crypto.Encrypt(ctx, plaintext)
}

// Open decrypts a stored credential.
func (s configSecretSealer) Open(ctx context.Context, sealed []byte) ([]byte, error) {
	return s.crypto.Decrypt(ctx, sealed)
}
