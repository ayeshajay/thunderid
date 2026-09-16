// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dataplane

import (
	"testing"

	"github.com/thunder-id/thunderid/internal/secretstore"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// A data plane holds the credentials the configuration applied to it refers to, so it keeps a store
// whether or not one was configured. Defaulting is done here rather than in the store package
// because it is a per-plane decision: a control plane keeps none at all, which is why this file is
// in the data plane's package and not in the shared build.
func TestAnUnsetModeDefaultsToTheDatabase(t *testing.T) {
	withRuntime(t)
	got := secretStoreConfig(engineconfig.SecretProviderConfig{}, nil, "acme:dev")

	if got.Mode != secretstore.ModeDB {
		t.Fatalf("expected an unset mode to default to the database, got %q", got.Mode)
	}
}

// A configured mode is never overridden, so a deployment that asked for a key vault or a file gets it.
func TestAConfiguredModeIsKept(t *testing.T) {
	withRuntime(t)
	for _, mode := range []string{"file", "kv", "service"} {
		got := secretStoreConfig(engineconfig.SecretProviderConfig{Mode: mode}, nil, "acme:dev")
		if string(got.Mode) != mode {
			t.Fatalf("expected %q to be kept, got %q", mode, got.Mode)
		}
	}
}

// withRuntime loads a minimal server runtime, which building the store configuration reads to reach
// the database provider.
func withRuntime(t *testing.T) {
	t.Helper()
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{}); err != nil {
		t.Fatalf("initialize runtime: %v", err)
	}
	t.Cleanup(config.ResetServerRuntime)
}
