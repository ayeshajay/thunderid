// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// doc decodes an application document for the fill to work on.
func doc(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var document map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

// oauthOf returns the OAuth config of the first inbound entry.
func oauthOf(t *testing.T, document map[string]interface{}) map[string]interface{} {
	t.Helper()
	configs, ok := document["inboundAuthConfig"].([]interface{})
	if !ok || len(configs) == 0 {
		t.Fatal("the document has no inbound auth config")
	}
	return configs[0].(map[string]interface{})["config"].(map[string]interface{})
}

// The caller leaves the deployment's fields out and the backend puts a reference in their place, so
// the caller neither has to know the naming nor can get it wrong.
func TestFill_PutsAReferenceWhereADeploymentValueBelongs(t *testing.T) {
	document := doc(t, `{"name":"Storefront","inboundAuthConfig":[{"type":"oauth2","config":{
		"clientId":"storefront","grantTypes":["authorization_code"],
		"tokenEndpointAuthMethod":"client_secret_basic"}}]}`)

	if err := (deploymentFields{}).Fill(context.Background(), "Storefront", document); err != nil {
		t.Fatal(err)
	}

	oauth := oauthOf(t, document)
	redirects, _ := oauth["redirectUris"].([]interface{})
	if len(redirects) != 1 || redirects[0] != "var:APPLICATION_STOREFRONT_REDIRECT_URIS" {
		t.Errorf("redirect URIs = %v, want the derived reference", oauth["redirectUris"])
	}
	if oauth["clientSecret"] != "sec:APPLICATION_STOREFRONT_CLIENT_SECRET" {
		t.Errorf("client secret = %v, want the derived reference", oauth["clientSecret"])
	}
	if oauth["clientId"] != "storefront" {
		t.Error("a field the document does own must be left alone")
	}
}

// A value supplied for one is refused rather than overwritten: accepting it and quietly replacing it
// would tell the caller their value was stored when it had been discarded.
func TestFill_RefusesAValueForADeploymentField(t *testing.T) {
	for _, tc := range []struct{ name, config, wants string }{
		{
			"a redirect URI",
			`"grantTypes":["authorization_code"],"redirectUris":["https://storefront.example/cb"]`,
			"Redirect URIs are not accepted here",
		},
		{
			"a client secret",
			`"grantTypes":["authorization_code"],"clientSecret":"a-real-secret"`,
			"client secret is not accepted here",
		},
	} {
		document := doc(t, `{"name":"Storefront","inboundAuthConfig":[{"config":{`+tc.config+`}}]}`)
		err := (deploymentFields{}).Fill(context.Background(), "Storefront", document)
		if err == nil {
			t.Errorf("%s should be refused", tc.name)
			continue
		}
		if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wants)) {
			t.Errorf("%s: error = %q, want it to mention %q", tc.name, err, tc.wants)
		}
	}
}

// A document read back and written again carries the references it was given, which is not a caller
// trying to set a value.
func TestFill_AcceptsTheReferencesItPutThere(t *testing.T) {
	document := doc(t, `{"name":"Storefront","inboundAuthConfig":[{"config":{
		"grantTypes":["authorization_code"],
		"redirectUris":["var:APPLICATION_STOREFRONT_REDIRECT_URIS"],
		"clientSecret":"sec:APPLICATION_STOREFRONT_CLIENT_SECRET"}}]}`)

	if err := (deploymentFields{}).Fill(context.Background(), "Storefront", document); err != nil {
		t.Fatalf("a re-submitted document should be accepted: %v", err)
	}
}

// A reference goes only where a value would: a client that never redirects gets none for a redirect
// URI, and a public client gets none for a credential it must not hold.
func TestFill_PlacesAReferenceOnlyWhereAValueBelongs(t *testing.T) {
	for _, tc := range []struct {
		name         string
		config       string
		wantRedirect bool
		wantSecret   bool
	}{
		{
			"a public browser client",
			`"grantTypes":["authorization_code"],"publicClient":true,"tokenEndpointAuthMethod":"none"`,
			true, false,
		},
		{
			"a machine to machine client",
			`"grantTypes":["client_credentials"],"tokenEndpointAuthMethod":"client_secret_basic"`,
			false, true,
		},
	} {
		document := doc(t, `{"name":"App","inboundAuthConfig":[{"config":{`+tc.config+`}}]}`)
		if err := (deploymentFields{}).Fill(context.Background(), "App", document); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		oauth := oauthOf(t, document)
		if _, present := oauth["redirectUris"]; present != tc.wantRedirect {
			t.Errorf("%s: redirect reference present = %v, want %v", tc.name, present, tc.wantRedirect)
		}
		if _, present := oauth["clientSecret"]; present != tc.wantSecret {
			t.Errorf("%s: secret reference present = %v, want %v", tc.name, present, tc.wantSecret)
		}
	}
}
