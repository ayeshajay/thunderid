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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/gateway/model"
	"github.com/thunder-id/thunderid/internal/gateway/store"
	"github.com/thunder-id/thunderid/internal/gateway/thunder"
)

const testAppB = "app-b"

// fakeClient records import calls and serves canned export/reveal data.
type fakeClient struct {
	exportResources string
	exportEnv       string
	secrets         map[string]string
	envVars         map[string]string
	secretNames     []string
	// secretNamesCalls counts how often the data plane was asked, so a test can show that pods do
	// not each ask for themselves.
	secretNamesCalls int
	kvWrites         map[string]map[string]interface{}
	kvExisting       map[string][2]string
	imports          []thunder.ImportRequest
}

func (f *fakeClient) Export(context.Context) (thunder.ExportResult, error) {
	return thunder.ExportResult{Resources: f.exportResources, EnvFile: f.exportEnv}, nil
}

func (f *fakeClient) SecretKeys(context.Context) ([]string, error) {
	keys := make([]string, 0, len(f.secrets))
	for k := range f.secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (f *fakeClient) GatewayVariables(context.Context, string) (map[string]string, error) {
	return f.envVars, nil
}

func (f *fakeClient) SecretNames(context.Context) ([]string, error) {
	f.secretNamesCalls++
	return f.secretNames, nil
}

func (f *fakeClient) PutSecret(_ context.Context, name string, body map[string]interface{}) error {
	if f.kvWrites == nil {
		f.kvWrites = map[string]map[string]interface{}{}
	}
	f.kvWrites[name] = body
	f.secretNames = append(f.secretNames, name)
	return nil
}

func (f *fakeClient) GetSecret(_ context.Context, name string) (string, string, bool, error) {
	if body, ok := f.kvWrites[name]; ok {
		kind, _ := body["kind"].(string)
		value, _ := body["value"].(string)
		return kind, value, true, nil
	}
	if v, ok := f.kvExisting[name]; ok {
		return v[0], v[1], true, nil
	}
	return "", "", false, nil
}

func (f *fakeClient) Import(_ context.Context, req thunder.ImportRequest) (*thunder.ImportResponse, error) {
	f.imports = append(f.imports, req)
	return &thunder.ImportResponse{Summary: &thunder.ImportSummary{}}, nil
}

func (f *fakeClient) lastImport() thunder.ImportRequest {
	return f.imports[len(f.imports)-1]
}

// fakeDataPlanes hands every gateway the same fake, standing in for the connections data planes
// hold open to the control plane.
type fakeDataPlanes struct {
	plane     DataPlane
	connected bool
}

func (f *fakeDataPlanes) For(dataPlaneID string) (DataPlane, error) {
	if !f.connected {
		return nil, fmt.Errorf("data plane %s is not connected", dataPlaneID)
	}
	return f.plane, nil
}

func (f *fakeDataPlanes) Status(string) model.DataPlaneStatus {
	return model.DataPlaneStatus{Connected: f.connected}
}

func newTestService(t *testing.T, fake *fakeClient) *Service {
	t.Helper()
	svc := New(newMemStore(), func(string, thunder.Credentials, string) ThunderClient { return fake })
	svc.SetWorkspaceURL("https://cp")
	svc.SetOrganization("org1")
	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: true})
	svc.SetSecretSealer(fakeSealer{})
	return svc
}

// app returns a minimal application document.
func app(id string) string {
	return fmt.Sprintf("resource_type: application\nid: %s\nname: %s", id, id)
}

func bundleOf(ids ...string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, app(id))
	}
	return strings.Join(parts, "\n---\n")
}

func TestApplyTracksDeletions(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)

	env, err := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	if err != nil {
		t.Fatalf("create env: %v", err)
	}

	if _, err := svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB), nil, "v1"); err != nil {
		t.Fatalf("upload v1: %v", err)
	}
	res, err := svc.Apply(context.Background(), env.ID, "latest", false)
	if err != nil {
		t.Fatalf("apply v1: %v", err)
	}
	if len(fake.lastImport().Deletions) != 0 {
		t.Fatalf("first apply should have no deletions: %+v", fake.lastImport().Deletions)
	}
	if res.TargetSeq != 1 {
		t.Fatalf("expected target seq 1, got %d", res.TargetSeq)
	}

	// v2 removes app-b -> apply must request its deletion.
	if _, err := svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v2"); err != nil {
		t.Fatalf("upload v2: %v", err)
	}
	if _, err := svc.Apply(context.Background(), env.ID, "latest", false); err != nil {
		t.Fatalf("apply v2: %v", err)
	}
	dels := fake.lastImport().Deletions
	if len(dels) != 1 || dels[0].ResourceType != "application" || dels[0].ID != testAppB {
		t.Fatalf("expected deletion of application app-b, got %+v", dels)
	}

	// Applied version must be recorded on the gateway.
	got, _ := svc.GetGateway(context.Background(), env.ID)
	if got.AppliedSeq != 2 {
		t.Fatalf("expected appliedSeq 2, got %d", got.AppliedSeq)
	}
}

func TestApplyDryRunDoesNotRecord(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v1")

	if _, err := svc.Apply(context.Background(), env.ID, "latest", true); err != nil {
		t.Fatalf("dry run apply: %v", err)
	}
	if !fake.lastImport().DryRun {
		t.Fatalf("expected dryRun propagated to import")
	}
	got, _ := svc.GetGateway(context.Background(), env.ID)
	if got.AppliedSeq != 0 {
		t.Fatalf("dry run must not record appliedSeq, got %d", got.AppliedSeq)
	}
}

