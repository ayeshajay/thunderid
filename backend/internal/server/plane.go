// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/attributecache"
	authnAssert "github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	authnOAuth "github.com/thunder-id/thunderid/internal/authn/oauth"
	authnOIDC "github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Plane is what one server binary is. There is no configuration for it and no way to ask a running
// server to become the other one: a binary links the plane it serves and nothing of the plane it
// does not, so the question "which plane is this?" is answered at build time or not at all.
//
// A plane is named for its logs, and given the chance to mount the surfaces that are its own once
// the shared services exist. Whatever it does not mount is not merely refused, it is absent: the
// multiplexer has no handler for a path nobody registered, and answers not-found on its own.
type Plane struct {
	// Name identifies this plane in the startup log.
	Name string

	// StaticApps are the frontend applications this plane serves, by directory name under
	// <serverHome>/apps. A plane that never authenticates an end user has no use for the Gate.
	StaticApps []string

	// Provide runs before the shared services are built, for the few things a plane supplies to
	// them. It is separate from Wire because these are inputs to the shared build rather than
	// surfaces mounted on top of it.
	Provide func(ctx context.Context, mux *http.ServeMux) (Dependencies, error)

	// Wire mounts the surfaces that belong to this plane alone.
	Wire func(ctx context.Context, svcs *Services) error

	// Stop releases whatever this plane started, during graceful shutdown. A plane that holds an
	// outbound connection closes it here rather than leaving the other end to time it out.
	Stop func(ctx context.Context)
}

// SecretCapturer receives a credential that a shared service generated, so a plane which promotes
// configuration elsewhere can keep it. A data plane supplies none: it generates its own secrets and
// is where they belong.
type SecretCapturer interface {
	CaptureSecret(ctx context.Context, resourceType, resourceName, field, value string)
}

// Dependencies are what a plane hands to the shared build. Everything here is optional, and the
// zero value is a plane that supplies nothing.
type Dependencies struct {
	SecretCapturer SecretCapturer

	// SecretStore, when set, is called once the configuration crypto exists and before any
	// declarative resource is loaded, so a secret reference met during loading already resolves. A
	// plane that keeps no secrets leaves it nil, and then nothing in the process serves or reads one.
	SecretStore func(ctx context.Context, mux *http.ServeMux,
		configCrypto kmprovider.ConfigCryptoProvider, deploymentID string)
}

// Services is the seam between the services every plane builds and the surfaces one plane mounts.
//
// It is wide because the runtime is wide, and that is the honest shape: these are exactly the
// services a plane-specific wiring needs and no more. A field appears here only because something
// outside this package asks for it.
type Services struct {
	Mux           *http.ServeMux
	MCPServer     *mcpsdk.Server
	ImportService importer.ImportServiceInterface

	RuntimeCryptoSvc kmprovider.RuntimeCryptoProvider
	ConfigCryptoSvc  kmprovider.ConfigCryptoProvider
	JWTService       jwt.JWTServiceInterface
	JWEService       jwe.JWEServiceInterface

	IDPService         idp.IDPServiceInterface
	AuthnProvider      providers.AuthnProviderManager
	AuthAssertGen      authnAssert.AuthAssertGeneratorInterface
	OTPService         otp.OTPAuthnServiceInterface
	NotifSenderSvc     notification.NotificationSenderServiceInterface
	TemplateService    template.TemplateServiceInterface
	MagicLinkService   magiclink.MagicLinkAuthnServiceInterface
	OAuthAuthnService  authnOAuth.OAuthAuthnServiceInterface
	OIDCAuthnService   authnOIDC.OIDCAuthnServiceInterface
	GoogleAuthnService google.GoogleOIDCAuthnServiceInterface
	GitHubAuthnService github.GithubOAuthAuthnServiceInterface
	DirectAuthSecret   string

	AuthZService           providers.AuthorizationProvider
	EntityProvider         entityprovider.EntityProviderInterface
	ResourceService        resource.ResourceServiceInterface
	ResourceServerProvider providers.ResourceServerProvider
	OUService              ou.OrganizationUnitServiceInterface
	UserService            user.UserServiceInterface
	ApplicationService     application.ApplicationServiceInterface
	I18nService            i18nmgt.I18nServiceInterface

	FlowMgtService      flowmgt.FlowMgtServiceInterface
	ActorProvider       providers.ActorProvider
	ExecRegistry        executor.ExecutorRegistryInterface
	InterceptorRegistry interceptor.InterceptorRegistryInterface
	GraphBuilder        graphbuilder.GraphBuilderInterface
	FlowConfig          flowconfig.Config

	ObservabilitySvc      observability.ObservabilityServiceInterface
	ServerConfigService   serverconfig.ServerConfigService
	RuntimeStoreProvider  providers.RuntimeStoreProvider
	Transactioner         providers.Transactioner
	AttributeCacheService attributecache.AttributeCacheServiceInterface

	DPoPVerifier       dpop.VerifierInterface
	RevocationEnforcer revocation.EnforcementServiceInterface
	RevocationSvc      revocation.RevocationServiceInterface
	SessionService     flowsession.Service

	OpenID4VCICredSvc credential.CredentialConfigurationServiceInterface
	OAuthCfg          oauthconfig.Config
}
