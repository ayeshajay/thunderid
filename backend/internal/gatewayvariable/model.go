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

// Package gatewayvariable manages deployment-scoped, non-secret gateway variables on the
// Control Plane (for example redirect URLs). Each variable is keyed by the declarative placeholder
// name it resolves and its value is stored in plaintext. Unlike secrets, values are readable through
// the management API because they carry no confidential material.
package gatewayvariable

// GatewayVariable is the API representation of a stored gateway variable, including its
// value.
type GatewayVariable struct {
	ID          string `json:"id,omitempty"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// GatewayVariableResolveResponse is the response for the resolve path: every key mapped to its
// value, used by the config export/apply tooling to substitute declarative placeholders.
type GatewayVariableResolveResponse struct {
	Variables map[string]string `json:"variables"`
}

// GatewayVariableListResponse is the paginated response for listing gateway variables.
type GatewayVariableListResponse struct {
	TotalResults     int               `json:"totalResults"`
	Count            int               `json:"count"`
	GatewayVariables []GatewayVariable `json:"gatewayVariables"`
}

// CreateGatewayVariableRequest is the request body for creating an gateway variable.
type CreateGatewayVariableRequest struct {
	Key         string `json:"key" native:"required,min=1,max=255"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty" native:"max=255"`
}

// UpdateGatewayVariableRequest is the request body for updating an gateway variable's value
// and/or description.
type UpdateGatewayVariableRequest struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty" native:"max=255"`
}