func TestCaptureRecordsSecretKeysWithoutValues(t *testing.T) {
	fake := &fakeClient{
		exportResources: bundleOf("app-a"),
		exportEnv:       "APP_A_CLIENT_ID=abc\nAPP_A_CLIENT_SECRET=from-export",
		secrets:         map[string]string{"APP_A_CLIENT_SECRET": "never-read"},
	}
	svc := newTestService(t, fake)
	_, _ = svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})

	v, err := svc.CaptureVersion(context.Background(), "captured")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	full, _ := svc.GetVersion(context.Background(), v.Seq)

	if full.Variables["APP_A_CLIENT_ID"] != "abc" {
		t.Fatalf("expected the non-secret variable to be carried over")
	}
	// A secret's value must not be stored, even when the export happened to include one.
	if _, present := full.Variables["APP_A_CLIENT_SECRET"]; present {
		t.Fatalf("secret value was stored: %#v", full.Variables)
	}
	if len(full.SecretKeys) != 1 || full.SecretKeys[0] != "APP_A_CLIENT_SECRET" {
		t.Fatalf("expected the secret key to be recorded, got %v", full.SecretKeys)
	}
}

// A gateway's own variables are not folded into a capture: the version belongs to the organization,
// and the value that should reach a data plane is the one configured against the gateway being
// applied to. So the configured value wins at apply time, over whatever the export happened to carry.
func TestApplyLetsAGatewaysVariablesOverrideTheExport(t *testing.T) {
	fake := &fakeClient{
		exportResources: bundleOf("app-a"),
		exportEnv:       "APP_A_REDIRECT_URIS=[\"https://stale\"]",
		envVars:         map[string]string{"APP_A_REDIRECT_URIS": `["https://configured"]`},
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})

	v, _ := svc.CaptureVersion(context.Background(), "")
	// The capture carries only what the export gave it.
	full, _ := svc.GetVersion(context.Background(), v.Seq)
	if full.Variables["APP_A_REDIRECT_URIS"] != `["https://stale"]` {
		t.Fatalf("capture should carry the export's own value, got %q", full.Variables["APP_A_REDIRECT_URIS"])
	}

	// Resolving for the gateway being applied to is where its own value takes precedence.
	values := svc.resolveVariables(context.Background(), env.Gateway, full)
	if values["APP_A_REDIRECT_URIS"] != `["https://configured"]` {
		t.Fatalf("the gateway's configured value should win at apply, got %q", values["APP_A_REDIRECT_URIS"])
	}
}

func TestApplyOmitsSecretsSoTheDataPlaneFillsThem(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})

	resources := "resource_type: application\nid: app-a\nname: app-a\nclientSecret: {{.APP_A_CLIENT_SECRET}}"
	stored, err := svc.store.AddVersion(context.Background(), model.Version{
		Origin:     model.OriginUploaded,
		CreatedAt:  svc.now().UTC(),
		Resources:  resources,
		Variables:  map[string]string{},
		SecretKeys: []string{"APP_A_CLIENT_SECRET"},
	})
	if err != nil {
		t.Fatalf("add version: %v", err)
	}
	if _, err := svc.Apply(context.Background(), env.ID, strconv.Itoa(stored.Seq), false); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if _, present := fake.lastImport().Variables["APP_A_CLIENT_SECRET"]; present {
		t.Fatalf("a secret must be omitted so the data plane resolves it, got %#v",
			fake.lastImport().Variables["APP_A_CLIENT_SECRET"])
	}
	// The placeholder itself must survive in the content for the data plane to fill.
	if !strings.Contains(fake.lastImport().Content, "{{.APP_A_CLIENT_SECRET}}") {
		t.Fatalf("the placeholder should remain in the content: %s", fake.lastImport().Content)
	}
}

// A capture reads the organization's workspace, so a service that does not know where its control
// plane answers has nothing to read and says so rather than storing an empty version.
func TestCaptureRequiresAWorkspace(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	svc.SetWorkspaceURL("")
	_, _ = svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	if _, err := svc.CaptureVersion(context.Background(), ""); !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("expected ErrNoWorkspace, got %v", err)
	}
}

func TestApplyPicksUpVariablesAddedAfterCapture(t *testing.T) {
	// The control plane has nothing configured at capture time.
	fake := &fakeClient{
		exportResources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"redirectUris:\n  {{- range .APP_A_REDIRECT_URIS}}\n  - {{.}}\n  {{- end}}",
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})
	if _, err := svc.CaptureVersion(context.Background(), ""); err != nil {
		t.Fatalf("capture: %v", err)
	}

	status, err := svc.CheckVariables(context.Background(), env.ID, "latest")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(status.Missing) != 1 || status.Missing[0] != "APP_A_REDIRECT_URIS" {
		t.Fatalf("expected the redirect URIs to be reported missing, got %v", status.Missing)
	}

	// The operator now sets it in the control plane, without re-capturing.
	fake.envVars = map[string]string{"APP_A_REDIRECT_URIS": `["https://dp/callback"]`}

	status, err = svc.CheckVariables(context.Background(), env.ID, "latest")
	if err != nil {
		t.Fatalf("re-check: %v", err)
	}
	if len(status.Missing) != 0 {
		t.Fatalf("the new value should clear the warning, got %v", status.Missing)
	}

	if _, err := svc.Apply(context.Background(), env.ID, "latest", false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	arr, ok := fake.lastImport().Variables["APP_A_REDIRECT_URIS"].([]interface{})
	if !ok || len(arr) != 1 || arr[0] != "https://dp/callback" {
		t.Fatalf("apply should use the live value, got %#v", fake.lastImport().Variables["APP_A_REDIRECT_URIS"])
	}
}

