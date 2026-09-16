// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package varname

import "testing"

// A gateway resolving a reference has to know which store to look in, and the reference itself is
// what tells it.
func TestParseReference_TellsAVariableFromASecret(t *testing.T) {
	for _, tc := range []struct {
		value      string
		wantName   string
		wantSecret bool
		wantOK     bool
	}{
		{"var:APPLICATION_MY_APP_REDIRECT_URIS", "APPLICATION_MY_APP_REDIRECT_URIS", false, true},
		{"sec:APPLICATION_MY_APP_CLIENT_SECRET", "APPLICATION_MY_APP_CLIENT_SECRET", true, true},
		{"  sec:PADDED  ", "PADDED", true, true},
		{"https://app.example/cb", "", false, false},
		{"", "", false, false},
		{"variable:NOT_THE_PREFIX", "", false, false},
	} {
		name, isSecret, ok := ParseReference(tc.value)
		if ok != tc.wantOK || name != tc.wantName || isSecret != tc.wantSecret {
			t.Errorf("ParseReference(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tc.value, name, isSecret, ok, tc.wantName, tc.wantSecret, tc.wantOK)
		}
	}
}

// The renderers and the parser are two halves of one convention, so they have to agree.
func TestReferences_RoundTrip(t *testing.T) {
	const derived = "APPLICATION_MY_APP_CLIENT_SECRET"

	name, isSecret, ok := ParseReference(SecretReference(derived))
	if !ok || isSecret != true || name != derived {
		t.Errorf("a secret reference should parse back as a secret, got (%q, %v, %v)", name, isSecret, ok)
	}

	name, isSecret, ok = ParseReference(VariableReference(derived))
	if !ok || isSecret != false || name != derived {
		t.Errorf("a variable reference should parse back as a variable, got (%q, %v, %v)", name, isSecret, ok)
	}
}
