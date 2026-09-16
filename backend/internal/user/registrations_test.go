// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A user is mostly its attributes, and which it may carry is its user type's business, held per
// deployment. What is shared is the shape a user document has whatever it is for.
func TestSharedRules_CheckTheShapeAndNotTheWorld(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		wants   string
	}{
		{"no type", `{"attributes":{"username":"alice"}}`, "user type is required"},
		{"no attributes", `{"type":"Person"}`, "needs attributes"},
		{"not a user", `[]`, "not a user"},
		{
			"an email that is not one",
			`{"type":"Person","attributes":{"username":"alice","email":"alice-at-example"}}`,
			"not a valid address",
		},
	} {
		err := (sharedRules{}).Validate(context.Background(), []byte(tc.payload))
		if err == nil {
			t.Errorf("%s should be refused", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wants) {
			t.Errorf("%s: error = %q, want it to mention %q", tc.name, err, tc.wants)
		}
	}

	for _, tc := range []struct{ name, payload string }{
		{"a real address", `{"type":"Person","attributes":{"username":"alice","email":"alice@example.com"}}`},
		{"a referenced address", `{"type":"Person","attributes":{"email":"var:USER_ALICE_EMAIL"}}`},
		{"no email at all", `{"type":"Person","attributes":{"username":"alice"}}`},
	} {
		if err := (sharedRules{}).Validate(context.Background(), []byte(tc.payload)); err != nil {
			t.Errorf("%s should be accepted: %v", tc.name, err)
		}
	}
}

// A password is never carried between deployments even in principle: it is kept as a one way hash,
// so there is nothing to copy, and each deployment's copy of a user has its own.
func TestDeploymentFields_ReferenceThePassword(t *testing.T) {
	// Every one of these says the same thing: this user has a password, and the gateway chooses it.
	// An author writing the document by hand reaches for whichever of them reads best.
	for _, test := range []struct {
		name        string
		credentials map[string]interface{}
	}{
		{"a credentials block with nothing in it", map[string]interface{}{}},
		{"the field named and left empty", map[string]interface{}{"password": ""}},
		{"the field named and left null", map[string]interface{}{"password": nil}},
	} {
		t.Run(test.name, func(t *testing.T) {
			document := map[string]interface{}{
				"type":        "Person",
				"attributes":  map[string]interface{}{"username": "alice"},
				"credentials": test.credentials,
			}

			if err := (deploymentFields{}).Fill(context.Background(), "alice", document); err != nil {
				t.Fatal(err)
			}

			credentials := document["credentials"].(map[string]interface{})
			if credentials["password"] != "sec:USER_ALICE_PASSWORD" {
				t.Errorf("password = %v, want the derived reference", credentials["password"])
			}
		})
	}
}

func TestDeploymentFields_RefuseASuppliedPassword(t *testing.T) {
	document := map[string]interface{}{
		"credentials": map[string]interface{}{"password": "hunter2"},
	}

	err := (deploymentFields{}).Fill(context.Background(), "alice", document)
	if err == nil {
		t.Fatal("a supplied password must be refused")
	}
	if !strings.Contains(err.Error(), "not accepted here") {
		t.Errorf("error = %q, want it to say the password is not accepted", err)
	}
}

// A document read back and written again carries the reference it was given, and a user with no
// credentials is signing in some other way.
func TestDeploymentFields_LeaveWhatItShould(t *testing.T) {
	round := map[string]interface{}{
		"credentials": map[string]interface{}{"password": "sec:USER_ALICE_PASSWORD"},
	}
	if err := (deploymentFields{}).Fill(context.Background(), "alice", round); err != nil {
		t.Errorf("a re-submitted document should be accepted: %v", err)
	}

	none := map[string]interface{}{"type": "Person"}
	if err := (deploymentFields{}).Fill(context.Background(), "alice", none); err != nil {
		t.Errorf("a user with no credentials should be left alone: %v", err)
	}
	if _, added := none["credentials"]; added {
		t.Error("credentials must not be invented for a user that carries none")
	}
}

// A password is held by a person, so it is generated to the usual four classes rather than as raw
// entropy, which is where it differs from a client secret.
func TestPasswordGenerator(t *testing.T) {
	g := passwordGenerator{}

	if g.ResourceType() != "user" {
		t.Errorf("ResourceType = %q, want user", g.ResourceType())
	}

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		password, err := g.Generate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(password) != passwordLength {
			t.Fatalf("length = %d, want %d", len(password), passwordLength)
		}
		if seen[password] {
			t.Fatal("two generated passwords were the same")
		}
		seen[password] = true

		for name, class := range map[string]string{
			"upper": passwordUpper, "lower": passwordLower,
			"digit": passwordDigits, "symbol": passwordSymbol,
		} {
			if !strings.ContainsAny(password, class) {
				t.Errorf("password %q carries no %s", password, name)
			}
		}
		// The characters people confuse are left out, so a password can be read aloud.
		if strings.ContainsAny(password, "O0lI1") {
			t.Errorf("password %q uses a character that is easily misread", password)
		}
	}
}

// The three registrations are what a resource type offers, so they are reachable together.
func TestRegistrations_AreAcceptedTogether(t *testing.T) {
	for name, register := range map[string]func() error{
		"shared rules":      registerSharedRules,
		"deployment fields": registerDeploymentFields,
		"secret generator":  registerSecretGenerator,
	} {
		if err := register(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// And the shared rules reach a user document through the registry the same way both planes do.
	payload, err := json.Marshal(map[string]interface{}{
		"type": "Person", "attributes": map[string]interface{}{"username": "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (sharedRules{}).Validate(context.Background(), payload); err != nil {
		t.Errorf("a valid user should be accepted: %v", err)
	}
}