// A selection is remembered as the target's exclusions rather than becoming a version of its own:
// the version belongs to the organization, and what differs per gateway is what it holds back.
func TestPromoteSelectiveHoldsTheRestBackOnTheTarget(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dev"}})
	prod, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "prod",
		Target: model.Target{DataPlaneID: "prod"}})

	captured, _ := svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB), nil, "v1")

	result, err := svc.Promote(context.Background(), PromoteInput{
		FromGatewayID: dev.ID, ToGatewayID: prod.ID, Selection: []string{"application/id:app-a"},
		SelectionProvided: true,
	})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if result.Preview.Summary.Added != 2 {
		t.Fatalf("preview should show 2 additions, got %+v", result.Preview.Summary)
	}
	// The organization's version is what the target moves onto; promoting created none.
	if result.Seq != captured.Seq {
		t.Fatalf("promote should move onto version %d, got %d", captured.Seq, result.Seq)
	}
	versions, _ := svc.ListVersions(context.Background())
	if len(versions) != 1 {
		t.Fatalf("promotion must not create a version, got %d", len(versions))
	}
	got, _ := svc.GetGateway(context.Background(), prod.ID)
	if len(got.Excluded) != 1 || !strings.Contains(got.Excluded[0], "app-b") {
		t.Fatalf("app-b should be held back on the target, got %+v", got.Excluded)
	}
}

func TestPromoteAllAndApply(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dev"}})
	prod, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "prod",
		Target: model.Target{DataPlaneID: "prod"}})
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB), nil, "v1")

	result, err := svc.Promote(context.Background(),
		PromoteInput{FromGatewayID: dev.ID, ToGatewayID: prod.ID, Apply: true})
	if err != nil {
		t.Fatalf("promote+apply: %v", err)
	}
	if result.Applied == nil {
		t.Fatalf("expected apply result")
	}
	got, _ := svc.GetGateway(context.Background(), prod.ID)
	if got.AppliedSeq != result.Seq {
		t.Fatalf("prod applied seq %d != promoted version %d", got.AppliedSeq, result.Seq)
	}
	// Applying is what puts an entry in the gateway's own history.
	history, _ := svc.GatewayHistory(context.Background(), prod.ID)
	if len(history) != 1 || history[0].Seq != result.Seq {
		t.Fatalf("expected one history entry for version %d, got %+v", result.Seq, history)
	}
}

// Reverting moves the gateway back onto a version rather than creating one: the stream belongs to
// the organization, and going back is a change of which version this gateway runs.
func TestRevertMovesTheGatewayBackWithoutCreatingAVersion(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v1")
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB), nil, "v2")
	if _, err := svc.Apply(context.Background(), env.ID, "2", false); err != nil {
		t.Fatalf("apply v2: %v", err)
	}

	result, err := svc.Revert(context.Background(), RevertInput{GatewayID: env.ID, ToRef: "1"})
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if result.Seq != 1 {
		t.Fatalf("revert should target version 1, got %d", result.Seq)
	}
	versions, _ := svc.ListVersions(context.Background())
	if len(versions) != 2 {
		t.Fatalf("revert must not create a version, got %d", len(versions))
	}
	// Preview reflects removing app-b (running v2 -> target v1).
	if result.Preview.Summary.Deleted != 1 {
		t.Fatalf("expected one deletion in revert preview, got %+v", result.Preview.Summary)
	}
}

// "previous" is what this gateway ran before its current version, read from its own history rather
// than from the organization's stream: a version captured in between was never on this gateway.
func TestRevertToPreviousReadsTheGatewaysOwnHistory(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v1")
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB), nil, "v2")
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB, "app-c"), nil, "v3")

	// This gateway ran v1 and then v3; v2 never reached it, so going back means v1.
	if _, err := svc.Apply(context.Background(), env.ID, "1", false); err != nil {
		t.Fatalf("apply v1: %v", err)
	}
	if _, err := svc.Apply(context.Background(), env.ID, "3", false); err != nil {
		t.Fatalf("apply v3: %v", err)
	}

	result, err := svc.Revert(context.Background(), RevertInput{GatewayID: env.ID, ToRef: "previous"})
	if err != nil {
		t.Fatalf("revert to previous: %v", err)
	}
	if result.Seq != 1 {
		t.Fatalf("previous should be version 1, the last one this gateway ran, got %d", result.Seq)
	}
}

func TestRevertToPreviousRequiresSomethingEarlierInTheHistory(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v1")
	if _, err := svc.Apply(context.Background(), env.ID, "1", false); err != nil {
		t.Fatalf("apply v1: %v", err)
	}

	if _, err := svc.Revert(context.Background(),
		RevertInput{GatewayID: env.ID, ToRef: "previous"}); !errors.Is(err, ErrNoPreviousVersion) {
		t.Fatalf("expected ErrNoPreviousVersion, got %v", err)
	}
}

