package importer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
)

// The whole chain: a control plane hashes a credential into the provider, the data plane loads it,
// the import stores a reference, and nothing anywhere holds the secret.
// clientSecretDocument is one application whose client secret is a placeholder, which is what each of
// these imports is asked to resolve.
const clientSecretDocument = "resource_type: application\nid: a\nclientSecret: {{.MY_APP_CLIENT_SECRET}}\n"

func TestEndToEnd_AReadableSecretIsFilledInAsItself(t *testing.T) {
	const value = "the-client-secret"
	// The provider serves the whole set at /secrets and one entry at /secrets/{name}. Both are needed:
	// the resolver loads the set at startup and asks for a single name it has not seen.
	entry := map[string]interface{}{"kind": "value", "value": value}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secrets" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"secrets": map[string]interface{}{"MY_APP_CLIENT_SECRET": entry},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(entry)
	}))
	defer srv.Close()

	prev := secretresolver.Default()
	r := secretresolver.New(secretresolver.Config{BaseURL: srv.URL})
	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	secretresolver.SetDefault(r)
	defer secretresolver.SetDefault(prev)

	content := clientSecretDocument
	filled := fillSecretPlaceholders(context.Background(), content, nil)

	if filled["MY_APP_CLIENT_SECRET"] != value {
		t.Fatalf("expected the credential itself, got %#v", filled["MY_APP_CLIENT_SECRET"])
	}
}

// A value the caller sent is never replaced by one held here, so an explicit request always wins.
func TestEndToEnd_AnExplicitValueIsNotOverridden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"secrets": map[string]interface{}{
			"MY_APP_CLIENT_SECRET": map[string]interface{}{"kind": "value", "value": "from-the-store"},
		}})
	}))
	defer srv.Close()

	prev := secretresolver.Default()
	r := secretresolver.New(secretresolver.Config{BaseURL: srv.URL})
	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	secretresolver.SetDefault(r)
	defer secretresolver.SetDefault(prev)

	content := clientSecretDocument
	filled := fillSecretPlaceholders(context.Background(), content,
		map[string]interface{}{"MY_APP_CLIENT_SECRET": "from-the-caller"})

	if filled["MY_APP_CLIENT_SECRET"] != "from-the-caller" {
		t.Fatalf("the caller's value must win, got %#v", filled["MY_APP_CLIENT_SECRET"])
	}
}

// asDataPlane makes this server a Data Plane for the duration of a test, which is what decides
// whether a credential held as a hash keeps its reference or is refused.
func asDataPlane(t *testing.T) {
	t.Helper()
	RunsAsDataPlane()
	t.Cleanup(func() { dataPlane.Store(false) })
}

// hashOnlyProvider serves one credential held as a hash, the way a provider does.
func hashOnlyProvider(t *testing.T) {
	t.Helper()
	entry := map[string]interface{}{
		"kind": "hash", "value": "the-hash", "algorithm": "PBKDF2",
		"parameters": map[string]interface{}{"salt": "s", "iterations": 1000, "keySize": 32},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secrets" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"secrets": map[string]interface{}{"MY_APP_CLIENT_SECRET": entry},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(entry)
	}))
	t.Cleanup(srv.Close)

	prev := secretresolver.Default()
	r := secretresolver.New(secretresolver.Config{BaseURL: srv.URL})
	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	secretresolver.SetDefault(r)
	t.Cleanup(func() { secretresolver.SetDefault(prev) })
}

// On a hybrid server a credential held as a hash keeps its reference, which is resolved at
// authentication. A hybrid server is not applied to by a Control Plane, so rewriting its resources
// with values read from a local store would change what a plain import means.
func TestHybridKeepsTheReferenceForAHashedCredential(t *testing.T) {
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	hashOnlyProvider(t)

	filled := fillSecretPlaceholders(context.Background(), clientSecretDocument, nil)

	if filled["MY_APP_CLIENT_SECRET"] != "kv:MY_APP_CLIENT_SECRET" {
		t.Fatalf("a hybrid import should keep the reference, got %#v", filled["MY_APP_CLIENT_SECRET"])
	}
}

// A Data Plane never keeps a reference: the Control Plane pushed the credential into its store as a
// value, so a hash means the credential this import needs is simply not there, and leaving a
// reference would write one nothing can resolve.
func TestTheDataPlaneDoesNotFillFromAHash(t *testing.T) {
	asDataPlane(t)
	hashOnlyProvider(t)

	filled := fillSecretPlaceholders(context.Background(), clientSecretDocument, nil)

	if got, ok := filled["MY_APP_CLIENT_SECRET"]; ok {
		t.Fatalf("expected the placeholder left unfilled, got %#v", got)
	}
}
