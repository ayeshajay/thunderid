// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/secretgen"
)

// clientSecretGenerator makes an application's client secret.
//
// It lives here rather than in the generator registry because what makes an acceptable client secret
// is an OAuth question, and this package is where the answer already is. The registry only routes.
type clientSecretGenerator struct{}

// ResourceType names the type this generator serves, spelled as the declarative resources spell it.
func (clientSecretGenerator) ResourceType() string { return resourceTypeApplication }

// Generate returns a new client secret. It is not stored here: where a credential is kept is the
// caller's decision, and on a control plane it is written to the gateway that will use it.
func (clientSecretGenerator) Generate(context.Context) (string, error) {
	return oauthutils.GenerateOAuth2ClientSecret()
}

// registerSecretGenerator plugs this resource type's credential rule into the registry.
func registerSecretGenerator() error {
	return secretgen.Register(clientSecretGenerator{})
}
