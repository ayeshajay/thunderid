// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/system/resourcevalidation"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/internal/system/varname"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// sharedRules are the application rules that hold on any plane.
//
// They are the rules about the document rather than about the world: a URL has to look like a URL,
// a client that redirects has to say where to. Nothing here performs a lookup, so a control plane
// designing configuration for elsewhere can apply exactly the rules a gateway will.
//
// A value that is a reference is left alone. On an authoring plane the real value belongs to
// whichever gateway the document is applied to, and checking a reference as though it were a URL
// would reject the document for not carrying something it was deliberately not given. The gateway
// resolves it and checks the result.
type sharedRules struct{}

// ResourceType names the type these rules are for.
func (sharedRules) ResourceType() string { return resourceTypeApplication }

// Validate applies the shared rules to an application payload.
func (sharedRules) Validate(_ context.Context, payload []byte) error {
	var app model.ApplicationDTO
	if err := json.Unmarshal(payload, &app); err != nil {
		return fmt.Errorf("the payload is not an application: %w", err)
	}

	if app.Name == "" {
		return fmt.Errorf("an application name is required")
	}

	for field, value := range map[string]string{
		"url":       app.URL,
		"logoUrl":   app.LogoURL,
		"tosUri":    app.TosURI,
		"policyUri": app.PolicyURI,
	} {
		if err := checkURI(field, value); err != nil {
			return err
		}
	}

	for i := range app.InboundAuthConfig {
		if err := checkOAuth(app.InboundAuthConfig[i].OAuthConfig); err != nil {
			return err
		}
	}
	return nil
}

// checkURI confirms a value looks like a URI, unless it is a reference to one a gateway supplies.
func checkURI(field, value string) error {
	if value == "" || varname.IsReference(value) {
		return nil
	}
	if !sysutils.IsValidURI(value) {
		return fmt.Errorf("%s is not a valid URI", field)
	}
	return nil
}

// checkOAuth applies the consistency rules between a client's grant types and the rest of its
// configuration. These are about the document: they are as true on a control plane as on a gateway.
func checkOAuth(oauth *providers.OAuthConfigWithSecret) error {
	if oauth == nil {
		return nil
	}

	redirects := usesAuthorizationCode(oauth)
	if redirects && len(oauth.RedirectURIs) == 0 {
		return fmt.Errorf("the authorization_code grant requires redirect URIs")
	}
	if !redirects && len(oauth.RedirectURIs) > 0 {
		return fmt.Errorf("redirect URIs are only used by the authorization_code grant")
	}
	for _, redirectURI := range oauth.RedirectURIs {
		if err := checkURI("redirectUris", redirectURI); err != nil {
			return err
		}
	}

	if oauth.PublicClient && oauth.ClientSecret != "" {
		return fmt.Errorf("a public client cannot hold a client secret")
	}
	if oauth.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodNone && oauth.ClientSecret != "" {
		return fmt.Errorf("the 'none' authentication method cannot have a client secret")
	}
	if oauth.PKCERequired && !redirects {
		return fmt.Errorf("PKCE applies only to the authorization_code grant")
	}
	return nil
}

// usesAuthorizationCode reports whether this client redirects.
func usesAuthorizationCode(oauth *providers.OAuthConfigWithSecret) bool {
	for _, grant := range oauth.GrantTypes {
		if grant == providers.GrantTypeAuthorizationCode {
			return true
		}
	}
	return false
}

// registerSharedRules plugs this resource type's plane-independent rules into the registry.
func registerSharedRules() error {
	return resourcevalidation.Register(sharedRules{})
}
