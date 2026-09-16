// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package dataplane runs identity for end users: it authenticates them, executes flows, and issues
// tokens. It is the whole product minus the authoring surface, which is why a deployment that wants
// one server runs this one.
//
// Everything reachable only at runtime is linked here and nowhere else. A control plane binary does
// not import this package, so it does not contain an authorization endpoint to protect, an issuer to
// misconfigure, or a token to leak.
package dataplane

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/attestation"
	"github.com/thunder-id/thunderid/internal/authn"
	"github.com/thunder-id/thunderid/internal/authzen"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/oauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dcr"
	"github.com/thunder-id/thunderid/internal/openid4vci"
	"github.com/thunder-id/thunderid/internal/secretstore"
	"github.com/thunder-id/thunderid/internal/server"
	"github.com/thunder-id/thunderid/internal/system/channel"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Plane describes the data plane to the server that runs it.
//
// It serves the Gate, because it is the plane an end user authenticates against, and the Console,
// because a data plane is administered in its own right.
func Plane() server.Plane {
	p := &plane{}
	return server.Plane{
		Name:       "data plane",
		StaticApps: []string{"gate", "console"},
		Provide:    p.provide,
		Wire:       p.wire,
		Stop:       p.stop,
	}
}

// plane holds what this plane builds in two stages: the secret store exists before the shared
// services, and the channel that serves it to a control plane is opened after them.
type plane struct {
	secrets       *secretstore.Store
	channelClient *channel.Client
}

// provide gives the shared build the secret store this plane keeps.
//
// The configuration applied to a gateway refers to credentials by name, and this is where those
// names are answered. A control plane supplies none, so nothing there serves or reads a secret.
func (p *plane) provide(_ context.Context, _ *http.ServeMux) (server.Dependencies, error) {
	// Configuration applied here comes from a control plane, which pushed the credentials it refers
	// to into this deployment's store first. That is what lets an imported credential be written as a
	// value through the ordinary API rather than kept as a reference.
	importer.RunsAsDataPlane()

	return server.Dependencies{
		SecretStore: func(ctx context.Context, mux *http.ServeMux,
			configCrypto kmprovider.ConfigCryptoProvider, deploymentID string) {
			logger := log.GetLogger()
			p.secrets = initSecretStore(ctx, logger, mux, secretProviderConfig(), configCrypto, deploymentID)
			initSecretResolver(ctx, logger, secretProviderConfig(), p.secrets)
		},
	}, nil
}

// secretProviderConfig is the deployment's secret provider configuration.
func secretProviderConfig() engineconfig.SecretProviderConfig {
	return config.GetServerRuntime().Config.Server.SecurityConfig.SecretProvider
}

