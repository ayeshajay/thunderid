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

// Package gateway promotes configuration between deployments: it captures a control plane's
// configuration as a version, diffs versions, and applies one to a data plane. It is the standalone
// gateway manager service brought in-process; the standalone one still exists and is unchanged.
package gateway

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/gateway/auth"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize mounts the gateway manager on the given mux and returns its service.
func Initialize(mux *http.ServeMux) (*registry, error) {
	reg := newRegistry()
	registerRoutes(mux, reg)
	return reg, nil
}

// registerRoutes mounts each route on the server's own mux rather than delegating a subtree, so
// these paths sit alongside the rest of the management API instead of behind a prefix.
//
// Every route records the caller's bearer token: the capture and apply paths call a control plane
// and a data plane as the caller, and without the token those calls arrive unauthenticated.
func registerRoutes(mux *http.ServeMux, reg *registry) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	routes := map[string]func(*Server) http.HandlerFunc{
		"POST /gateways":            func(s *Server) http.HandlerFunc { return s.createGateway },
		"GET /gateways":             func(s *Server) http.HandlerFunc { return s.listGateways },
		"GET /gateways/{id}":        func(s *Server) http.HandlerFunc { return s.getGateway },
		"PATCH /gateways/{id}":      func(s *Server) http.HandlerFunc { return s.updateGateway },
		"GET /data-plane-jobs/{id}": func(s *Server) http.HandlerFunc { return s.getDataPlaneJob },
		"DELETE /gateways/{id}":     func(s *Server) http.HandlerFunc { return s.deleteGateway },
		// Versions belong to the organization, so they are not served under a gateway.
		"POST /versions":                     func(s *Server) http.HandlerFunc { return s.createVersion },
		"GET /versions":                      func(s *Server) http.HandlerFunc { return s.listVersions },
		"GET /versions/{seq}":                func(s *Server) http.HandlerFunc { return s.getVersion },
		"GET /gateways/{id}/history":         func(s *Server) http.HandlerFunc { return s.gatewayHistory },
		"GET /gateways/{id}/diff":            func(s *Server) http.HandlerFunc { return s.diff },
		"GET /gateways/{id}/variable-status": func(s *Server) http.HandlerFunc { return s.checkVariables },
		"POST /gateways/{id}/apply":          func(s *Server) http.HandlerFunc { return s.apply },
		"GET /gateways/{id}/secrets":         func(s *Server) http.HandlerFunc { return s.listSecrets },
		"POST /gateways/{id}/data-plane-token": func(s *Server) http.HandlerFunc {
			return s.regenerateDataPlaneToken
		},
		"PUT /gateways/{id}/secrets/{name}": func(s *Server) http.HandlerFunc { return s.setSecret },
		"POST /gateways/{id}/secrets/{name}/regenerate": func(s *Server) http.HandlerFunc {
			return s.regenerateSecret
		},
		"POST /gateways/{id}/revert": func(s *Server) http.HandlerFunc { return s.revert },
		"POST /gateways/{id}/managed": func(s *Server) http.HandlerFunc {
			return s.setManagedGateway
		},
		"POST /apply":                                func(s *Server) http.HandlerFunc { return s.applyAll },
		"GET /promotions/preview":                    func(s *Server) http.HandlerFunc { return s.promotePreview },
		"POST /promotions":                           func(s *Server) http.HandlerFunc { return s.promote },
		"PUT /tenants/{deploymentId}/secrets/{name}": func(s *Server) http.HandlerFunc { return s.captureSecret },
	}
	// Promotion is the one action gated on a scope; see requirePromotionScope.
	promotionRoutes := promotionGatedRoutes()
	for pattern, pick := range routes {
		handler := reg.handler(pick)
		if promotionRoutes[pattern] {
			handler = requirePromotionScope(handler)
		}
		mux.HandleFunc(middleware.WithCORS(pattern, auth.RecordCallerToken(handler), opts))
	}

	for _, pattern := range []string{"OPTIONS /gateways", "OPTIONS /gateways/", "OPTIONS /versions",
		"OPTIONS /promotions", "OPTIONS /apply"} {
		mux.HandleFunc(middleware.WithCORS(pattern, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	}
}

// promotionGatedRoutes are the routes only the gateway manager may call.
//
// It holds the organization's gateway hierarchy and is the only caller that promotes. The gateway
// details it records describe that hierarchy, so writing them is gated the same way promoting is.
// Applying and reverting are deliberately not here: those belong to any member of the organization.
func promotionGatedRoutes() map[string]bool {
	return map[string]bool{
		"GET /promotions/preview": true,
		"POST /promotions":        true,
		"PATCH /gateways/{id}":    true,
	}
}