func TestApplyAllPushesEveryGateway(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	_, _ = svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev"},
	})
	_, _ = svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "prod", Target: model.Target{DataPlaneID: "prod"},
	})
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v1")
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v1")

	results := svc.ApplyAll(context.Background(), false)

	if len(results) != 2 {
		t.Fatalf("expected a result per gateway, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != "" || r.Applied == nil {
			t.Fatalf("%s should have applied, got error %q", r.GatewayName, r.Error)
		}
	}
	if len(fake.imports) != 2 {
		t.Fatalf("expected two imports, got %d", len(fake.imports))
	}
}

// With versions owned by the organization there is no gateway "without a version": every gateway can
// be applied to as soon as the organization has captured one. What is still worth reporting is an
// organization that has captured nothing at all.
func TestApplyAllReportsWhenTheOrganizationHasNoVersion(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	one, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "one", Target: model.Target{DataPlaneID: "one"},
	})
	two, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "two", Target: model.Target{DataPlaneID: "two"},
	})

	results := svc.ApplyAll(context.Background(), false)
	byID := map[string]ApplyAllResult{}
	for _, r := range results {
		byID[r.GatewayID] = r
	}
	for _, id := range []string{one.ID, two.ID} {
		if byID[id].Error == "" {
			t.Fatalf("a gateway with nothing to apply should say so, got %+v", byID[id])
		}
	}

	// Once the organization has a version, every gateway takes it.
	_, _ = svc.UploadVersion(context.Background(), bundleOf("app-a"), nil, "v1")
	results = svc.ApplyAll(context.Background(), false)
	for _, r := range results {
		if r.Applied == nil {
			t.Fatalf("every gateway should apply the organization's version, got %+v", r)
		}
	}
}

func TestCheckVariablesReportsSecretsTheDataPlaneLacks(t *testing.T) {
	// The data plane holds one of the two secrets the configuration needs.
	fake := &fakeClient{
		exportResources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"clientSecret: {{.APP_A_CLIENT_SECRET}}\nother: {{.APP_B_CLIENT_SECRET}}",
		secrets:     map[string]string{"APP_A_CLIENT_SECRET": "x", "APP_B_CLIENT_SECRET": "y"},
		secretNames: []string{"APP_A_CLIENT_SECRET"},
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev",
		Target: model.Target{
			DataPlaneID: "dp",
		},
	})
	if _, err := svc.CaptureVersion(context.Background(), ""); err != nil {
		t.Fatalf("capture: %v", err)
	}

	status, err := svc.CheckVariables(context.Background(), env.ID, "latest")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !status.SecretsChecked {
		t.Fatal("the secret service was configured, so it should have been consulted")
	}
	if len(status.MissingSecrets) != 1 || status.MissingSecrets[0] != "APP_B_CLIENT_SECRET" {
		t.Fatalf("expected only the absent secret to be reported, got %v", status.MissingSecrets)
	}
}

func TestCheckVariablesUsesTheDataPlanesOwnSecretStore(t *testing.T) {
	fake := &fakeClient{
		exportResources: "resource_type: application\nid: app-a\nname: app-a\n" +
			"clientSecret: {{.APP_A_CLIENT_SECRET}}",
		secrets:     map[string]string{"APP_A_CLIENT_SECRET": "x"},
		secretNames: []string{"APP_A_CLIENT_SECRET"},
	}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "dp"},
	})
	_, _ = svc.CaptureVersion(context.Background(), "")

	status, _ := svc.CheckVariables(context.Background(), env.ID, "latest")
	// With no separate service named, the data plane's own store answers, so the check is real.
	if !status.SecretsChecked {
		t.Fatal("the data plane's own secret store should have been consulted")
	}
	if len(status.MissingSecrets) != 0 {
		t.Fatalf("the store holds the secret, got %v", status.MissingSecrets)
	}
}

