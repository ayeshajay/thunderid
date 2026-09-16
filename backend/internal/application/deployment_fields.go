// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/resourcevalidation"
	"github.com/thunder-id/thunderid/internal/system/varname"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// The fields of an application that belong to the deployment it is applied to, spelled as the
// serialized document spells them. That spelling is what an export derives a variable name from, so
// using the Go field name here would produce a name no captured state ever refers to.
const (
	documentFieldRedirectURIs = "redirectUris"
	documentFieldClientSecret = "clientSecret"
)

// deploymentFields declares which parts of an application belong to the deployment rather than to
// the application being designed.
type deploymentFields struct{}

// ResourceType names the type these fields belong to.
func (deploymentFields) ResourceType() string { return resourceTypeApplication }

// Fill replaces an application's deployment-owned fields with references to what each gateway
// supplies.
//
// A value supplied for one is refused rather than overwritten. Accepting it and quietly replacing it
// would tell the caller their redirect URI was stored when it had been discarded. A reference that
// is already there is left alone, so a document read back and written again round trips.
//
// A reference is placed only where a value would belong: a client that never redirects does not get
// one for a redirect URI, and a public client, which must not hold a secret at all, does not get one
// for a credential. Putting them in regardless would author configuration invalid on every gateway.
func (deploymentFields) Fill(_ context.Context, name string, document map[string]interface{}) error {
	configs, ok := document["inboundAuthConfig"].([]interface{})
	if !ok {
		return nil
	}

	for _, entry := range configs {
		wrapper, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		oauth, ok := wrapper["config"].(map[string]interface{})
		if !ok {
			continue
		}

		if err := fillRedirectURIs(name, oauth); err != nil {
			return err
		}
		if err := fillClientSecret(name, oauth); err != nil {
			return err
		}
	}
	return nil
}

// fillRedirectURIs puts a reference where the redirect URIs would be, on a client that redirects.
func fillRedirectURIs(name string, oauth map[string]interface{}) error {
	supplied, present := oauth[documentFieldRedirectURIs]
	if present && !onlyReferences(supplied) {
		return fmt.Errorf(
			"redirect URIs are not accepted here: they belong to the gateway this application is " +
				"applied to, and a reference is stored in their place")
	}
	if !redirects(oauth) {
		return nil
	}
	oauth[documentFieldRedirectURIs] = []interface{}{
		varname.VariableReference(varname.DeriveVariableName(resourceTypeApplication, name, documentFieldRedirectURIs)),
	}
	return nil
}

// fillClientSecret puts a reference where the credential would be, on a client that holds one.
func fillClientSecret(name string, oauth map[string]interface{}) error {
	supplied, present := oauth[documentFieldClientSecret]
	if present && !isReference(supplied) {
		return fmt.Errorf(
			"a client secret is not accepted here: it belongs to the gateway this application is " +
				"applied to, and a reference is stored in its place")
	}
	if !holdsSecret(oauth) {
		return nil
	}
	oauth[documentFieldClientSecret] = varname.SecretReference(
		varname.DeriveVariableName(resourceTypeApplication, name, documentFieldClientSecret))
	return nil
}

// redirects reports whether this client uses the grant that redirects.
func redirects(oauth map[string]interface{}) bool {
	grants, ok := oauth["grantTypes"].([]interface{})
	if !ok {
		return false
	}
	for _, grant := range grants {
		if text, ok := grant.(string); ok && text == string(providers.GrantTypeAuthorizationCode) {
			return true
		}
	}
	return false
}

// holdsSecret reports whether this client has a credential at all. A public client cannot keep one,
// and "none" states that it does not authenticate with one.
func holdsSecret(oauth map[string]interface{}) bool {
	if public, ok := oauth["publicClient"].(bool); ok && public {
		return false
	}
	method, ok := oauth["tokenEndpointAuthMethod"].(string)
	return !ok || method != string(providers.TokenEndpointAuthMethodNone)
}

// isReference reports whether a value is already a reference.
func isReference(value interface{}) bool {
	text, ok := value.(string)
	return ok && varname.IsReference(text)
}

// onlyReferences reports whether a list holds nothing but references, which is what a document read
// back and written again carries.
func onlyReferences(value interface{}) bool {
	items, ok := value.([]interface{})
	if !ok || len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !isReference(item) {
			return false
		}
	}
	return true
}

// registerDeploymentFields declares which of this type's fields belong to the deployment.
func registerDeploymentFields() error {
	return resourcevalidation.RegisterDeploymentFields(deploymentFields{})
}
