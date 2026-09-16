// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resourcevalidation

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// DeploymentFields names the fields of a resource type that belong to the deployment a document is
// applied to, rather than to the document.
//
// A redirect URI is where one deployment sends its users and a client secret is one deployment's
// credential. Authoring either would produce a value that is wrong everywhere except where it came
// from, so an authoring plane does not accept them and puts a reference in their place.
//
// Which fields those are is the resource type's knowledge, like its rules and its credential, so the
// type says so here and the authoring plane asks.
type DeploymentFields interface {
	// ResourceType is the type these fields belong to.
	ResourceType() string
	// Fill removes the deployment's fields from a document and puts a reference to each in their
	// place, reporting an error if the document carried a value for one.
	//
	// It works on the decoded document so that a field is found wherever it sits, rather than by
	// matching text that happens to look like one.
	Fill(ctx context.Context, name string, document map[string]interface{}) error
}

var fieldRegistry = struct {
	sync.RWMutex
	byType map[string]DeploymentFields
}{byType: map[string]DeploymentFields{}}

// RegisterDeploymentFields records which fields of a resource type belong to the deployment.
func RegisterDeploymentFields(f DeploymentFields) error {
	if f == nil {
		return fmt.Errorf("resourcevalidation: deployment fields are required")
	}
	resourceType := f.ResourceType()
	if resourceType == "" {
		return fmt.Errorf("resourcevalidation: deployment fields must name their resource type")
	}

	fieldRegistry.Lock()
	defer fieldRegistry.Unlock()
	fieldRegistry.byType[resourceType] = f
	return nil
}

// FillDeploymentFields replaces a resource type's deployment-owned fields with references.
//
// A type that declared none is left alone: not every resource has a field that belongs elsewhere,
// and a type that has not said so yet should not have its document rewritten on a guess.
func FillDeploymentFields(
	ctx context.Context, resourceType, name string, document map[string]interface{},
) error {
	fieldRegistry.RLock()
	f, known := fieldRegistry.byType[resourceType]
	fieldRegistry.RUnlock()

	if !known {
		return nil
	}
	return f.Fill(ctx, name, document)
}

// DeclaredDeploymentFieldTypes lists the types that have declared deployment-owned fields.
func DeclaredDeploymentFieldTypes() []string {
	fieldRegistry.RLock()
	defer fieldRegistry.RUnlock()

	types := make([]string, 0, len(fieldRegistry.byType))
	for resourceType := range fieldRegistry.byType {
		types = append(types, resourceType)
	}
	sort.Strings(types)
	return types
}

// resetFields clears the registry, for tests.
func resetFields() {
	fieldRegistry.Lock()
	defer fieldRegistry.Unlock()
	fieldRegistry.byType = map[string]DeploymentFields{}
}