// wire mounts the runtime surfaces on top of the services every plane builds.
func (p *plane) wire(ctx context.Context, svcs *server.Services) error {
	_, directAuthGuard := authn.Initialize(svcs.Mux, svcs.MCPServer, svcs.IDPService, svcs.JWTService,
		svcs.AuthnProvider, svcs.AuthAssertGen, svcs.OTPService, svcs.NotifSenderSvc,
		svcs.TemplateService, svcs.MagicLinkService, svcs.OAuthAuthnService, svcs.OIDCAuthnService,
		svcs.GoogleAuthnService, svcs.GitHubAuthnService, svcs.DirectAuthSecret)

	// AuthZEN access-evaluation endpoints are Direct API endpoints, so they reuse the Direct Auth
	// guard created by the authn service.
	authzen.Initialize(svcs.Mux, svcs.AuthZService, svcs.EntityProvider, svcs.ResourceService,
		directAuthGuard)

	attestationProvider, err := attestation.Initialize(svcs.RuntimeCryptoSvc)
	if err != nil {
		return fmt.Errorf("failed to initialize attestation provider: %w", err)
	}

	flowExecService, err := flowexec.Initialize(svcs.Mux, svcs.FlowMgtService, svcs.ActorProvider,
		svcs.ExecRegistry, svcs.InterceptorRegistry, svcs.ObservabilitySvc, svcs.RuntimeCryptoSvc,
		attestationProvider, svcs.GraphBuilder, svcs.JWTService, svcs.RuntimeStoreProvider,
		svcs.Transactioner, svcs.ServerConfigService, svcs.FlowConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize flow execution service: %w", err)
	}

	tokenValidator, err := oauth.Initialize(svcs.Mux, svcs.ActorProvider, svcs.AuthnProvider,
		svcs.JWTService, svcs.JWEService, flowExecService, svcs.ObservabilitySvc,
		svcs.RuntimeCryptoSvc, svcs.OUService, svcs.AttributeCacheService, svcs.AuthZService,
		svcs.ResourceServerProvider, svcs.I18nService, svcs.IDPService, svcs.DPoPVerifier,
		svcs.RuntimeStoreProvider, svcs.Transactioner, svcs.RevocationEnforcer, svcs.RevocationSvc,
		svcs.SessionService, svcs.FlowMgtService, svcs.OAuthCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize OAuth services: %w", err)
	}

	// Initialized after the OAuth services because credential issuance validates the presented
	// access token with the OAuth token validator and resolves the wallet application behind it.
	if _, err := openid4vci.Initialize(svcs.Mux, svcs.RuntimeCryptoSvc, tokenValidator,
		svcs.UserService, svcs.DPoPVerifier, svcs.OpenID4VCICredSvc, svcs.ActorProvider,
		svcs.RuntimeStoreProvider); err != nil {
		return fmt.Errorf("failed to initialize OpenID4VCI issuer service: %w", err)
	}

	if svcs.OAuthCfg.OAuth.DCR.IsEnabled() {
		if err := dcr.Initialize(svcs.Mux, svcs.ApplicationService, svcs.OUService, svcs.I18nService,
			svcs.OAuthCfg); err != nil {
			return fmt.Errorf("failed to initialize OAuth2 DCR service: %w", err)
		}
	}

	// This gateway dials the control plane and holds the connection open. The control plane reaches
	// it over that connection, so this server needs no inbound route and issues no credential for its
	// own management API. The secret store is served over the same channel, so a captured credential
	// arrives here without the control plane keeping one.
	p.channelClient = initChannelClient(svcs.ImportService, p.secrets)
	if p.channelClient != nil {
		p.channelClient.Start(ctx)
	}

	return nil
}

// stop closes the channel this plane dialed, so the control plane sees it go rather than waiting
// for the connection to time out.
func (p *plane) stop(_ context.Context) {
	if p.channelClient != nil {
		p.channelClient.Stop()
	}
}

// initChannelClient builds the control plane channel client from configuration, or returns nil when
// it is disabled.
func initChannelClient(importService importer.ImportServiceInterface,
	localSecrets *secretstore.Store) *channel.Client {
	c := config.GetServerRuntime().Config.Channel.Client
	return channel.InitializeClient(channel.ClientConfig{
		Enabled:          c.Enabled,
		ID:               c.ID,
		Instance:         c.Instance,
		ControlPlaneURL:  c.ControlPlaneURL,
		AuthToken:        c.AuthToken,
		CAFile:           c.CAFile,
		ReadLimit:        c.ReadLimitBytes,
		PingInterval:     time.Duration(c.PingIntervalSeconds) * time.Second,
		ReconnectInitial: time.Duration(c.ReconnectInitialSeconds) * time.Second,
		ReconnectMax:     time.Duration(c.ReconnectMaxSeconds) * time.Second,
	}, importService, secretStoreOrNil(localSecrets))
}

// secretStoreOrNil keeps a nil store nil rather than a non-nil interface holding a nil pointer,
// which would register channel handlers that panic.
func secretStoreOrNil(store *secretstore.Store) channel.SecretStore {
	if store == nil {
		return nil
	}
	return store
}