func TestPromoteLeavesTheDestinationsCredentialsAlone(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev"},
	})
	stage, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "stage",
		Target: model.Target{
			DataPlaneID: "stage",
		},
	})

	// A version whose configuration needs one secret.
	if _, err := svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(),
		Resources:  "resource_type: application\nid: app-a\nname: app-a",
		Variables:  map[string]string{},
		SecretKeys: []string{"APP_A_CLIENT_SECRET"},
	}); err != nil {
		t.Fatalf("add version: %v", err)
	}

	result, err := svc.Promote(context.Background(), PromoteInput{FromGatewayID: dev.ID, ToGatewayID: stage.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Nothing invents a credential for the destination: it is named as one the destination has to hold,
	// and set against its own data plane.
	if len(result.Secrets.Skipped) != 1 {
		t.Fatalf("a promote must name what the destination still needs, got %+v", result.Secrets)
	}
	// A promotion is started from the source, so it must not reach into the destination's secret store:
	// those credentials arrive by the destination's own control plane capturing them.
	if len(fake.kvWrites) != 0 {
		t.Fatalf("a promote must not write to the destination's secret store, got %v", fake.kvWrites)
	}

	// A promotion moves a version onto the destination and writes nothing anywhere else: the
	// organization has one workspace, and the promoted configuration reaches a deployment only when it
	// is applied.
	if len(fake.imports) != 0 {
		t.Fatalf("a promote must import nothing on its own, got %v", fake.imports)
	}
}

func TestPromoteNeverReadsTheDestinationsSecretStore(t *testing.T) {
	// The destination holds a credential of its own. A promote must neither consult it nor replace it:
	// the destination's secret store belongs to the destination, and the source cannot manage it.
	fake := &fakeClient{kvExisting: map[string][2]string{"APP_A_CLIENT_SECRET": {"hash", "the-hash"}}}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev"},
	})
	stage, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "stage",
		Target: model.Target{
			DataPlaneID: "stage",
		},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(),
		Resources: "resource_type: application\nid: app-a\nname: app-a",
		Variables: map[string]string{}, SecretKeys: []string{"APP_A_CLIENT_SECRET"},
	})

	_, err := svc.Promote(context.Background(), PromoteInput{FromGatewayID: dev.ID, ToGatewayID: stage.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	if len(fake.kvWrites) != 0 {
		t.Fatalf("the destination's credential must be left alone, got %v", fake.kvWrites)
	}
}

// A credential is created once, in the organization's workspace, and belongs to the one gateway
// the control plane administers directly. Sending it everywhere would set the credential running in
// production from a change made while developing.
func TestCaptureSecretReachesOnlyTheControlPlaneManagedGateway(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "prod", Target: model.Target{DataPlaneID: "prod-dp"},
	})

	// The first gateway takes the mark, so a credential has somewhere to go from the outset.
	stored, _ := svc.GetGateway(context.Background(), dev.ID)
	if !stored.ManagedByControlPlane {
		t.Fatal("the organization's first gateway must be the one the control plane manages")
	}

	delivered, err := svc.CaptureSecretForTenant(context.Background(), "acme", "MY_SECRET",
		map[string]interface{}{"kind": "hash", "value": "h", "algorithm": "PBKDF2"})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("expected the secret sent to one gateway, got %d", delivered)
	}
	if fake.kvWrites["MY_SECRET"]["kind"] != "hash" {
		t.Fatalf("expected the credential stored, got %#v", fake.kvWrites)
	}
}

// The mark moves, and it moves rather than toggling: an organization is never left without one.
func TestSetManagedGatewayMovesTheMark(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	prod, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "prod", Target: model.Target{DataPlaneID: "prod-dp"},
	})

	if _, err := svc.SetManagedGateway(context.Background(), prod.ID); err != nil {
		t.Fatalf("set managed: %v", err)
	}

	movedTo, _ := svc.GetGateway(context.Background(), prod.ID)
	movedFrom, _ := svc.GetGateway(context.Background(), dev.ID)
	if !movedTo.ManagedByControlPlane {
		t.Fatal("the named gateway should hold the mark")
	}
	if movedFrom.ManagedByControlPlane {
		t.Fatal("the gateway that held it should have given it up")
	}
}

// Removing the marked gateway hands the mark to what is left, so a credential created afterwards
// still has somewhere to go.
func TestDeletingTheManagedGatewayPassesTheMarkOn(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	stage, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "stage", Target: model.Target{DataPlaneID: "stage-dp"},
	})

	if err := svc.DeleteGateway(context.Background(), dev.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	successor, _ := svc.GetGateway(context.Background(), stage.ID)
	if !successor.ManagedByControlPlane {
		t.Fatal("the mark should have passed to the gateway left lowest in the chain")
	}
}

// An organization with no gateway yet is not an error: there is simply nowhere to send it, and
// the credential is recreated when one is registered.
func TestCaptureSecretWithNoGatewaysDeliversNothing(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	if n, err := svc.CaptureSecretForTenant(context.Background(), "acme", "X",
		map[string]interface{}{"kind": "value", "value": "v"}); err != nil || n != 0 {
		t.Fatalf("expected zero deliveries and no error, got %d %v", n, err)
	}
}

// A gateway keeps only its most recent applies, and a version one of them still names survives
// pruning however old it is: otherwise going back would land on a version that is gone.
func TestGatewayHistoryIsBoundedAndKeepsWhatItNames(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})

	for i := 1; i <= 6; i++ {
		if _, err := svc.UploadVersion(context.Background(), bundleOf("app-"+strconv.Itoa(i)), nil, "v"); err != nil {
			t.Fatalf("upload: %v", err)
		}
		if _, err := svc.Apply(context.Background(), env.ID, strconv.Itoa(i), false); err != nil {
			t.Fatalf("apply v%d: %v", i, err)
		}
	}

	history, err := svc.GatewayHistory(context.Background(), env.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != store.KeepApplies {
		t.Fatalf("expected %d history entries, got %d", store.KeepApplies, len(history))
	}
	// Newest first: the last three applied, 6, 5 and 4.
	for i, want := range []int{6, 5, 4} {
		if history[i].Seq != want {
			t.Fatalf("history[%d] should name version %d, got %d", i, want, history[i].Seq)
		}
	}

	// Every version the history still names is readable, so a revert to any of them resolves.
	for _, entry := range history {
		if _, err := svc.GetVersion(context.Background(), entry.Seq); err != nil {
			t.Fatalf("version %d is named by the history but was pruned: %v", entry.Seq, err)
		}
	}
}

