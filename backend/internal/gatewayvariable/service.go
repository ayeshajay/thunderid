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

package gatewayvariable

import (
	"context"
	"errors"
	"regexp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const gatewayVariableLoggerComponentName = "GatewayVariableService"

// gatewayVariableKeyPattern restricts keys to valid gateway-variable names so they can back
// the declarative placeholders (for example MY_APP_REDIRECT_URL) they resolve.
var gatewayVariableKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// GatewayVariableServiceInterface defines gateway variable management operations.
type GatewayVariableServiceInterface interface {
	CreateGatewayVariable(ctx context.Context, envID string,
		request CreateGatewayVariableRequest) (*GatewayVariable, *tidcommon.ServiceError)
	GetGatewayVariable(ctx context.Context, envID,
		id string) (*GatewayVariable, *tidcommon.ServiceError)
	GetGatewayVariableList(ctx context.Context, envID string, limit,
		offset int) (*GatewayVariableListResponse, *tidcommon.ServiceError)
	UpdateGatewayVariable(ctx context.Context, envID, id string,
		request UpdateGatewayVariableRequest) (*GatewayVariable, *tidcommon.ServiceError)
	DeleteGatewayVariable(ctx context.Context, envID, id string) *tidcommon.ServiceError
	// ResolveGatewayVariables returns every key mapped to its value for one gateway. Used by
	// the config export/apply tooling to substitute declarative placeholders for a Data Plane.
	ResolveGatewayVariables(ctx context.Context,
		envID string) (map[string]string, *tidcommon.ServiceError)
}

// gatewayVariableService is the default implementation of GatewayVariableServiceInterface.
type gatewayVariableService struct {
	store gatewayVariableStoreInterface
}

// newGatewayVariableService creates a new gatewayVariableService.
func newGatewayVariableService(store gatewayVariableStoreInterface) GatewayVariableServiceInterface {
	return &gatewayVariableService{store: store}
}

// CreateGatewayVariable validates and stores a new gateway variable.
func (s *gatewayVariableService) CreateGatewayVariable(ctx context.Context, envID string,
	request CreateGatewayVariableRequest) (*GatewayVariable, *tidcommon.ServiceError) {
	if !gatewayVariableKeyPattern.MatchString(request.Key) {
		return nil, &ErrorInvalidGatewayVariableRequest
	}

	existing, err := s.store.GetGatewayVariableByKey(ctx, envID, request.Key)
	if err != nil && !errors.Is(err, errGatewayVariableNotFound) {
		return nil, s.internalError(ctx, "failed to check gateway variable key uniqueness", err)
	}
	if err == nil && existing.ID != "" {
		return nil, &ErrorGatewayVariableKeyConflict
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return nil, s.internalError(ctx, "failed to generate gateway variable id", err)
	}

	created := GatewayVariable{
		ID:          id,
		Key:         request.Key,
		Value:       request.Value,
		Description: request.Description,
	}
	if err := s.store.CreateGatewayVariable(ctx, envID, created); err != nil {
		return nil, s.internalError(ctx, "failed to create gateway variable", err)
	}

	return &created, nil
}

// GetGatewayVariable returns an gateway variable by id, including its value.
func (s *gatewayVariableService) GetGatewayVariable(ctx context.Context,
	envID, id string) (*GatewayVariable, *tidcommon.ServiceError) {
	stored, err := s.store.GetGatewayVariableByID(ctx, envID, id)
	if err != nil {
		if errors.Is(err, errGatewayVariableNotFound) {
			return nil, &ErrorGatewayVariableNotFound
		}
		return nil, s.internalError(ctx, "failed to get gateway variable", err)
	}
	return &stored, nil
}

// GetGatewayVariableList returns a paginated list of gateway variables.
func (s *gatewayVariableService) GetGatewayVariableList(ctx context.Context, envID string,
	limit, offset int) (*GatewayVariableListResponse, *tidcommon.ServiceError) {
	total, err := s.store.GetGatewayVariableCount(ctx, envID)
	if err != nil {
		return nil, s.internalError(ctx, "failed to count gateway variables", err)
	}

	variables, err := s.store.GetGatewayVariableList(ctx, envID, limit, offset)
	if err != nil {
		return nil, s.internalError(ctx, "failed to list gateway variables", err)
	}

	return &GatewayVariableListResponse{
		TotalResults:     total,
		Count:            len(variables),
		GatewayVariables: variables,
	}, nil
}

// UpdateGatewayVariable updates an gateway variable's value and description.
func (s *gatewayVariableService) UpdateGatewayVariable(ctx context.Context, envID, id string,
	request UpdateGatewayVariableRequest) (*GatewayVariable, *tidcommon.ServiceError) {
	err := s.store.UpdateGatewayVariableByID(ctx, envID, id, request.Description, request.Value)
	if err != nil {
		if errors.Is(err, errGatewayVariableNotFound) {
			return nil, &ErrorGatewayVariableNotFound
		}
		return nil, s.internalError(ctx, "failed to update gateway variable", err)
	}

	return s.GetGatewayVariable(ctx, envID, id)
}

// DeleteGatewayVariable removes an gateway variable by id.
func (s *gatewayVariableService) DeleteGatewayVariable(ctx context.Context,
	envID, id string) *tidcommon.ServiceError {
	if err := s.store.DeleteGatewayVariableByID(ctx, envID, id); err != nil {
		if errors.Is(err, errGatewayVariableNotFound) {
			return &ErrorGatewayVariableNotFound
		}
		return s.internalError(ctx, "failed to delete gateway variable", err)
	}
	return nil
}

// ResolveGatewayVariables returns every key mapped to its value for one gateway.
func (s *gatewayVariableService) ResolveGatewayVariables(ctx context.Context,
	envID string) (map[string]string, *tidcommon.ServiceError) {
	values, err := s.store.GetGatewayVariableValues(ctx, envID)
	if err != nil {
		return nil, s.internalError(ctx, "failed to read gateway variable values", err)
	}
	return values, nil
}

// internalError logs the underlying error and returns the generic server-side ServiceError.
func (s *gatewayVariableService) internalError(ctx context.Context, msg string,
	err error) *tidcommon.ServiceError {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, gatewayVariableLoggerComponentName))
	logger.Error(ctx, msg, log.Error(err))
	return &ErrorInternalServer
}
