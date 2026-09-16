/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/gateway/model"
)

// secretsFixture builds an gateway holding one credential of each kind: an application's client
// secret, which is only ever verified, and a connection's, which is replayed to the provider.
func secretsFixture(t *testing.T) (*Service, *fakeClient, model.Gateway) {
	t.Helper()
	fake := &fakeClient{
		exportResources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"clientSecret: {{.APPLICATION_APP_A_CLIENT_SECRET}}\n" +
			"---\nresource_type: connection\nid: conn-a\nname: conn-a\n" +
			"clientSecret: {{.CONNECTION_CONN_A_CLIENT_SECRET}}",
		secrets: map[string]string{
			"APPLICATION_APP_A_CLIENT_SECRET": "x",
			"CONNECTION_CONN_A_CLIENT_SECRET": "y",
		},
		secretNames: []string{"CONNECTION_CONN_A_CLIENT_SECRET"},
	}
	svc := newTestService(t, fake)
	env, err := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})
	if err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	if _, err := svc.CaptureVersion(context.Background(), ""); err != nil {
		t.Fatalf("capture: %v", err)
	}
	return svc, fake, env.Gateway
}

func TestListSecretsClassifiesByResourceAndReportsWhatTheDataPlaneHolds(t *testing.T) {
	svc, _, env := secretsFixture(t)

	list, err := svc.ListSecrets(context.Background(), env.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !list.Checked {
		t.Fatal("the data plane's store answered, so held is a judgement")
	}
	byName := map[string]SecretEntry{}
	for _, entry := range list.Secrets {
		byName[entry.Name] = entry
	}

	app := byName["APPLICATION_APP_A_CLIENT_SECRET"]
	// An application's client secret is only ever compared against what a client presents.
	if app.Kind != KindHash {
		t.Fatalf("an application client secret must be hashed, got %q", app.Kind)
	}
	if app.Held {
		t.Fatal("the data plane does not hold it, so it must be reported as missing")
	}
	if app.ResourceType != "application" || app.ResourceName != "app-a" {
		t.Fatalf("the owning resource must be reported, got %+v", app)
	}

	conn := byName["CONNECTION_CONN_A_CLIENT_SECRET"]
	// The same field name on a connection is handed to the upstream provider, so it cannot be hashed.
	if conn.Kind != KindValue {
		t.Fatalf("a connection client secret must stay readable, got %q", conn.Kind)
	}
	if !conn.Held {
		t.Fatal("the data plane holds it")
	}
}

// Every credential is stored as its value, whether it is one that is only ever verified or one that is
// replayed to a provider. Hashing is the importing API's job, so the store keeps what a resource needs.
func TestSetSecretStoresEveryCredentialAsItsValue(t *testing.T) {
	svc, fake, env := secretsFixture(t)
	ctx := context.Background()

	for name, value := range map[string]string{
		"APPLICATION_APP_A_CLIENT_SECRET": "chosen",
		"CONNECTION_CONN_A_CLIENT_SECRET": "provider-issued",
	} {
		if _, err := svc.SetSecret(ctx, env.ID, name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
		written := fake.kvWrites[name]
		if written["kind"] != KindValue || written["value"] != value {
			t.Fatalf("%s must be stored as its value, got %v", name, written)
		}
	}
}

func TestRegenerateSecretIssuesAValueAndReturnsItOnce(t *testing.T) {
	svc, fake, env := secretsFixture(t)

	entry, value, err := svc.RegenerateSecret(context.Background(), env.ID, "APPLICATION_APP_A_CLIENT_SECRET")
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if value == "" {
		t.Fatal("the new credential has to be returned, because this is the only time it is shown")
	}
	if !entry.Held {
		t.Fatal("the secret was just written, so it is held")
	}
	written := fake.kvWrites["APPLICATION_APP_A_CLIENT_SECRET"]
	if written["value"] != value {
		t.Fatalf("the stored credential must be the returned value, got %v", written)
	}
}

func TestRegenerateSecretRefusesACredentialThatIsReplayed(t *testing.T) {
	svc, _, env := secretsFixture(t)

	// A random value here would simply not be the credential the provider issued.
	_, _, err := svc.RegenerateSecret(context.Background(), env.ID, "CONNECTION_CONN_A_CLIENT_SECRET")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected a validation error, got %v", err)
	}
}

// A credential is stored as its value even when it is one that is only ever verified.
//
// The data plane fills the placeholder with it at import and writes the resource through its own API,
// which hashes it on the way in. Storing a hash here instead would leave nothing able to fill that
// placeholder, because a hash cannot be turned back into the value the resource needs.
func TestSetSecretStoresAVerifiableCredentialAsItsValue(t *testing.T) {
	svc, fake, env := secretsFixture(t)

	if _, err := svc.SetSecret(context.Background(), env.ID,
		"APPLICATION_APP_A_CLIENT_SECRET", "chosen"); err != nil {
		t.Fatalf("set secret: %v", err)
	}

	written, ok := fake.kvWrites["APPLICATION_APP_A_CLIENT_SECRET"]
	if !ok {
		t.Fatal("expected the credential to be written")
	}
	if written["kind"] != KindValue {
		t.Fatalf("expected the credential stored as its value, got %v", written["kind"])
	}
	if written["value"] != "chosen" {
		t.Fatalf("expected the credential itself, got %v", written["value"])
	}
}

func TestSecretKindFallsBackToTheNameWhenTheBundleDoesNotExplainIt(t *testing.T) {
	// A key recorded at capture time whose resource has since left the bundle still has to be settable,
	// and settable as the right kind.
	for name, want := range map[string]string{
		"APPLICATION_APP_A_CLIENT_SECRET": KindHash,
		"USER_ADMIN_PASSWORD":             KindHash,
		"CONNECTION_TWILIO_AUTH_TOKEN":    KindValue,
		"CONNECTION_X_CLIENT_SECRET":      KindValue,
		"NOTIFICATION_SENDER_API_KEY":     KindValue,
	} {
		if got := kindFromName(name); got != want {
			t.Fatalf("%s: expected %q, got %q", name, want, got)
		}
	}
}

func TestListSecretsFallsBackToWhatTheDataPlaneLastReported(t *testing.T) {
	fake := &fakeClient{secretNames: []string{"APPLICATION_APP_A_CLIENT_SECRET"}}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(),
		Resources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"clientSecret: {{.APPLICATION_APP_A_CLIENT_SECRET}}",
		SecretKeys: []string{"APPLICATION_APP_A_CLIENT_SECRET"},
	})

	// While the connection is here, the names are recorded.
	if _, err := svc.ListSecrets(context.Background(), env.ID); err != nil {
		t.Fatalf("list with a connection: %v", err)
	}
	stored, _ := svc.store.GetGateway(context.Background(), env.ID)
	if len(stored.SecretNames) == 0 || stored.SecretNamesAt.IsZero() {
		t.Fatal("expected the reported credentials to be recorded for other pods")
	}

	// With no connection, the recorded names still answer.
	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: false})
	list, err := svc.ListSecrets(context.Background(), env.ID)
	if err != nil {
		t.Fatalf("expected the recorded names to answer, got %v", err)
	}
	if !list.Checked {
		t.Fatal("expected the listing to report that the credentials are known")
	}
}

