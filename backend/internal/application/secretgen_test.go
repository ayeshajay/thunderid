// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"
)

// The rule for what an application's credential looks like lives with the application, and the
// registry only routes to it.
func TestClientSecretGenerator(t *testing.T) {
	g := clientSecretGenerator{}

	if g.ResourceType() != "application" {
		t.Errorf("ResourceType = %q, want application", g.ResourceType())
	}

	first, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first == "" {
		t.Fatal("a generated credential must not be empty")
	}

	second, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("two generated credentials must differ")
	}
}
