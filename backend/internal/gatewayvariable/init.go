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
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize creates the gateway variable service and registers its management routes.
func Initialize(mux *http.ServeMux) (GatewayVariableServiceInterface, error) {
	store := newGatewayVariableStore()
	service := newGatewayVariableService(store)
	handler := newGatewayVariableHandler(service)
	registerGatewayVariableRoutes(mux, handler)
	return service, nil
}

// registerGatewayVariableRoutes registers the CRUD routes for the gateway variable handler
// under one gateway.
//
// A variable belongs to an gateway, not to the organization: its value is a property of the
// deployment it is applied to, so the gateway is named in the path rather than inferred.
func registerGatewayVariableRoutes(mux *http.ServeMux, h *gatewayVariableHandler) {
	const basePath = "/gateways/{gatewayId}/variables"

	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST "+basePath, h.HandleGatewayVariablePostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath, h.HandleGatewayVariableListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	// The literal "resolve" segment takes precedence over "{id}".
	mux.HandleFunc(middleware.WithCORS("GET "+basePath+"/resolve", h.HandleGatewayVariableResolveRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath+"/{id}", h.HandleGatewayVariableGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT "+basePath+"/{id}", h.HandleGatewayVariablePutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE "+basePath+"/{id}", h.HandleGatewayVariableDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath+"/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))
}
