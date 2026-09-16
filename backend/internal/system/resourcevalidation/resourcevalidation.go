// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package resourcevalidation holds the checks on a resource that hold on any plane.
//
// A rule is either about the document or about the world. "An application with the authorization
// code grant needs a redirect URI" is about the document: it is true wherever the document is, so
// both planes apply it and they apply the same one. "This organization unit exists" is about the
// world, and each plane has its own: a control plane designing configuration for elsewhere cannot
// answer it, and should not try.
//
// So the shared rules live here, registered by the resource type that owns them, and each plane adds
// what only it can check. A control plane refuses the fields that belong to the deployment a
// document is applied to; a gateway resolves the references and checks the result against its own
// stores. Neither re-implements the rules in this package, which is the point: two copies of one
// rule drift, and the drift shows up as a document that one plane accepts and the other rejects.
package resourcevalidation

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Validator applies the plane-independent rules for one resource type.
type Validator interface {
	// ResourceType is the type these rules are for, as the declarative resources name it.
	ResourceType() string
	// Validate checks a payload against the rules that hold on any plane.
	//
	// It performs no lookups, so it can run on a plane that holds none of the things a lookup would
	// consult. Where a value may be a reference to something a gateway supplies, the rule tolerates
	// it: an authoring plane cannot check a value it was deliberately not given.
	Validate(ctx context.Context, payload []byte) error
}

var registry = struct {
	sync.RWMutex
	byType map[string]Validator
}{byType: map[string]Validator{}}

// Register records the shared rules for a resource type.
//
// Registering a type again replaces what was there, because registration happens while a resource
// type initializes and a process may do that more than once.
func Register(v Validator) error {
	if v == nil {
		return fmt.Errorf("resourcevalidation: a validator is required")
	}
	resourceType := v.ResourceType()
	if resourceType == "" {
		return fmt.Errorf("resourcevalidation: a validator must name its resource type")
	}

	registry.Lock()
	defer registry.Unlock()
	registry.byType[resourceType] = v
	return nil
}

// Validate applies the shared rules for a resource type.
//
// A type nothing registered rules for is an error naming it. Accepting it unchecked would let a
// control plane store a document that no plane has ever validated, and the first thing to notice
// would be the gateway it was applied to.
func Validate(ctx context.Context, resourceType string, payload []byte) error {
	registry.RLock()
	v, known := registry.byType[resourceType]
	registry.RUnlock()

	if !known {
		return fmt.Errorf("resourcevalidation: no rules registered for %q, known types are %v",
			resourceType, RegisteredTypes())
	}
	return v.Validate(ctx, payload)
}

// Supports reports whether shared rules exist for a resource type.
func Supports(resourceType string) bool {
	registry.RLock()
	defer registry.RUnlock()
	_, known := registry.byType[resourceType]
	return known
}

// RegisteredTypes lists the types that have shared rules, in a stable order.
func RegisteredTypes() []string {
	registry.RLock()
	defer registry.RUnlock()

	types := make([]string, 0, len(registry.byType))
	for resourceType := range registry.byType {
		types = append(types, resourceType)
	}
	sort.Strings(types)
	return types
}

// reset clears the registry, for tests that register their own rules.
func reset() {
	registry.Lock()
	defer registry.Unlock()
	registry.byType = map[string]Validator{}
}
