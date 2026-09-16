/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/gatewayvariable"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// consoleVariables are the placeholders the exported Console application fills, named the way the
// exporter derives them from the resource type, name and field.
const (
	consoleURLVariable          = "APPLICATION_CONSOLE_URL"
	consoleRedirectURIsVariable = "APPLICATION_CONSOLE_REDIRECT_URIS"
	consoleClientIDVariable     = "APPLICATION_CONSOLE_CLIENT_ID"
)

// consoleClientID is the client id every Console signs in with. The bootstrap bundle gives the
// Console this fixed id and each plane's console is built to use it, so it is a convention rather
// than an operator's choice. The exporter externalizes client ids as a rule, which leaves this one
// as a placeholder with nobody to fill it, so it is filled here.
const consoleClientID = "CONSOLE"

// setConsoleVariables fills in what a newly created gateway's Console application needs: where it
// serves, and the client id it signs in with.
//
// The Console is the one application a control plane creates for itself, so its URL and redirect URI
// are the control plane's. Applied unchanged they would send a data plane's users to the control
// plane to sign in. The data plane serves its own console, so the gateway records that address
// and an apply resolves the placeholders to it instead.
//
// Only the Console is treated this way. Every other resource is configuration an operator wrote, and
// where its URLs should point is their decision, not this server's.
//
// A gateway registered without an address has nothing to point at, and its Console keeps the control
// plane's URLs until the gateway records one.
func (s *Server) setConsoleVariables(ctx context.Context, envID, dataPlaneURL string) error {
	if s.envVars == nil || strings.TrimSpace(dataPlaneURL) == "" {
		return nil
	}
	console := strings.TrimSuffix(strings.TrimSpace(dataPlaneURL), "/") + "/console"
	// Variables sit in the organization's partition, alongside the gateways they belong to.
	scoped := deployment.WithID(ctx, s.org)

	for key, value := range map[string]string{
		consoleURLVariable: console,
		// An array placeholder is read as a JSON array when it is not supplied as indexed values.
		consoleRedirectURIsVariable: fmt.Sprintf("[%q]", console),
		consoleClientIDVariable:     consoleClientID,
	} {
		_, svcErr := s.envVars.CreateGatewayVariable(scoped, envID,
			gatewayvariable.CreateGatewayVariableRequest{
				Key:         key,
				Value:       value,
				Description: "Set when the gateway was created, so its Console signs in against its own data plane",
			})
		if svcErr != nil {
			return fmt.Errorf("setting %s failed: %s", key, svcErr.ErrorDescription.DefaultValue)
		}
	}
	return nil
}
