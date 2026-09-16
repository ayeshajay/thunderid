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

package tenant

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Config holds the settings the tenant module needs to provision a workspace.
type Config struct {
	// DefaultsDir is the bootstrap bundle directory (e.g. <serverHome>/bootstrap).
	DefaultsDir string
	// PublicURL is the Control Plane's public base URL, used to template the provisioned baseline.
	PublicURL string
}

// Initialize creates the tenant service and registers the /tenant routes.
func Initialize(mux *http.ServeMux, importSvc importer.ImportServiceInterface,
	cfg Config) (TenantServiceInterface, error) {
	store := newTenantStore()
	service := newTenantService(store, importSvc, cfg.DefaultsDir, cfg.PublicURL)
	handler := newTenantHandler(service)
	registerTenantRoutes(mux, handler)
	return service, nil
}

// registerTenantRoutes registers the tenant self-management routes under /tenant.
//
// The path names no tenant because a caller can only ever act on its own: the organization comes from
// the token, so there is nothing to address.
func registerTenantRoutes(mux *http.ServeMux, h *tenantHandler) {
	const basePath = "/tenant"

	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST "+basePath, h.HandleTenantPostRequest, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath, h.HandleTenantGetRequest, opts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+basePath, h.HandleTenantDeleteRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