func TestVersionHistoryPruned(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	_, _ = svc.CreateGateway(context.Background(), CreateGatewayInput{Name: "dev",
		Target: model.Target{DataPlaneID: "dp"}})
	for i := 0; i < 7; i++ {
		if _, err := svc.UploadVersion(context.Background(), bundleOf("app-"+strconv.Itoa(i)), nil,
			"v"); err != nil {
			t.Fatalf("upload: %v", err)
		}
	}
	versions, _ := svc.ListVersions(context.Background())
	if len(versions) != store.KeepVersions() {
		t.Fatalf("expected %d versions retained, got %d", store.KeepVersions(), len(versions))
	}
	// Newest first: seq 7 down to whatever the configured retention allows.
	oldest := 7 - store.KeepVersions() + 1
	if versions[0].Seq != 7 || versions[len(versions)-1].Seq != oldest {
		t.Fatalf("unexpected retained range: %d..%d", versions[0].Seq, versions[len(versions)-1].Seq)
	}
}

// setupPromotionPair builds a dev -> prod pair with two applications in dev.
func setupPromotionPair(t *testing.T, fake *fakeClient) (*Service, model.Gateway, model.Gateway) {
	t.Helper()
	svc := newTestService(t, fake)
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev"}})
	prod, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "prod", Target: model.Target{DataPlaneID: "prod"}})
	if _, err := svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB), nil,
		"dev-v1"); err != nil {
		t.Fatalf("upload: %v", err)
	}
	return svc, dev.Gateway, prod.Gateway
}

func TestPromoteRemembersWhatWasHeldBack(t *testing.T) {
	svc, dev, prod := setupPromotionPair(t, &fakeClient{})

	// The user promotes app-a and deliberately leaves app-b behind.
	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromGatewayID: dev.ID, ToGatewayID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("promote: %v", err)
	}

	env, err := svc.GetGateway(context.Background(), prod.ID)
	if err != nil {
		t.Fatalf("get gateway: %v", err)
	}
	if len(env.Excluded) != 1 || env.Excluded[0] != "application/id:app-b" {
		t.Fatalf("the held back resource should have been recorded, got %v", env.Excluded)
	}
}

func TestPromoteKeepsHoldingBackOnALaterRun(t *testing.T) {
	fake := &fakeClient{}
	svc, dev, prod := setupPromotionPair(t, fake)

	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromGatewayID: dev.ID, ToGatewayID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("first promote: %v", err)
	}

	// dev changes again and the user promotes without expressing a preference. The earlier decision
	// stands: asking again every time is how a held back resource eventually slips through.
	_, err := svc.UploadVersion(context.Background(), bundleOf("app-a", testAppB, "app-c"), nil, "dev-v2")
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromGatewayID: dev.ID, ToGatewayID: prod.ID, Apply: true,
	}); err != nil {
		t.Fatalf("second promote: %v", err)
	}

	// The exclusion is honored where it matters: what reaches the data plane.
	content := fake.lastImport().Content
	if strings.Contains(content, testAppB) {
		t.Fatalf("app-b was held back earlier and must stay held back:\n%s", content)
	}
	if !strings.Contains(content, "app-c") {
		t.Fatalf("a resource nobody held back should promote:\n%s", content)
	}
}

func TestPromoteReleasesAResourceWhenItIsSelectedAgain(t *testing.T) {
	svc, dev, prod := setupPromotionPair(t, &fakeClient{})

	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromGatewayID: dev.ID, ToGatewayID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("first promote: %v", err)
	}

	// The user changes their mind and selects app-b this time.
	result, err := svc.Promote(context.Background(), PromoteInput{
		FromGatewayID: dev.ID, ToGatewayID: prod.ID,
		Selection: []string{"application/id:app-b"}, SelectionProvided: true,
	})
	if err != nil {
		t.Fatalf("second promote: %v", err)
	}

	full, _ := svc.GetVersion(context.Background(), result.Seq)
	if !strings.Contains(full.Resources, testAppB) {
		t.Fatalf("app-b was selected and should have promoted:\n%s", full.Resources)
	}
	env, _ := svc.GetGateway(context.Background(), prod.ID)
	for _, key := range env.Excluded {
		if key == "application/id:app-b" {
			t.Fatal("selecting a resource again must clear the record, or it would be held back forever")
		}
	}
}

func TestApplyLeavesAHeldBackResourceAlone(t *testing.T) {
	fake := &fakeClient{}
	svc, dev, prod := setupPromotionPair(t, fake)

	if _, err := svc.Promote(context.Background(), PromoteInput{
		FromGatewayID: dev.ID, ToGatewayID: prod.ID,
		Selection: []string{"application/id:app-a"}, SelectionProvided: true,
	}); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if _, err := svc.Apply(context.Background(), prod.ID, "latest", false); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Held back means left alone on the data plane: neither pushed nor deleted.
	if strings.Contains(fake.lastImport().Content, testAppB) {
		t.Fatalf("a held back resource must not be applied:\n%s", fake.lastImport().Content)
	}
	for _, d := range fake.lastImport().Deletions {
		if d.ID == testAppB {
			t.Fatal("holding a resource back means leaving it alone, not deleting it from the data plane")
		}
	}
}

