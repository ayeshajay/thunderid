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
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// gatewayVariableStoreInterface defines persistence operations for gateway variables.
type gatewayVariableStoreInterface interface {
	CreateGatewayVariable(ctx context.Context, envID string, ev GatewayVariable) error
	GetGatewayVariableCount(ctx context.Context, envID string) (int, error)
	GetGatewayVariableList(ctx context.Context, envID string, limit,
		offset int) ([]GatewayVariable, error)
	GetGatewayVariableByID(ctx context.Context, envID, id string) (GatewayVariable, error)
	GetGatewayVariableByKey(ctx context.Context, envID, key string) (GatewayVariable, error)
	UpdateGatewayVariableByID(ctx context.Context, envID, id, description, value string) error
	DeleteGatewayVariableByID(ctx context.Context, envID, id string) error
	GetGatewayVariableValues(ctx context.Context, envID string) (map[string]string, error)
}

// gatewayVariableStore is the default DB-backed implementation of
// gatewayVariableStoreInterface.
type gatewayVariableStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newGatewayVariableStore creates a new gatewayVariableStore bound to the gateway
// database, which it shares with the gateway manager the variables are resolved for.
func newGatewayVariableStore() gatewayVariableStoreInterface {
	return &gatewayVariableStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// CreateGatewayVariable inserts a new gateway variable row.
func (s *gatewayVariableStore) CreateGatewayVariable(ctx context.Context, envID string,
	ev GatewayVariable) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.QueryContext(ctx, queryCreateGatewayVariable,
		ev.ID, envID, ev.Key, ev.Value, ev.Description, s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to create gateway variable: %w", err)
	}
	return nil
}

// GetGatewayVariableCount returns the total number of gateway variables for the deployment.
func (s *gatewayVariableStore) GetGatewayVariableCount(ctx context.Context,
	envID string) (int, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetGatewayVariableCount,
		envID, s.scope(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	if len(results) > 0 {
		if count, ok := results[0]["total"].(int64); ok {
			return int(count), nil
		}
		return 0, fmt.Errorf("failed to parse count result")
	}
	return 0, nil
}

// GetGatewayVariableList returns a paginated list of gateway variables.
func (s *gatewayVariableStore) GetGatewayVariableList(ctx context.Context, envID string,
	limit, offset int) ([]GatewayVariable, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetGatewayVariableList,
		limit, offset, envID, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	variables := make([]GatewayVariable, 0, len(results))
	for _, row := range results {
		variables = append(variables, parseGatewayVariable(row))
	}
	return variables, nil
}

// GetGatewayVariableByID returns a single gateway variable by id.
func (s *gatewayVariableStore) GetGatewayVariableByID(ctx context.Context,
	envID, id string) (GatewayVariable, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return GatewayVariable{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetGatewayVariableByID, id, envID,
		s.scope(ctx))
	if err != nil {
		return GatewayVariable{}, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return GatewayVariable{}, errGatewayVariableNotFound
	}
	return parseGatewayVariable(results[0]), nil
}

// GetGatewayVariableByKey returns a single gateway variable by key.
func (s *gatewayVariableStore) GetGatewayVariableByKey(ctx context.Context,
	envID, key string) (GatewayVariable, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return GatewayVariable{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetGatewayVariableByKey, key, envID,
		s.scope(ctx))
	if err != nil {
		return GatewayVariable{}, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return GatewayVariable{}, errGatewayVariableNotFound
	}
	return parseGatewayVariable(results[0]), nil
}

// UpdateGatewayVariableByID updates an gateway variable's description and value.
func (s *gatewayVariableStore) UpdateGatewayVariableByID(ctx context.Context, envID, id,
	description, value string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryUpdateGatewayVariableByID,
		description, value, id, envID, s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to update gateway variable: %w", err)
	}
	if rowsAffected == 0 {
		return errGatewayVariableNotFound
	}
	return nil
}

// DeleteGatewayVariableByID deletes an gateway variable by id.
func (s *gatewayVariableStore) DeleteGatewayVariableByID(ctx context.Context,
	envID, id string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeleteGatewayVariableByID, id, envID,
		s.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to delete gateway variable: %w", err)
	}
	if rowsAffected == 0 {
		return errGatewayVariableNotFound
	}
	return nil
}

// GetGatewayVariableValues returns every gateway variable key mapped to its value for one
// gateway.
func (s *gatewayVariableStore) GetGatewayVariableValues(ctx context.Context,
	envID string) (map[string]string, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetGatewayVariableValues,
		envID, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	values := make(map[string]string, len(results))
	for _, row := range results {
		values[parseString(row["key"])] = parseString(row["value"])
	}
	return values, nil
}

// parseGatewayVariable maps a database row to an GatewayVariable.
func parseGatewayVariable(row map[string]interface{}) GatewayVariable {
	return GatewayVariable{
		ID:          parseString(row["id"]),
		Key:         parseString(row["key"]),
		Value:       parseString(row["value"]),
		Description: parseString(row["description"]),
		CreatedAt:   parseTimeString(row["created_at"]),
		UpdatedAt:   parseTimeString(row["updated_at"]),
	}
}

// parseString coerces a nullable text column value to a string.
func parseString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// parseTimeString coerces a timestamp column value (string on SQLite, time.Time on PostgreSQL) to a
// string.
func parseTimeString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

// scope is the deployment this store acts for: the one the request names, or the one this
// store was built for when a caller carries none.
func (s *gatewayVariableStore) scope(ctx context.Context) string {
	return deployment.ResolveOr(ctx, s.deploymentID)
}
