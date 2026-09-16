// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package controlplane designs configuration for gateways to run. It authors documents, versions
// them, and applies them to the data planes it knows about; it never authenticates an end user.
//
// It links none of the runtime: no authorization endpoint, no token issuance, no flow execution.
// Those paths are not guarded on a control plane, they are absent, because nothing here registers
// them.
package controlplane

import (
	"context"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/authored"
	"github.com/thunder-id/thunderid/internal/dataplanetoken"
	"github.com/thunder-id/thunderid/internal/gateway"
	gatewaydataplane "github.com/thunder-id/thunderid/internal/gateway/dataplane"
	"github.com/thunder-id/thunderid/internal/gateway/jobrunner"
	"github.com/thunder-id/thunderid/internal/gateway/secretcapture"
	gatewayservice "github.com/thunder-id/thunderid/internal/gateway/service"
	"github.com/thunder-id/thunderid/internal/gateway/thunder"
	"github.com/thunder-id/thunderid/internal/gatewayvariable"
	"github.com/thunder-id/thunderid/internal/server"
	"github.com/thunder-id/thunderid/internal/system/channel"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// gatewayRegistry is the part of the gateway manager this plane wires up. It is an interface here
// because the manager is built before the services that complete it exist.
type gatewayRegistry interface {
	secretcapture.LocalCaptureRouter
	SetWorkspaceURL(baseURL string)
	SetWorkspaceCA(caFile string)
	SetDataPlanes(planes gatewayservice.DataPlanes)
	SetDataPlaneTokenIssuer(issuer gatewayservice.DataPlaneTokenIssuer)
	SetSecretSealer(sealer gatewayservice.SecretSealer)
	SetGatewayVariables(envVars gatewayvariable.GatewayVariableServiceInterface)
	DeliverPending(ctx context.Context, dataPlaneID string) error
}

// plane holds what this plane builds in two stages: the gateway manager exists before the shared
// services, because they hand it the credentials they generate, and is completed after them.
type plane struct {
	gateways  gatewayRegistry
	variables gatewayvariable.GatewayVariableServiceInterface
}

// Plane describes the control plane to the server that runs it.
//
// It serves the Console and not the Gate: the Gate is where an end user signs in, and end users do
// not sign in to a control plane. Its own operators authenticate against a trusted issuer.
func Plane() server.Plane {
	p := &plane{}
	return server.Plane{
		Name:       "control plane",
		StaticApps: []string{"console"},
		Provide:    p.provide,
		Wire:       p.wire,
	}
}

// provide builds the gateway manager and the variable store the shared services depend on.
//
// Promotion between gateways is a control plane concern, so the manager is wired only here. A
// captured credential is handed straight to the gateway's secret service, which is why it must exist
// before any service that generates one.
func (p *plane) provide(ctx context.Context, mux *http.ServeMux) (server.Dependencies, error) {
	logger := log.GetLogger()

	// This plane fronts many tenants, so a trusted intermediary may name the deployment a request
	// acts for in a header. A gateway serves one deployment and never does, which is why this is
	// declared here rather than read from configuration on both planes.
	security.TrustDeploymentHeaders()

	variables, err := gatewayvariable.Initialize(mux)
	if err != nil {
		return server.Dependencies{}, err
	}
	p.variables = variables

	registry, err := gateway.Initialize(mux)
	if err != nil {
		// A control plane without a gateway manager still authors; it just cannot apply anywhere.
		logger.Error(ctx, "Failed to start the in-process gateway manager", log.Error(err))
		return server.Dependencies{}, nil
	}
	p.gateways = registry
	p.gateways.SetGatewayVariables(variables)

	return server.Dependencies{
		SecretCapturer: secretcapture.Select(ctx, logger, config.GetServerRuntime().Config, p.gateways),
	}, nil
}

// wire mounts the authoring surface and completes the gateway manager now that the shared services
// exist.
func (p *plane) wire(ctx context.Context, svcs *server.Services) error {
	logger := log.GetLogger()
	_ = authored.Initialize(svcs.Mux)

	if p.gateways != nil {
		// A capture reads the organization's workspace, which is this very server, so the manager is
		// told where that answers and which certificate it presents. On premise that certificate is
		// commonly signed by a private CA the system roots do not carry, so it is trusted explicitly.
		runtime := config.GetServerRuntime()
		p.gateways.SetWorkspaceURL(gateway.WorkspaceURL(runtime.Config))
		workspaceCA := gateway.WorkspaceCA(runtime.Config, runtime.ServerHome)
		if err := thunder.CheckCA(workspaceCA); err != nil {
			logger.Error(ctx, "The workspace certificate cannot be trusted, so a capture will fail",
				log.Error(err))
		}
		p.gateways.SetWorkspaceCA(workspaceCA)
	}

	// Tokens are issued per data plane when a gateway is registered and held encrypted here, so the
	// handshake knows which data plane connected rather than believing the id it claims.
	tokens := dataplanetoken.New()
	if p.gateways != nil {
		if svcs.ConfigCryptoSvc != nil {
			// A credential queued for a data plane is encrypted at rest with the configuration key.
			p.gateways.SetSecretSealer(jobrunner.NewConfigSealer(svcs.ConfigCryptoSvc))
		}
	}

	chCfg := config.GetServerRuntime().Config.Channel.Server
	channelServer := channel.InitializeServer(svcs.Mux, channel.ServerConfig{
		Enabled:    chCfg.Enabled,
		Path:       chCfg.Path,
		AuthToken:  chCfg.AuthToken,
		ReadLimit:  chCfg.ReadLimitBytes,
		RPCTimeout: time.Duration(chCfg.RPCTimeoutSeconds) * time.Second,
	}, tokens)

	// Data planes are reached over the connections they hold open to this server, so the manager is
	// given those rather than a client for each one's management API. Without a channel there is no
	// way to reach a data plane at all, and an apply says so rather than failing obscurely.
	if p.gateways != nil {
		p.gateways.SetDataPlanes(gatewaydataplane.New(channelServer))
		p.gateways.SetDataPlaneTokenIssuer(tokens)
		// Work queued by a pod that held no connection to its data plane waits in the database. This
		// is what picks up the share of it this pod can deliver.
		jobrunner.Start(ctx, p.gateways, channelServer)
	}

	return nil
}
