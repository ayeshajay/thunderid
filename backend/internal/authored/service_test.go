// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// fakeStore keeps authored documents in memory, keyed as the real store keys them.
type fakeStore struct {
	items map[string]Resource
}

func newFakeStore() *fakeStore { return &fakeStore{items: map[string]Resource{}} }

func key(resourceType, name string) string { return resourceType + "/" + name }

func (f *fakeStore) Upsert(_ context.Context, r Resource) error {
	f.items[key(r.ResourceType, r.Name)] = r
	return nil
}

func (f *fakeStore) Get(_ context.Context, resourceType, name string) (Resource, bool, error) {
	r, ok := f.items[key(resourceType, name)]
	return r, ok, nil
}

// List drops the payload, as the listing query does.
func (f *fakeStore) List(context.Context) ([]Resource, error) {
	listed := make([]Resource, 0, len(f.items))
	for _, r := range f.items {
		r.Payload = ""
		listed = append(listed, r)
	}
	return listed, nil
}

func (f *fakeStore) Delete(_ context.Context, resourceType, name string) error {
	delete(f.items, key(resourceType, name))
	return nil
}

const document = `{"name":"Storefront","url":"var:APPLICATION_STOREFRONT_URL",` +
	`"clientSecret":"sec:APPLICATION_STOREFRONT_CLIENT_SECRET"}`

// The document is kept with its references intact. A control plane is not running it, so nothing
// here is resolved, and resolving it would bind the document to this plane's values.
//
// What is kept is the content, not the bytes: the document is decoded so the deployment's fields can
// be filled in, and re-encoding it settles the key order.
func TestAuthor_KeepsTheReferencesInTheDocument(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()

	stored, err := svc.Author(ctx, "application", "Storefront", document)
	if err != nil {
		t.Fatal(err)
	}

	var got, want map[string]interface{}
	if err := json.Unmarshal([]byte(stored.Payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(document), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the document should be stored as written,\n got %v\nwant %v", got, want)
	}
	if stored.ResourceType != "application" || stored.Name != "Storefront" {
		t.Errorf("the document should be identified by its type and name, got %+v", stored)
	}
}

// Authoring the same resource twice is an edit, not a second document.
func TestAuthor_ReplacesADocumentOfTheSameTypeAndName(t *testing.T) {
	store := newFakeStore()
	svc := newService(store)
	ctx := context.Background()

	if _, err := svc.Author(ctx, "application", "Storefront", document); err != nil {
		t.Fatal(err)
	}
	edited := `{"name":"Storefront","url":"var:APPLICATION_STOREFRONT_NEW_URL"}`
	if _, err := svc.Author(ctx, "application", "Storefront", edited); err != nil {
		t.Fatal(err)
	}

	if len(store.items) != 1 {
		t.Fatalf("expected one document, got %d", len(store.items))
	}
	stored, err := svc.Get(ctx, "application", "Storefront")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored.Payload, "APPLICATION_STOREFRONT_NEW_URL") {
		t.Errorf("the later document should stand, got %s", stored.Payload)
	}
}

// A payload that is not a document is a mistake this plane can see on its own, and catching it here
// names the document rather than surfacing as a parse error on a gateway much later.
func TestAuthor_RefusesWhatIsNotADocument(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()

	for _, tc := range []struct{ name, payload string }{
		{"empty", ""},
		{"not JSON", "this is not a document"},
		{"an empty object", "{}"},
	} {
		if _, err := svc.Author(ctx, "application", "X", tc.payload); err == nil {
			t.Errorf("%s should be refused", tc.name)
		}
	}

	if _, err := svc.Author(ctx, "", "X", document); err == nil {
		t.Error("a document with no resource type should be refused")
	}
	if _, err := svc.Author(ctx, "application", "", document); err == nil {
		t.Error("a document with no name should be refused")
	}
}

// A listing says what has been authored, not what is in it.
func TestList_DoesNotCarryThePayloads(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()

	if _, err := svc.Author(ctx, "application", "Storefront", document); err != nil {
		t.Fatal(err)
	}

	listed, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one entry, got %d", len(listed))
	}
	if listed[0].Name != "Storefront" || listed[0].ResourceType != "application" {
		t.Errorf("a listing should identify the document, got %+v", listed[0])
	}
}

// What a document expects a gateway to supply, and which of the gateway's two stores each comes
// from, read from the references themselves rather than from a schema of which fields are secret.
func TestReferences_ReportsWhatAGatewayMustSupply(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()

	if _, err := svc.Author(ctx, "application", "Storefront", document); err != nil {
		t.Fatal(err)
	}

	refs, err := svc.References(ctx, "application", "Storefront")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected two references, got %+v", refs)
	}

	kind := map[string]bool{}
	for _, r := range refs {
		kind[r.Name] = r.Secret
	}
	if secret, found := kind["APPLICATION_STOREFRONT_URL"]; !found || secret {
		t.Error("the URL should be reported as a plain value")
	}
	if secret, found := kind["APPLICATION_STOREFRONT_CLIENT_SECRET"]; !found || !secret {
		t.Error("the credential should be reported as a secret")
	}
}

// A document nothing authored is reported as such rather than as an empty one.
func TestGet_ReportsWhatWasNeverAuthored(t *testing.T) {
	svc := newService(newFakeStore())

	_, err := svc.Get(context.Background(), "application", "Missing")
	if err == nil {
		t.Fatal("a document that was never authored should be reported")
	}
	if !strings.Contains(err.Error(), "Missing") {
		t.Errorf("the error should name what was asked for, got %q", err)
	}
}
