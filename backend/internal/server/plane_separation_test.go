// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"os/exec"
	"strings"
	"testing"
)

// runtimePackages are the surfaces only a data plane runs. A control plane does not guard these
// paths, it does not have them, and the way to be sure of that is that the code behind them is not
// in the binary at all.
var runtimePackages = []string{
	"github.com/thunder-id/thunderid/internal/oauth",
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dcr",
	"github.com/thunder-id/thunderid/internal/openid4vci",
	"github.com/thunder-id/thunderid/internal/flow/flowexec",
	"github.com/thunder-id/thunderid/internal/authn",
	"github.com/thunder-id/thunderid/internal/authzen",
	"github.com/thunder-id/thunderid/internal/dataplane",
}

// authoringPackages are the surfaces only a control plane runs.
var authoringPackages = []string{
	"github.com/thunder-id/thunderid/internal/authored",
	"github.com/thunder-id/thunderid/internal/controlplane",
}

// dependenciesOf returns the full transitive package list linked into a binary.
func dependenciesOf(t *testing.T, mainPackage string) map[string]bool {
	t.Helper()

	out, err := exec.Command("go", "list", "-deps", mainPackage).Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", mainPackage, err)
	}

	deps := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		deps[line] = true
	}
	return deps
}

// TestControlPlaneLinksNoRuntime is the separation itself, asserted where it is decided.
//
// The planes are not told apart by configuration, so there is no setting to get wrong and no guard
// to leave off: a path a plane does not serve is missing from its binary. This test is what makes
// that claim checkable, and it fails the build the day someone wires a runtime service into the
// shared registration instead of into the data plane.
func TestControlPlaneLinksNoRuntime(t *testing.T) {
	deps := dependenciesOf(t, "github.com/thunder-id/thunderid/cmd/cpserver")

	for _, pkg := range runtimePackages {
		if deps[pkg] {
			t.Errorf("the control plane binary links %s, so it carries a runtime it must not serve", pkg)
		}
	}

	// It must still link what it is for.
	for _, pkg := range authoringPackages {
		if !deps[pkg] {
			t.Errorf("the control plane binary does not link %s", pkg)
		}
	}
}

// TestDataPlaneLinksNoAuthoring is the other half: a gateway runs configuration, it does not design
// it, so the authoring API is absent there for the same reason.
func TestDataPlaneLinksNoAuthoring(t *testing.T) {
	deps := dependenciesOf(t, "github.com/thunder-id/thunderid/cmd/server")

	for _, pkg := range authoringPackages {
		if deps[pkg] {
			t.Errorf("the data plane binary links %s, so it carries an authoring surface it must not serve", pkg)
		}
	}

	for _, pkg := range runtimePackages {
		if !deps[pkg] {
			t.Errorf("the data plane binary does not link %s", pkg)
		}
	}
}