// A pod that cannot reach the data plane queues the question rather than reporting the credentials
// as unknown forever, and hands back the job to follow.
func TestListSecretsQueuesTheQuestionWhenThisPodCannotAsk(t *testing.T) {
	fake := &fakeClient{secretNames: []string{"APPLICATION_APP_A_CLIENT_SECRET"}}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(),
		Resources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"clientSecret: {{.APPLICATION_APP_A_CLIENT_SECRET}}",
		SecretKeys: []string{"APPLICATION_APP_A_CLIENT_SECRET"},
	})

	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: false})
	list, err := svc.ListSecrets(context.Background(), env.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Checked {
		t.Fatal("expected the credentials to be reported as not yet known")
	}
	if list.PendingJobID == "" {
		t.Fatal("expected a job to follow")
	}

	// The pod holding the connection carries it out, which records the answer.
	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: true})
	if err := svc.DeliverNext(context.Background(), "dev-dp"); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	stored, _ := svc.store.GetGateway(context.Background(), env.ID)
	if stored.SecretNamesAt.IsZero() || len(stored.SecretNames) != 1 {
		t.Fatalf("expected the answer to be recorded, got %v", stored.SecretNames)
	}

	// With it recorded, the pod that could not ask now answers.
	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: false})
	again, err := svc.ListSecrets(context.Background(), env.ID)
	if err != nil {
		t.Fatalf("second list: %v", err)
	}
	if !again.Checked {
		t.Fatal("expected the recorded answer to be used")
	}
}
