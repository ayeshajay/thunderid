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

// Package tenant provides the Control Plane APIs a tenant provisions and deprovisions itself with.
//
// Every operation here acts on the caller's own organization, named by the deployment claim in its
// token. There is no privileged tenant that manages the others: a token reaches its own workspace and
// nothing else, which is the same guarantee every other resource on this plane gets.
package tenant

// Tenant is the API representation of a tenant recorded in the registry.
type Tenant struct {
	ID           string `json:"id,omitempty"`
	DeploymentID string `json:"deploymentId"`
	Name         string `json:"name,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// CreateTenantRequest is the request body for provisioning the caller's own workspace.
//
// It names no organization: the organization is the one the caller's token is for. Accepting one
// would let a token ask for a workspace it has no claim to.
type CreateTenantRequest struct {
	Name string `json:"name,omitempty" native:"max=255"`
}
