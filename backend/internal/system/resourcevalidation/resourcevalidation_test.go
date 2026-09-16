// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resourcevalidation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stub struct {
	resourceType string
	err          error
}

func (s stub) ResourceType() string { return s.resourceType }

func (s stub) Validate(context.Context, []byte) error { return s.err }

// The rules for a type come from that type, and both planes reach them the same way.
func TestValidate_AsksTheResourceTypeThatOwnsTheRules(t *testing.T) {
	reset()
	defer reset()

	refused := errors.New("a redirecting client needs somewhere to redirect")
	if err := Register(stub{resourceType: "application", err: refused}); err != nil {
		t.Fatal(err)
	}
	if err := Register(stub{resourceType: "connection"}); err != nil {
		t.Fatal(err)
	}

	if err := Validate(context.Background(), "application", []byte(`{}`)); !errors.Is(err, refused) {
		t.Errorf("the type's own rule should reach the caller, got %v", err)
	}
	if err := Validate(context.Background(), "connection", []byte(`{}`)); err != nil {
		t.Errorf("a type whose rules pass should be accepted, got %v", err)
	}
}

// Accepting an unknown type unchecked would let a control plane store a document that no plane has
// ever validated, and the gateway would be the first to notice.
func TestValidate_RefusesATypeWithNoRules(t *testing.T) {
	reset()
	defer reset()

	if err := Register(stub{resourceType: "application"}); err != nil {
		t.Fatal(err)
	}

	err := Validate(context.Background(), "agent", []byte(`{}`))
	if err == nil {
		t.Fatal("a type with no registered rules must be refused")
	}
	if !strings.Contains(err.Error(), "agent") || !strings.Contains(err.Error(), "application") {
		t.Errorf("the error should name the type asked for and what is known, got %q", err)
	}
	if Supports("agent") {
		t.Error("Supports must report an unregistered type as unsupported")
	}
}

// A resource type may initialize more than once in a process.
func TestRegister_IsIdempotentForAType(t *testing.T) {
	reset()
	defer reset()

	if err := Register(stub{resourceType: "application"}); err != nil {
		t.Fatal(err)
	}
	if err := Register(stub{resourceType: "application"}); err != nil {
		t.Fatalf("registering a type again should be allowed: %v", err)
	}
	if len(RegisteredTypes()) != 1 {
		t.Errorf("a type registered twice is still one type, got %v", RegisteredTypes())
	}
}

func TestRegister_RequiresAValidatorAndAType(t *testing.T) {
	reset()
	defer reset()

	if err := Register(nil); err == nil {
		t.Error("a nil validator must be refused")
	}
	if err := Register(stub{resourceType: ""}); err == nil {
		t.Error("a validator with no resource type must be refused")
	}
}

// stubFields stands in for a resource type declaring which of its fields belong to the deployment.
type stubFields struct {
	resourceType string
	filled       bool
}

func (s *stubFields) ResourceType() string { return s.resourceType }

func (s *stubFields) Fill(context.Context, string, map[string]interface{}) error {
	s.filled = true
	return nil
}

// A type that declared no deployment-owned fields is left alone. Not every resource has a field that
// belongs elsewhere, and rewriting a document on a guess would be worse than leaving it.
func TestFillDeploymentFields_LeavesATypeThatDeclaredNone(t *testing.T) {
	resetFields()
	defer resetFields()

	application := &stubFields{resourceType: "application"}
	if err := RegisterDeploymentFields(application); err != nil {
		t.Fatal(err)
	}

	if err := FillDeploymentFields(context.Background(), "agent", "X", map[string]interface{}{}); err != nil {
		t.Errorf("a type with no declaration should be left alone, got %v", err)
	}
	if application.filled {
		t.Error("another type's declaration must not be applied")
	}

	if err := FillDeploymentFields(context.Background(), "application", "X", map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if !application.filled {
		t.Error("the type's own declaration should have been applied")
	}
	if len(DeclaredDeploymentFieldTypes()) != 1 {
		t.Errorf("expected one declared type, got %v", DeclaredDeploymentFieldTypes())
	}
}

func TestRegisterDeploymentFields_RequiresADeclarationAndAType(t *testing.T) {
	resetFields()
	defer resetFields()

	if err := RegisterDeploymentFields(nil); err == nil {
		t.Error("a nil declaration must be refused")
	}
	if err := RegisterDeploymentFields(&stubFields{resourceType: ""}); err == nil {
		t.Error("a declaration with no resource type must be refused")
	}
}
