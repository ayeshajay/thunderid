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

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/gateway/model"
)

// CaptureSecretForTenant relays a secret the control plane captured to the secret provider of the one
// gateway the control plane administers directly.
//
// A credential is created once, in the organization's single workspace, but no control plane holds
// one: they live in each data plane's own store. It goes to that gateway alone, because creating
// an application while developing must not reach into production and set the credential running
// there. The others receive theirs when one is set against them deliberately.
//
// It returns how many providers received the secret. Zero with no error means the organization has no
// gateway registered yet, which the caller treats as "nothing to do" rather than a failure:
// secrets created before an gateway exists are recreated on promote.
func (s *Service) CaptureSecretForTenant(ctx context.Context, deploymentID, name string,
	body map[string]interface{}) (int, error) {
	if strings.TrimSpace(deploymentID) == "" || strings.TrimSpace(name) == "" {
		return 0, fmt.Errorf("%w: a deployment id and a secret name are required", ErrValidation)
	}

	envs, err := s.store.ListGateways(ctx)
	if err != nil {
		return 0, err
	}
	target, ok := managedGateway(envs)
	if !ok {
		return 0, nil
	}

	if _, err := s.queueSecret(ctx, target, name, body); err != nil {
		return 0, fmt.Errorf("failed to store the secret for %s: %w", target.Name, err)
	}
	return 1, nil
}

// managedGateway is the gateway the control plane administers directly, which is where a
// credential created in the workspace is issued.
//
// It is the one marked. The organization's first gateway takes the mark as it is created, so a set
// with none marked is one whose marked gateway was removed; the first listed stands in, which keeps a
// credential landing somewhere rather than nowhere.
func managedGateway(envs []model.Gateway) (model.Gateway, bool) {
	for _, env := range envs {
		if env.ManagedByControlPlane {
			return env, true
		}
	}
	if len(envs) == 0 {
		return model.Gateway{}, false
	}
	return envs[0], true
}
