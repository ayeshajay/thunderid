// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/resourcevalidation"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/internal/system/varname"
)

// ServiceInterface is the authoring surface of a control plane.
type ServiceInterface interface {
	// Author records a document as it was written, replacing any document of the same type and name.
	Author(ctx context.Context, resourceType, name, payload string) (*Resource, error)
	// Get returns one authored document, payload included.
	Get(ctx context.Context, resourceType, name string) (*Resource, error)
	// List reports what has been authored, without the payloads.
	List(ctx context.Context) ([]Summary, error)
	// Delete removes an authored document.
	Delete(ctx context.Context, resourceType, name string) error
	// References reports the values a document refers to, so an operator knows what a gateway must
	// hold before it is applied there.
	References(ctx context.Context, resourceType, name string) ([]Reference, error)
}

// Reference is one value a document expects a gateway to supply.
type Reference struct {
	Name string `json:"name"`
	// Secret says which of the gateway's two stores the value comes from. It is read from the
	// reference itself rather than from a schema of which fields are secret.
	Secret bool `json:"secret"`
}

type service struct {
	store storeInterface
}

func newService(store storeInterface) ServiceInterface { return &service{store: store} }

// Author records a document as it was written.
//
// Two kinds of check run, and they are deliberately different in where they come from. The rules
// that hold on any plane come from the resource type itself, through the shared registry, so this
// plane applies exactly what the gateway will rather than a second opinion that could drift from it.
// What this plane adds is only what it alone can say: that the document is a document at all.
//
// What is not checked here is anything that needs a lookup. Whether an organization unit exists is
// a question about a world this plane is not designing for, and the gateway answers it at import,
// against real values, after the references have been resolved.
func (s *service) Author(ctx context.Context, resourceType, name, payload string) (*Resource, error) {
	resourceType = strings.TrimSpace(resourceType)
	name = strings.TrimSpace(name)

	if resourceType == "" {
		return nil, fmt.Errorf("a resource type is required")
	}
	if name == "" {
		return nil, fmt.Errorf("a name is required")
	}
	document, err := decode(payload)
	if err != nil {
		return nil, err
	}

	// The fields that belong to the deployment this document is applied to are not accepted here.
	// The resource type says which they are, and a reference to what each gateway supplies is put in
	// their place, so the caller neither has to know the naming nor can get it wrong.
	if err := resourcevalidation.FillDeploymentFields(ctx, resourceType, name, document); err != nil {
		return nil, err
	}

	filled, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("failed to store the document: %w", err)
	}
	payload = string(filled)

	// The rules the resource type owns, which both planes apply.
	if resourcevalidation.Supports(resourceType) {
		if err := resourcevalidation.Validate(ctx, resourceType, filled); err != nil {
			return nil, err
		}
	}

	id, err := sysutils.GenerateUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate an id: %w", err)
	}

	resource := Resource{ID: id, ResourceType: resourceType, Name: name, Payload: payload}
	if err := s.store.Upsert(ctx, resource); err != nil {
		return nil, err
	}
	return s.Get(ctx, resourceType, name)
}

// decode reads the payload as a document, which is the one thing this plane can check on its own.
//
// A payload that is not JSON is a mistake, and catching it here names the document rather than
// surfacing as a parse error on a gateway much later.
func decode(payload string) (map[string]interface{}, error) {
	if strings.TrimSpace(payload) == "" {
		return nil, fmt.Errorf("a payload is required")
	}
	var document map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &document); err != nil {
		return nil, fmt.Errorf("the payload is not a JSON document: %w", err)
	}
	if len(document) == 0 {
		return nil, fmt.Errorf("the payload describes nothing")
	}
	return document, nil
}

// Get returns one authored document.
func (s *service) Get(ctx context.Context, resourceType, name string) (*Resource, error) {
	resource, found, err := s.store.Get(ctx, strings.TrimSpace(resourceType), strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("nothing authored of type %q named %q", resourceType, name)
	}
	return &resource, nil
}

// List reports what has been authored.
func (s *service) List(ctx context.Context) ([]Summary, error) {
	stored, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]Summary, 0, len(stored))
	for _, r := range stored {
		summaries = append(summaries, summarize(r))
	}
	return summaries, nil
}

// Delete removes an authored document.
func (s *service) Delete(ctx context.Context, resourceType, name string) error {
	return s.store.Delete(ctx, strings.TrimSpace(resourceType), strings.TrimSpace(name))
}

// References reports the values a document expects a gateway to supply.
func (s *service) References(ctx context.Context, resourceType, name string) ([]Reference, error) {
	resource, err := s.Get(ctx, resourceType, name)
	if err != nil {
		return nil, err
	}
	return referencesIn(resource.Payload), nil
}

// referencesIn walks a document for the values it refers to, in a stable order and without repeats.
func referencesIn(payload string) []Reference {
	var document interface{}
	if err := json.Unmarshal([]byte(payload), &document); err != nil {
		return nil
	}

	seen := map[string]bool{}
	found := make([]Reference, 0, 4)
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case string:
			if name, secret, ok := varname.ParseReference(typed); ok && !seen[name] {
				seen[name] = true
				found = append(found, Reference{Name: name, Secret: secret})
			}
		case []interface{}:
			for _, item := range typed {
				walk(item)
			}
		case map[string]interface{}:
			for _, key := range sortedKeys(typed) {
				walk(typed[key])
			}
		}
	}
	walk(document)
	return found
}

// sortedKeys returns a map's keys in a stable order, so the references a document names are reported
// the same way every time rather than in Go's map order.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
