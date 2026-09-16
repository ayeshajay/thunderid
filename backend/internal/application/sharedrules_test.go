// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strings"
	"testing"
)

// payload builds an application document with the given OAuth config fragment.
func payload(name, oauth string) []byte {
	return []byte(`{"name":"` + name + `","type":"fullstack",` +
		`"inboundAuthConfig":[{"type":"oauth2","config":{` + oauth + `}}]}`)
}

// The same rules run on both planes, so a document that breaks one is refused wherever it is sent.
func TestSharedRules_RefuseWhatIsWrongOnAnyPlane(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload []byte
		wants   string
	}{
		{
			"a redirecting client with nowhere to redirect",
			payload("App", `"grantTypes":["authorization_code"]`),
			"requires redirect URIs",
		},
		{
			"a redirect URI on a client that never redirects",
			payload("App", `"grantTypes":["client_credentials"],"redirectUris":["https://a/cb"]`),
			"only used by the authorization_code grant",
		},
		{
			"a public client holding a secret",
			payload("Public", `"grantTypes":["authorization_code"],"redirectUris":["https://a/cb"],`+
				`"publicClient":true,"clientSecret":"s"`),
			"public client cannot hold a client secret",
		},
		{
			"PKCE without the grant it applies to",
			payload("App", `"grantTypes":["client_credentials"],"pkceRequired":true`),
			"PKCE applies only",
		},
		{
			"a URL that is not one",
			[]byte(`{"name":"App","url":"not a url"}`),
			"not a valid URI",
		},
		{
			"no name",
			[]byte(`{"url":"https://app.example"}`),
			"name is required",
		},
	} {
		err := (sharedRules{}).Validate(context.Background(), tc.payload)
		if err == nil {
			t.Errorf("%s: should have been refused", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wants) {
			t.Errorf("%s: error = %q, want it to mention %q", tc.name, err, tc.wants)
		}
	}
}

// A reference stands in for a value the gateway supplies. Checking it as though it were the value
// would refuse the document for not carrying what it was deliberately not given.
func TestSharedRules_ToleratesAReferenceWhereAValueBelongs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{
			"a referenced redirect URI",
			payload("Referenced", `"grantTypes":["authorization_code"],`+
				`"redirectUris":["var:APPLICATION_APP_REDIRECT_URIS"]`),
		},
		{
			"a referenced home URL",
			[]byte(`{"name":"App","url":"var:APPLICATION_APP_URL"}`),
		},
	} {
		if err := (sharedRules{}).Validate(context.Background(), tc.payload); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// The same document, with real values, is accepted too: the rules do not depend on which plane asks.
func TestSharedRules_AcceptRealValues(t *testing.T) {
	good := payload("Real",
		`"grantTypes":["authorization_code"],"redirectUris":["https://app.example/cb"],`+
			`"tokenEndpointAuthMethod":"client_secret_basic"`)
	if err := (sharedRules{}).Validate(context.Background(), good); err != nil {
		t.Errorf("a valid application should be accepted: %v", err)
	}
}
