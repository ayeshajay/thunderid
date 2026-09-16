// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package secretgen makes the credential a resource needs, by asking the resource type that needs
// it.
//
// A credential is not one thing. An application's client secret, an outbound connection's key and a
// user's password differ in how they are produced and in what would make one acceptable, and the
// only code that knows those rules is the code that owns the resource. So this holds no rules of its
// own: each resource type registers how its credential is made, and a request names the type.
//
// The alternative, one generator that every caller shares, works only while every credential happens
// to be the same shape. The first resource type that needs a different one has to either bend to the
// shared rule or grow a special case in a package that should not know it exists.
package secretgen

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Generator makes the credential for one resource type.
type Generator interface {
	// ResourceType is the type this generator makes credentials for, as the declarative resources
	// name it: "application", "connection", and so on.
	ResourceType() string
	// Generate returns a new credential. It is returned rather than stored: where a credential is
	// kept is the caller's decision, and a generator that also stored one could not be used by a
	// caller that keeps it somewhere else.
	Generate(ctx context.Context) (string, error)
}

// registry holds the generators, keyed by resource type.
var registry = struct {
	sync.RWMutex
	byType map[string]Generator
}{byType: map[string]Generator{}}

// Register records how a resource type's credential is made.
//
// Registering a type again replaces what was there. Registration happens while a resource type
// initializes, and a process may do that more than once; the rules for a type come from that type's
// own package, so the second registration is the same rule again. Failing instead would turn a
// second initialization into a startup error for no gain.
func Register(g Generator) error {
	if g == nil {
		return fmt.Errorf("secretgen: a generator is required")
	}
	resourceType := g.ResourceType()
	if resourceType == "" {
		return fmt.Errorf("secretgen: a generator must name its resource type")
	}

	registry.Lock()
	defer registry.Unlock()
	registry.byType[resourceType] = g
	return nil
}

// Generate makes a credential for the named resource type.
//
// A type nothing registered for is an error naming it, not a fallback to some default. Falling back
// would hand out a credential built to the wrong rules, and it would work well enough to hide that
// until something rejected it.
func Generate(ctx context.Context, resourceType string) (string, error) {
	registry.RLock()
	g, known := registry.byType[resourceType]
	registry.RUnlock()

	if !known {
		return "", fmt.Errorf("secretgen: nothing generates a credential for %q, known types are %v",
			resourceType, RegisteredTypes())
	}
	return g.Generate(ctx)
}

// Supports reports whether a credential can be made for the named resource type.
func Supports(resourceType string) bool {
	registry.RLock()
	defer registry.RUnlock()
	_, known := registry.byType[resourceType]
	return known
}

// RegisteredTypes lists the resource types that can have a credential made, in a stable order so
// that an error message naming them reads the same every time.
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

// reset clears the registry. It exists for tests, which need to register a generator without the
// registration leaking into the next test.
func reset() {
	registry.Lock()
	defer registry.Unlock()
	registry.byType = map[string]Generator{}
}
