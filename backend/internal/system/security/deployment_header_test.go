// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// withHeaderConfig runs a test against a server that trusts the header and knows the given key.
func withHeaderConfig(t *testing.T, key string) {
	t.Helper()
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Server: engineconfig.ServerConfig{
			DeploymentIDSource: engineconfig.DeploymentIDSourceToken,
			DeploymentIDClaim:  "deploymentId",
			DeploymentIDHeader: engineconfig.DeploymentIDHeaderConfig{Enabled: true, Key: key},
		},
	}); err != nil {
		t.Fatalf("initialize runtime: %v", err)
	}
	TrustDeploymentHeaders()
	t.Cleanup(func() {
		config.ResetServerRuntime()
		trustsHeaders.Store(false)
	})
}

func request(key, id string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/applications", nil)
	if key != "" {
		r.Header.Set(engineconfig.DefaultDeploymentKeyHeader, key)
	}
	if id != "" {
		r.Header.Set(engineconfig.DefaultDeploymentIDHeader, id)
	}
	return r
}

// A caller that proves the key may name the deployment it acts for.
func TestAHeldKeyNamesTheDeployment(t *testing.T) {
	withHeaderConfig(t, "shared-key")
	got, keyed := headerDeploymentID(request("shared-key", "acme"))
	if !keyed || got != "acme" {
		t.Fatalf("expected the named deployment, got %q keyed=%v", got, keyed)
	}
}

// Naming nothing leaves the decision to the token, which is the ordinary case.
func TestNoKeyLeavesItToTheToken(t *testing.T) {
	withHeaderConfig(t, "shared-key")
	got, keyed := headerDeploymentID(request("", "acme"))
	if keyed || got != "" {
		t.Fatalf("an unkeyed request must not name a deployment, got %q keyed=%v", got, keyed)
	}
}

// A wrong key is answered rather than ignored: the caller asked to act for someone else.
func TestAWrongKeyIsRefusedRatherThanIgnored(t *testing.T) {
	withHeaderConfig(t, "shared-key")
	got, keyed := headerDeploymentID(request("not-the-key", "acme"))
	if !keyed || got != "" {
		t.Fatalf("a wrong key must be refused, got %q keyed=%v", got, keyed)
	}
}

// The key alone names nothing, so it is refused the same way rather than falling back.
func TestAKeyWithNoDeploymentIsRefused(t *testing.T) {
	withHeaderConfig(t, "shared-key")
	got, keyed := headerDeploymentID(request("shared-key", ""))
	if !keyed || got != "" {
		t.Fatalf("a key naming nothing must be refused, got %q keyed=%v", got, keyed)
	}
}

// A plane that does not trust the header ignores it entirely, whatever the caller sends.
func TestAPlaneThatDoesNotTrustHeadersIgnoresThem(t *testing.T) {
	withHeaderConfig(t, "shared-key")
	trustsHeaders.Store(false)
	got, keyed := headerDeploymentID(request("shared-key", "acme"))
	if keyed || got != "" {
		t.Fatalf("an untrusting plane must ignore the header, got %q keyed=%v", got, keyed)
	}
}

// Configuring no key turns the mechanism off, so the token decides.
func TestNoConfiguredKeyDisablesTheMechanism(t *testing.T) {
	withHeaderConfig(t, "")
	got, keyed := headerDeploymentID(request("anything", "acme"))
	if keyed || got != "" {
		t.Fatalf("an unconfigured key must disable the header, got %q keyed=%v", got, keyed)
	}
}

// Switching it off leaves the token resolution untouched: the header is simply not read, so a caller
// holding the key acts for its own deployment like anyone else.
func TestSwitchedOffIgnoresTheHeaderEvenWithTheKey(t *testing.T) {
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Server: engineconfig.ServerConfig{
			DeploymentIDSource: engineconfig.DeploymentIDSourceToken,
			DeploymentIDClaim:  "deploymentId",
			DeploymentIDHeader: engineconfig.DeploymentIDHeaderConfig{Enabled: false, Key: "shared-key"},
		},
	}); err != nil {
		t.Fatalf("initialize runtime: %v", err)
	}
	TrustDeploymentHeaders()
	t.Cleanup(func() {
		config.ResetServerRuntime()
		trustsHeaders.Store(false)
	})

	got, keyed := headerDeploymentID(request("shared-key", "acme"))
	if keyed || got != "" {
		t.Fatalf("a disabled header must be ignored, got %q keyed=%v", got, keyed)
	}
}