// The promotion view shows whether each gateway's data plane is connected, because nothing can be
// applied or promoted to one that is not, and an operator should see that before starting.
func TestGatewaySummariesReportDataPlaneConnectivity(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	planes := &fakeDataPlanes{plane: fake, connected: true}
	svc.SetDataPlanes(planes)
	if _, err := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	summaries, err := svc.ListGatewaySummaries(context.Background())
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if !summaries[0].DataPlane.Connected {
		t.Fatal("a connected data plane should be reported as connected")
	}

	planes.connected = false
	summaries, err = svc.ListGatewaySummaries(context.Background())
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if summaries[0].DataPlane.Connected {
		t.Fatal("a data plane that dropped should be reported as disconnected")
	}
}

// An gateway that names no data plane cannot be applied to, and says so rather than failing at the
// transport with something that reads like an outage.
// A data plane this pod cannot reach is no longer a failure: the work is queued for whichever pod
// holds that connection, and the caller is given the id to collect the answer with.
func TestApplyQueuesWhenThisPodCannotReachTheDataPlane(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	svc.SetDataPlanes(&fakeDataPlanes{plane: fake, connected: false})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("a"),
	})

	result, err := svc.Apply(context.Background(), env.ID, "latest", false)

	if err != nil {
		t.Fatalf("expected the apply to be queued, got %v", err)
	}
	if result.JobID == "" {
		t.Fatal("expected a job id to collect the answer with")
	}
	if result.Status != store.JobPending {
		t.Fatalf("expected the job to be left pending, got %q", result.Status)
	}
	if result.Import != nil {
		t.Fatal("expected no import result, because nothing was delivered")
	}

	// Nothing was applied, so the gateway must not claim to hold the version.
	stored, _ := svc.store.GetGateway(context.Background(), env.ID)
	if stored.AppliedSeq != 0 {
		t.Fatalf("expected the gateway to record nothing applied, got %d", stored.AppliedSeq)
	}
}

// The pod that holds the connection finishes the work in the same request, so the caller gets its
// answer without polling.
func TestApplyDeliversInlineWhenThisPodHoldsTheConnection(t *testing.T) {
	fake := &fakeClient{}
	svc := newTestService(t, fake)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("a"),
	})

	result, err := svc.Apply(context.Background(), env.ID, "latest", false)

	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.Status != store.JobDone {
		t.Fatalf("expected the job to be finished, got %q", result.Status)
	}
	if result.Import == nil {
		t.Fatal("expected the data plane's answer in the same response")
	}

	stored, _ := svc.store.GetGateway(context.Background(), env.ID)
	if stored.AppliedSeq != result.TargetSeq {
		t.Fatalf("expected the gateway to record version %d applied, got %d",
			result.TargetSeq, stored.AppliedSeq)
	}
}

// fakeTokenIssuer records what it was asked to issue and hands back a predictable token.
type fakeTokenIssuer struct {
	issued []string
	n      int
}

func (f *fakeTokenIssuer) Issue(_ context.Context, dataPlaneID, _ string) (string, error) {
	f.issued = append(f.issued, dataPlaneID)
	f.n++
	return fmt.Sprintf("token-%d", f.n), nil
}

// Registering an gateway mints the credential its data plane connects with, and returns it once.
// Asking an operator to invent one and configure it on both sides is the step this removes.
func TestCreateGatewayIssuesTheDataPlaneToken(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	issuer := &fakeTokenIssuer{}
	svc.SetDataPlaneTokenIssuer(issuer)

	env, err := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name:   "dev",
		Target: model.Target{DataPlaneID: "org1:dev"},
	})

	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if env.DataPlaneToken != "token-1" {
		t.Fatalf("expected the issued token to be returned once, got %q", env.DataPlaneToken)
	}
	if len(issuer.issued) != 1 || issuer.issued[0] != "org1:dev" {
		t.Fatalf("expected a token for the gateway's data plane, got %v", issuer.issued)
	}
}

// A deployment using a single shared token issues none, and registering an gateway still works.
func TestCreateGatewayWithoutAnIssuerReturnsNoToken(t *testing.T) {
	svc := newTestService(t, &fakeClient{})

	env, err := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "org1:dev"},
	})

	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if env.DataPlaneToken != "" {
		t.Fatalf("expected no token, got %q", env.DataPlaneToken)
	}
}

// Rotation issues a new token for the same data plane.

// Rotation issues a new token for the same data plane.
func TestRegenerateDataPlaneTokenIssuesAnotherForTheSameDataPlane(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	issuer := &fakeTokenIssuer{}
	svc.SetDataPlaneTokenIssuer(issuer)
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "org1:dev"},
	})

	token, err := svc.RegenerateDataPlaneToken(context.Background(), env.ID)

	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if token != "token-2" {
		t.Fatalf("expected a freshly issued token, got %q", token)
	}
	if len(issuer.issued) != 2 || issuer.issued[1] != "org1:dev" {
		t.Fatalf("expected the same data plane to be reissued, got %v", issuer.issued)
	}
}

