// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// basePath is where a control plane serves its authoring API.
//
// It is a separate path from the resource APIs, not a mode of them. What is authored here is a
// document with references in it, for a gateway to run; what /applications holds is an application
// this server runs. One path serving both would mean a caller could not tell, from the path it
// called, which of the two it got.
const basePath = "/authored"

// Initialize builds the authoring service and registers its routes.
//
// A control plane plugs in this and not the resource services: it designs configuration rather than
// running it, so it needs somewhere to keep a document and nothing that would execute one.
func Initialize(mux *http.ServeMux) ServiceInterface {
	svc := newService(newStore())
	registerRoutes(mux, &handler{service: svc})
	return svc
}

// registerRoutes mounts the authoring routes.
func registerRoutes(mux *http.ServeMux, h *handler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	collection := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST "+basePath, h.HandleAuthor, collection))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath, h.HandleList, collection))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath, noContent, collection))

	item := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET "+basePath+"/{type}/{name}", h.HandleGet, item))
	mux.HandleFunc(middleware.WithCORS("DELETE "+basePath+"/{type}/{name}", h.HandleDelete, item))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath+"/{type}/{name}", noContent, item))

	// What a document expects a gateway to supply, which is what an operator has to set there before
	// it can be applied.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+basePath+"/{type}/{name}/references", h.HandleReferences, item))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+basePath+"/{type}/{name}/references", noContent, item))
}
