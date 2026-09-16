// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package secretgen

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stub makes a fixed credential for one resource type.
type stub struct {
	resourceType string
	value        string
	err          error
}

func (s stub) ResourceType() string { return s.resourceType }

func (s stub) Generate(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.value, nil
}

// A credential is made by the resource type that needs it, so two types produce their own.
func TestGenerate_AsksTheResourceTypeThatNeedsIt(t *testing.T) {
	reset()
	defer reset()

	if err := Register(stub{resourceType: "application", value: "app-secret"}); err != nil {
		t.Fatal(err)
	}
	if err := Register(stub{resourceType: "connection", value: "connection-key"}); err != nil {
		t.Fatal(err)
	}

	for resourceType, want := range map[string]string{
		"application": "app-secret",
		"connection":  "connection-key",
	} {
		got, err := Generate(context.Background(), resourceType)
		if err != nil {
			t.Fatalf("%s: %v", resourceType, err)
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", resourceType, got, want)
		}
	}
}

// A type nothing registered for is an error naming it. Falling back to some default would hand out a
// credential built to the wrong rules, and it would work well enough to hide that.
func TestGenerate_RefusesATypeNothingMakes(t *testing.T) {
	reset()
	defer reset()

	if err := Register(stub{resourceType: "application", value: "app-secret"}); err != nil {
		t.Fatal(err)
	}

	_, err := Generate(context.Background(), "agent")
	if err == nil {
		t.Fatal("a type with no generator must be refused")
	}
	if !strings.Contains(err.Error(), "agent") || !strings.Contains(err.Error(), "application") {
		t.Errorf("the error should name the type asked for and what is known, got %q", err)
	}
	if Supports("agent") {
		t.Error("Supports must report an unregistered type as unsupported")
	}
}

// A resource type may initialize more than once in a process, so registering again is the same rule
// again rather than an error.
func TestRegister_IsIdempotentForAType(t *testing.T) {
	reset()
	defer reset()

	if err := Register(stub{resourceType: "application", value: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := Register(stub{resourceType: "application", value: "second"}); err != nil {
		t.Fatalf("registering a type again should be allowed: %v", err)
	}

	value, err := Generate(context.Background(), "application")
	if err != nil || value != "second" {
		t.Errorf("the latest registration should stand, got %q %v", value, err)
	}
	if len(RegisteredTypes()) != 1 {
		t.Errorf("a type registered twice is still one type, got %v", RegisteredTypes())
	}
}

// A generator that cannot make one says so, rather than returning an empty credential that would be
// stored and fail much later.
func TestGenerate_ReportsAGeneratorFailure(t *testing.T) {
	reset()
	defer reset()

	failure := errors.New("no entropy")
	if err := Register(stub{resourceType: "application", err: failure}); err != nil {
		t.Fatal(err)
	}

	if _, err := Generate(context.Background(), "application"); !errors.Is(err, failure) {
		t.Errorf("the generator's failure should reach the caller, got %v", err)
	}
}

func TestRegister_RequiresAGeneratorAndAType(t *testing.T) {
	reset()
	defer reset()

	if err := Register(nil); err == nil {
		t.Error("a nil generator must be refused")
	}
	if err := Register(stub{resourceType: "", value: "x"}); err == nil {
		t.Error("a generator with no resource type must be refused")
	}
}
