// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// handler serves the authoring API.
type handler struct {
	service ServiceInterface
}

// authorRequest is a document being authored. The payload is carried as it was written, so it is
// taken raw rather than decoded into a shape this plane would have to model.
type authorRequest struct {
	ResourceType string          `json:"resourceType"`
	Name         string          `json:"name"`
	Payload      json.RawMessage `json:"payload"`
}

// HandleAuthor records a document as it was written.
func (h *handler) HandleAuthor(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read the request body")
		return
	}

	var req authorRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "the request body is not valid")
		return
	}

	resource, err := h.service.Author(r.Context(), req.ResourceType, req.Name, string(req.Payload))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resource)
}

// HandleList reports what has been authored, without the payloads.
func (h *handler) HandleList(w http.ResponseWriter, r *http.Request) {
	resources, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list authored resources")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]Summary{"authored": resources})
}

// HandleGet returns one authored document, payload included.
func (h *handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	resource, err := h.service.Get(r.Context(), r.PathValue("type"), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resource)
}

// HandleDelete removes an authored document.
func (h *handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), r.PathValue("type"), r.PathValue("name")); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete the authored resource")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleReferences reports the values a document expects a gateway to supply.
func (h *handler) HandleReferences(w http.ResponseWriter, r *http.Request) {
	references, err := h.service.References(r.Context(), r.PathValue("type"), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]Reference{"references": references})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"message": strings.TrimSpace(message)})
}
