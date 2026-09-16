// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// An organization's history is bounded, and the bound is the deployment's to set. A deployment that
// has not said keeps the product default rather than everything.
func TestKeepVersionsFallsBackToTheDefault(t *testing.T) {
	if got := (config.VersionsConfig{}).KeepVersions(); got != config.DefaultKeepVersions {
		t.Fatalf("an unset keep should fall back to %d, got %d", config.DefaultKeepVersions, got)
	}
	if got := (config.VersionsConfig{Keep: 7}).KeepVersions(); got != 7 {
		t.Fatalf("a configured keep should be honored, got %d", got)
	}
	// A negative or zero count would otherwise prune everything, including the version a gateway is
	// running, so it is read as "not configured" rather than as "keep none".
	if got := (config.VersionsConfig{Keep: -1}).KeepVersions(); got != config.DefaultKeepVersions {
		t.Fatalf("a nonsense keep should fall back to %d, got %d", config.DefaultKeepVersions, got)
	}
}

// Reverting to an arbitrary version takes everything applied in between with it, so a deployment can
// limit a gateway to stepping back one version at a time.
func TestRevertToPreviousOnlyRefusesAnArbitraryVersion(t *testing.T) {
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Versions: config.VersionsConfig{RevertToPreviousOnly: true},
	}); err != nil {
		t.Fatalf("initialize runtime: %v", err)
	}
	t.Cleanup(config.ResetServerRuntime)

	svc := &Service{}
	if _, err := svc.Revert(t.Context(), RevertInput{GatewayID: "env-1", ToRef: "2"}); err == nil {
		t.Fatal("reverting to a named version must be refused when only the previous is allowed")
	} else if !errors.Is(err, ErrRevertNotPrevious) {
		t.Fatalf("expected ErrRevertNotPrevious, got %v", err)
	}
}