// A promotion compares against what the destination is running, not its newest capture. Captures pile
// up as drafts; until one is applied the gateway has not moved, and comparing against the newest
// would describe a destination state that nothing is running.
func TestPromoteComparesAgainstWhatTheDestinationIsRunning(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	stage, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "stage", Target: model.Target{DataPlaneID: "stage-dp"},
	})

	// Stage is running v1. A later capture adds app-b, but nothing has adopted it.
	// One organization stream: v1 has app-a alone, v2 adds app-b.
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("app-a"),
	})
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginCaptured, CreatedAt: svc.now().UTC(),
		Resources: bundleOf("app-a", testAppB),
	})
	// Stage is still running v1; dev has moved on to v2.
	if _, err := svc.Apply(context.Background(), stage.ID, "1", false); err != nil {
		t.Fatalf("apply v1 to stage: %v", err)
	}
	if _, err := svc.Apply(context.Background(), dev.ID, "2", false); err != nil {
		t.Fatalf("apply v2 to dev: %v", err)
	}

	result, err := svc.Promote(context.Background(), PromoteInput{FromGatewayID: dev.ID, ToGatewayID: stage.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Against what stage runs, app-b is an addition. Against the organization's newest version it
	// would read as unchanged, and the operator would be told a promotion changes nothing when it
	// adds a resource.
	if result.Preview.Summary.Added != 1 {
		t.Fatalf("expected app-b to read as an addition, got %+v", result.Preview.Summary)
	}
}

// A promotion moves what the source is running. A capture taken on top of that is a draft, and
// promoting it would push work the source itself never adopted.

// A promotion moves what the source is running. A capture taken on top of that is a draft, and
// promoting it would push work the source itself never adopted.
func TestPromoteSendsWhatTheSourceIsRunning(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	dev, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	stage, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "stage", Target: model.Target{DataPlaneID: "stage-dp"},
	})

	// Dev is running v1; v2 is a capture that added app-b and was never applied.
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginUploaded, CreatedAt: svc.now().UTC(), Resources: app("app-a"),
	})
	if _, err := svc.Apply(context.Background(), dev.ID, "1", false); err != nil {
		t.Fatalf("apply v1 to dev: %v", err)
	}
	_, _ = svc.store.AddVersion(context.Background(), model.Version{
		Origin: model.OriginCaptured, CreatedAt: svc.now().UTC(),
		Resources: bundleOf("app-a", testAppB),
	})

	result, err := svc.Promote(context.Background(), PromoteInput{FromGatewayID: dev.ID, ToGatewayID: stage.ID})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	promoted, err := svc.store.GetVersion(context.Background(), result.Seq)
	if err != nil {
		t.Fatalf("get promoted version: %v", err)
	}
	if strings.Contains(promoted.Resources, testAppB) {
		t.Fatalf("app-b was captured but never applied, so it must not promote:\n%s", promoted.Resources)
	}
	if !strings.Contains(promoted.Resources, "app-a") {
		t.Fatalf("expected app-a to be promoted:\n%s", promoted.Resources)
	}
}

// The gateway manager records what it knows about a gateway after promoting. This server never
// reads those attributes, so they are stored and returned unchanged.
func TestUpdateGatewayRecordsWhatTheGatewayManagerKnows(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})

	name := "development"
	attrs := map[string]string{"tier": "nonprod", "hierarchyId": "env-7"}
	updated, err := svc.UpdateGateway(context.Background(), env.ID, UpdateGatewayInput{
		Name: &name, Attributes: &attrs,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Name != "development" {
		t.Fatalf("expected the gateway renamed, got %q", updated.Name)
	}
	if updated.Attributes["tier"] != "nonprod" || updated.Attributes["hierarchyId"] != "env-7" {
		t.Fatalf("expected the attributes stored unchanged, got %v", updated.Attributes)
	}

	stored, _ := svc.GetGateway(context.Background(), env.ID)
	if stored.Attributes["tier"] != "nonprod" {
		t.Fatalf("expected the attributes persisted, got %v", stored.Attributes)
	}
}

// A field left out is left alone, so recording attributes does not silently rename the gateway.
func TestUpdateGatewayLeavesOutFieldsAlone(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})

	attrs := map[string]string{"tier": "nonprod"}
	updated, err := svc.UpdateGateway(context.Background(), env.ID, UpdateGatewayInput{
		Attributes: &attrs,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Name != "dev" {
		t.Fatalf("the name should have been left alone, got %q", updated.Name)
	}
	if updated.Target.DataPlaneID != "dev-dp" {
		t.Fatalf("the target should have been left alone, got %+v", updated.Target)
	}
}

// Replacing rather than merging keeps the caller that owns the attributes authoritative: a key it has
// dropped goes away instead of lingering because nothing said to remove it.
func TestUpdateGatewayReplacesTheAttributes(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})
	first := map[string]string{"tier": "nonprod", "stale": "yes"}
	_, _ = svc.UpdateGateway(context.Background(), env.ID, UpdateGatewayInput{Attributes: &first})

	second := map[string]string{"tier": "prod"}
	updated, err := svc.UpdateGateway(context.Background(), env.ID, UpdateGatewayInput{
		Attributes: &second,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, present := updated.Attributes["stale"]; present {
		t.Fatalf("a dropped key must not linger, got %v", updated.Attributes)
	}
	if updated.Attributes["tier"] != "prod" {
		t.Fatalf("expected the new value, got %v", updated.Attributes)
	}
}

// A blank name is refused rather than stored, which would leave a gateway nothing can identify.
func TestUpdateGatewayRefusesABlankName(t *testing.T) {
	svc := newTestService(t, &fakeClient{})
	env, _ := svc.CreateGateway(context.Background(), CreateGatewayInput{
		Name: "dev", Target: model.Target{DataPlaneID: "dev-dp"},
	})

	blank := "   "
	if _, err := svc.UpdateGateway(context.Background(), env.ID,
		UpdateGatewayInput{Name: &blank}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected a validation error, got %v", err)
	}
}
