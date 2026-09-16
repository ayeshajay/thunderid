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

// Package service holds the gateway-management orchestration: capturing config into versions,
// diffing versions, applying a version to a data plane via the import API (create/update/delete), and
// promoting or reverting between versions.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/gateway/auth"
	"github.com/thunder-id/thunderid/internal/gateway/bundle"
	"github.com/thunder-id/thunderid/internal/gateway/diff"
	"github.com/thunder-id/thunderid/internal/gateway/model"
	"github.com/thunder-id/thunderid/internal/gateway/store"
	"github.com/thunder-id/thunderid/internal/gateway/thunder"
	"github.com/thunder-id/thunderid/internal/system/config"
)

// ThunderClient is the subset of the ThunderID API the service depends on. It is an interface so tests
// can substitute a fake.
type ThunderClient interface {
	Export(ctx context.Context) (thunder.ExportResult, error)
	SecretKeys(ctx context.Context) ([]string, error)
	GatewayVariables(ctx context.Context, envID string) (map[string]string, error)
	Import(ctx context.Context, req thunder.ImportRequest) (*thunder.ImportResponse, error)
}

// ClientFactory builds a ThunderClient for a base URL.
type ClientFactory func(baseURL string, creds thunder.Credentials, caFile string) ThunderClient

// Store is the persistence this service needs, which store.Store implements against the gateway
// database. It is an interface so tests can substitute one that needs no database.
//
// Every method carries the deployment implicitly: a Store is opened for one, and cannot reach
// another's gateways.
type Store interface {
	SaveGateway(ctx context.Context, env model.Gateway) error
	GetGateway(ctx context.Context, id string) (model.Gateway, error)
	ListGateways(ctx context.Context) ([]model.Gateway, error)
	DeleteGateway(ctx context.Context, id string) error
	// Versions belong to the organization, so none of these names a gateway.
	AddVersion(ctx context.Context, v model.Version) (model.Version, error)
	GetVersion(ctx context.Context, seq int) (model.Version, error)
	ListVersions(ctx context.Context) ([]model.Version, error)
	LatestSeq(ctx context.Context) (int, error)

	// What each gateway has run, which is the gateway's own history.
	RecordApply(ctx context.Context, gatewayID string, seq int) (model.Apply, error)
	ListApplies(ctx context.Context, gatewayID string) ([]model.Apply, error)

	// Work queued for a Data Plane. Enqueue and Get are this deployment's own; claiming and
	// completing are not, because a pod holds connections for whichever Data Planes dialed it
	// rather than for one organization.
	EnqueueJob(ctx context.Context, job store.Job) (store.Job, error)
	GetJob(ctx context.Context, id string) (store.Job, error)
	ClaimNextJob(ctx context.Context, dataPlaneID, claimedBy string) (store.Job, bool, error)
	CompleteJob(ctx context.Context, deploymentID, id, result, failure string) error
	ReleaseJob(ctx context.Context, deploymentID, id string) error
}

// callerCredentials presents the token of whoever is driving this request.
//
// The only server this service calls over HTTP is the control plane it runs inside, and always while
// serving a request, so forwarding the caller's own token is both sufficient and correct: the capture
// reads exactly what that caller is allowed to read. Data planes are reached over the channel they
// dial out on, and need no credential at all.
func callerCredentials(ctx context.Context) thunder.Credentials {
	return thunder.Credentials{Token: auth.CallerTokenFromContext(ctx)}
}

// Service is the gateway-management application service.
type Service struct {
	store     Store
	newClient ClientFactory
	// workspaceCA is the certificate the workspace presents, trusted in addition to the system roots
	// so a control plane behind a private CA can read its own configuration.
	workspaceCA string
	now         func() time.Time
	// dataPlanes reaches the data planes connected to this control plane.
	dataPlanes DataPlanes
	// tokens issues the credential a data plane connects with.
	tokens DataPlaneTokenIssuer
	// workspaceURL is where the control plane hosting this service answers. It is the organization
	// workspace a capture reads.
	workspaceURL string
	// org is the organization whose gateways this service manages.
	org string
	// sealer encrypts a queued payload that carries a credential. It is nil until the server installs
	// one, and queueing a credential without it is refused rather than done in the clear.
	sealer SecretSealer
}

// SetWorkspaceURL installs the address of the control plane this service runs in, which is the
// organization workspace a capture reads. It is separate from New because the address is resolved
// from the server's configuration after this service is built.
func (s *Service) SetWorkspaceURL(baseURL string) {
	s.workspaceURL = baseURL
}

// SetWorkspaceCA installs the certificate the workspace presents, so a capture trusts it without
// verification being turned off.
func (s *Service) SetWorkspaceCA(caFile string) {
	s.workspaceCA = caFile
}

// SetOrganization names the organization whose gateways this service manages.
func (s *Service) SetOrganization(org string) {
	s.org = org
}

// dataPlaneDeploymentID is the deployment a data plane serves under.
//
// The control plane's own deployment is the organization, because the organization has a single
// workspace. A data plane serves one gateway of it, so it needs an id no other gateway
// shares: the organization and the gateway together.
func (s *Service) dataPlaneDeploymentID(env model.Gateway) string {
	org := strings.TrimSpace(s.org)
	name := strings.TrimSpace(env.Name)
	if org == "" || name == "" {
		return org
	}
	return org + ":" + name
}

// workspaceClient reaches the organization workspace this service runs in.
//
// The caller's own token is forwarded, so a capture reads exactly what that caller is allowed to
// read, and the organization it lands in is the one their token names. There is nothing to configure
// per gateway: every gateway of an organization captures from the same workspace.
func (s *Service) workspaceClient(ctx context.Context) (ThunderClient, error) {
	if strings.TrimSpace(s.workspaceURL) == "" {
		return nil, ErrNoWorkspace
	}
	return s.newClient(s.workspaceURL, callerCredentials(ctx), s.workspaceCA), nil
}

// New builds a Service.
func New(st Store, factory ClientFactory) *Service {
	return &Service{store: st, newClient: factory, now: time.Now}
}

// Errors surfaced to the HTTP layer.
var (
	ErrNotFound    = store.ErrNotFound
	ErrValidation  = errors.New("invalid request")
	ErrNoWorkspace = errors.New(
		"this service does not know where its control plane answers, so there is nothing to capture")
	ErrNoVersions        = errors.New("gateway has no versions")
	ErrNothingApplied    = errors.New("gateway has nothing applied yet")
	ErrNoPreviousVersion = errors.New("gateway has no previous version to revert to")
	ErrBadRef            = errors.New("invalid version reference")
	ErrRevertNotPrevious = errors.New(
		"this deployment allows reverting only to the previous version")
)

// ---- gateways ----

// CreateGatewayInput is the input to CreateGateway.
type CreateGatewayInput struct {
	Name   string
	Target model.Target
	// ManagedByControlPlane marks this the one gateway the control plane administers directly,
	// which is where a credential created in the workspace is issued. The organization's first
	// gateway takes it whether or not it is asked for, so a credential always has somewhere to go.
	ManagedByControlPlane bool
}

// CreateGateway registers a new gateway.
func (s *Service) CreateGateway(ctx context.Context,
	in CreateGatewayInput) (CreateGatewayResult, error) {
	if strings.TrimSpace(in.Name) == "" {
		return CreateGatewayResult{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(in.Target.DataPlaneID) == "" {
		return CreateGatewayResult{}, fmt.Errorf("%w: target.dataPlaneId is required", ErrValidation)
	}
	existing, err := s.store.ListGateways(ctx)
	if err != nil {
		return CreateGatewayResult{}, err
	}

	now := s.now().UTC()
	env := model.Gateway{
		ID:     newID("env"),
		Name:   in.Name,
		Target: in.Target,
		// The organization's first gateway takes the mark whatever was asked for: a credential
		// created before a second one exists has nowhere else to go.
		ManagedByControlPlane: in.ManagedByControlPlane || len(existing) == 0,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.store.SaveGateway(ctx, env); err != nil {
		return CreateGatewayResult{}, err
	}
	if env.ManagedByControlPlane {
		if err := s.clearManagedExcept(ctx, existing, env.ID); err != nil {
			return CreateGatewayResult{}, err
		}
	}

	// The data plane needs a credential to connect with, and this is the only moment it is readable.
	// Issuing it here rather than asking for one means there is nothing for an operator to invent, and
	// nothing to configure on this side at all.
	token, err := s.issueDataPlaneToken(ctx, env)
	if err != nil {
		return CreateGatewayResult{}, err
	}
	return CreateGatewayResult{Gateway: env, DataPlaneToken: token}, nil
}

// CreateGatewayResult is a registered gateway and, once, the token its data plane connects
// with. The token is not stored in readable form and is never returned again.
type CreateGatewayResult struct {
	model.Gateway
	// DataPlaneToken is empty when this control plane issues no tokens, which is the case for a
	// deployment using a single shared one configured on both sides.
	DataPlaneToken string `json:"dataPlaneToken,omitempty"`
}

// issueDataPlaneToken mints the credential the gateway's data plane presents when it connects.
func (s *Service) issueDataPlaneToken(ctx context.Context, env model.Gateway) (string, error) {
	if s.tokens == nil {
		return "", nil
	}
	token, err := s.tokens.Issue(ctx, env.Target.DataPlaneID, s.dataPlaneDeploymentID(env))
	if err != nil {
		return "", fmt.Errorf("failed to issue a token for %s: %w", env.Target.DataPlaneID, err)
	}
	return token, nil
}

// RegenerateDataPlaneToken issues a new token for an gateway's data plane and returns it once.
//
// The previous token stops working immediately, so that data plane drops until the new one is in
// place. That is the honest behavior for a rotation: a credential that still worked afterwards would
// not have been rotated.
func (s *Service) RegenerateDataPlaneToken(ctx context.Context, envID string) (string, error) {
	env, err := s.store.GetGateway(ctx, envID)
	if err != nil {
		return "", err
	}
	if s.tokens == nil {
		return "", fmt.Errorf("this server issues no data plane tokens")
	}
	return s.issueDataPlaneToken(ctx, env)
}

// GetGateway returns an gateway.
func (s *Service) GetGateway(ctx context.Context, id string) (model.Gateway, error) {
	return s.store.GetGateway(ctx, id)
}

// ListGateways returns all gateways ordered by name.
func (s *Service) ListGateways(ctx context.Context) ([]model.Gateway, error) {
	return s.store.ListGateways(ctx)
}

// GatewaySummary is an gateway plus the version state the promotion view needs, so the chain
// can be rendered without a request per gateway.
type GatewaySummary struct {
	model.Gateway
	LatestSeq int `json:"latestSeq"`
	// HasPendingChanges reports whether the latest version differs from what is applied.
	HasPendingChanges bool `json:"hasPendingChanges"`
	// DataPlane reports whether this gateway's data plane is connected. Nothing can be applied or
	// promoted to one that is not, so it is shown alongside the chain rather than discovered by
	// starting a promotion that cannot finish.
	DataPlane model.DataPlaneStatus `json:"dataPlane"`
}

// ListGatewaySummaries returns every gateway of the organization, annotated with its version
// state. The set is flat: which gateway promotes into which is not this server's to say.
func (s *Service) ListGatewaySummaries(ctx context.Context) ([]GatewaySummary, error) {
	envs, err := s.store.ListGateways(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]GatewaySummary, 0, len(envs))
	for _, env := range envs {
		latest, err := s.store.LatestSeq(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, GatewaySummary{
			Gateway:           env,
			LatestSeq:         latest,
			HasPendingChanges: latest > 0 && latest != env.AppliedSeq,
			DataPlane:         s.DataPlaneStatus(env),
		})
	}
	return out, nil
}

// UpdateGatewayInput is the input to UpdateGateway. A nil field is left as it is, so a caller
// can change one thing without restating the rest.
type UpdateGatewayInput struct {
	// Name renames the gateway.
	Name *string
	// Attributes replaces what the gateway manager records about this gateway. Replacing rather
	// than merging keeps the caller that owns them authoritative: a key it has dropped goes away
	// instead of lingering because nothing said to remove it.
	Attributes *map[string]string
}

// UpdateGateway changes a gateway's own details.
//
// This is how the gateway manager records what it knows after a promotion. It does not touch the
// gateway's versions, its applied state or its target: those are this server's to manage, and a
// caller that could rewrite them could claim a deployment is running something it is not.
func (s *Service) UpdateGateway(ctx context.Context, id string,
	in UpdateGatewayInput) (model.Gateway, error) {
	env, err := s.store.GetGateway(ctx, id)
	if err != nil {
		return model.Gateway{}, err
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return model.Gateway{}, fmt.Errorf("%w: name cannot be blank", ErrValidation)
		}
		env.Name = name
	}
	if in.Attributes != nil {
		env.Attributes = *in.Attributes
	}
	env.UpdatedAt = s.now().UTC()

	if err := s.store.SaveGateway(ctx, env); err != nil {
		return model.Gateway{}, err
	}
	return env, nil
}

// SetManagedGateway makes an gateway the one the control plane administers directly, and
// takes the mark off whichever held it.
//
// Exactly one gateway of an organization holds it, so this is a move rather than a toggle: there
// is no way to leave an organization with none, which would strand every credential created
// afterwards.
func (s *Service) SetManagedGateway(ctx context.Context, id string) (model.Gateway, error) {
	env, err := s.store.GetGateway(ctx, id)
	if err != nil {
		return model.Gateway{}, err
	}
	envs, err := s.store.ListGateways(ctx)
	if err != nil {
		return model.Gateway{}, err
	}
	if !env.ManagedByControlPlane {
		env.ManagedByControlPlane = true
		env.UpdatedAt = s.now().UTC()
		if err := s.store.SaveGateway(ctx, env); err != nil {
			return model.Gateway{}, err
		}
	}
	if err := s.clearManagedExcept(ctx, envs, env.ID); err != nil {
		return model.Gateway{}, err
	}
	return env, nil
}

// clearManagedExcept takes the mark off every gateway but one.
func (s *Service) clearManagedExcept(ctx context.Context, envs []model.Gateway, keepID string) error {
	for _, other := range envs {
		if other.ID == keepID || !other.ManagedByControlPlane {
			continue
		}
		other.ManagedByControlPlane = false
		other.UpdatedAt = s.now().UTC()
		if err := s.store.SaveGateway(ctx, other); err != nil {
			return err
		}
	}
	return nil
}

// DeleteGateway removes an gateway and its versions.
func (s *Service) DeleteGateway(ctx context.Context, id string) error {
	envs, err := s.store.ListGateways(ctx)
	if err != nil {
		return err
	}
	if err := s.store.DeleteGateway(ctx, id); err != nil {
		return err
	}

	// Deleting it would leave the organization with none, and every credential created afterwards
	// with nowhere to go. The mark passes to whichever gateway is left lowest in the chain, which
	// is where work starts.
	remaining := make([]model.Gateway, 0, len(envs))
	for _, env := range envs {
		if env.ID != id && !env.ManagedByControlPlane {
			remaining = append(remaining, env)
		}
	}
	if !wasManaged(envs, id) || len(remaining) == 0 {
		return nil
	}
	successor, ok := managedGateway(remaining)
	if !ok {
		return nil
	}
	successor.ManagedByControlPlane = true
	successor.UpdatedAt = s.now().UTC()
	return s.store.SaveGateway(ctx, successor)
}

// wasManaged reports whether the gateway being removed was the one the control plane
// administered directly.
func wasManaged(envs []model.Gateway, id string) bool {
	for _, env := range envs {
		if env.ID == id {
			return env.ManagedByControlPlane
		}
	}
	return false
}

// ---- versions ----

// CaptureVersion captures the organization's configuration as a new version.
//
// It names no gateway. What it reads is the organization's configuration as authored on this control
// plane, which is the same whichever gateway the version is later applied to, so a gateway has no
// state of its own to capture: it only receives a version.
func (s *Service) CaptureVersion(ctx context.Context, note string) (model.Version, error) {
	client, err := s.workspaceClient(ctx)
	if err != nil {
		return model.Version{}, err
	}
	exported, err := client.Export(ctx)
	if err != nil {
		return model.Version{}, fmt.Errorf("export failed: %w", err)
	}
	// The default resource server's identifier is this deployment's audience. Captured verbatim it
	// would travel to every gateway the bundle is applied to, so each of them would name the audience
	// of the control plane it was captured from. Templated here, resolved per gateway on apply.
	exported.Resources = bundle.TemplateDeploymentURL(exported.Resources)

	secretKeys, err := client.SecretKeys(ctx)
	if err != nil {
		return model.Version{}, fmt.Errorf("listing secrets failed: %w", err)
	}
	// The control plane only lists the secrets it still holds, and it forwards a credential to the data
	// plane rather than keeping it. The bundle is the authority on which placeholders are credentials.
	secretKeys = mergeKeys(secretKeys, bundle.SecretVariables(exported.Resources))

	// Only what the export itself carries. A gateway's own variables are not folded in here: they
	// belong to the gateway, not to the organization's configuration, and an apply reads them from
	// whichever gateway it is writing to.
	vars := bundle.ParseEnv(exported.EnvFile)
	// A secret's value is never stored here; the placeholder is sent on apply instead.
	for _, key := range secretKeys {
		delete(vars, key)
	}

	return s.store.AddVersion(ctx, model.Version{
		Origin:     model.OriginCaptured,
		Note:       note,
		CreatedAt:  s.now().UTC(),
		Resources:  exported.Resources,
		Variables:  vars,
		SecretKeys: secretKeys,
	})
}

// UploadVersion stores a caller-supplied bundle as a new version of the organization.
func (s *Service) UploadVersion(ctx context.Context, resources string,
	variables map[string]string, note string) (model.Version, error) {
	if variables == nil {
		variables = map[string]string{}
	}
	return s.store.AddVersion(ctx, model.Version{
		Origin:    model.OriginUploaded,
		Note:      note,
		CreatedAt: s.now().UTC(),
		Resources: resources,
		Variables: variables,
	})
}

// GetVersion returns a full version.
func (s *Service) GetVersion(ctx context.Context, seq int) (model.Version, error) {
	return s.store.GetVersion(ctx, seq)
}

// ListVersions returns the organization's version metadata, newest first.
func (s *Service) ListVersions(ctx context.Context) ([]model.Version, error) {
	return s.store.ListVersions(ctx)
}

// GatewayHistory returns what a gateway has run, newest first. This is a gateway's own history: the
// organization versions applied to it, and what "go back to what it was running before" reads.
func (s *Service) GatewayHistory(ctx context.Context, gatewayID string) ([]model.Apply, error) {
	if _, err := s.store.GetGateway(ctx, gatewayID); err != nil {
		return nil, err
	}
	return s.store.ListApplies(ctx, gatewayID)
}

// Diff computes the difference between two version references (a sequence number, "latest" or
// "applied") within one gateway.
func (s *Service) Diff(ctx context.Context, envID, fromRef, toRef string) (diff.Diff, error) {
	env, err := s.store.GetGateway(ctx, envID)
	if err != nil {
		return diff.Diff{}, err
	}
	from, err := s.resolveResources(ctx, env, fromRef)
	if err != nil {
		return diff.Diff{}, err
	}
	to, err := s.resolveResources(ctx, env, toRef)
	if err != nil {
		return diff.Diff{}, err
	}
	// Held back resources are filtered from both sides, so a preview of an apply shows what the apply
	// would actually do rather than listing changes it is going to skip.
	return diff.Compute(withoutExcluded(from, env.Excluded), withoutExcluded(to, env.Excluded)), nil
}

// ---- apply ----

// ApplyResult reports what an apply did (or would do, for a dry run).
type ApplyResult struct {
	TargetSeq int       `json:"targetSeq"`
	Diff      diff.Diff `json:"diff"`
	DryRun    bool      `json:"dryRun"`
	// MissingVariables are placeholders that resolved to nothing. The import still reports success for
	// them, because an absent value simply renders as empty, so they are reported here to explain why an
	// applied resource can come out with a field such as its redirect URIs stripped.
	MissingVariables []string                `json:"missingVariables,omitempty"`
	Import           *thunder.ImportResponse `json:"import,omitempty"`
	// JobID identifies the queued work, and Status says whether it is finished. An apply is delivered
	// by the pod holding the data plane's connection, which may not be the one that took the request,
	// so a "pending" status means the answer is collected later by this id rather than that anything
	// went wrong. Import is set only once the status is "done".
	JobID  string `json:"jobId"`
	Status string `json:"status"`
}

// resolveVariables returns the values an apply would use: the version's captured snapshot overlaid
// with whatever the control plane currently holds.
//
// The overlay happens at apply time rather than only at capture time on purpose. A variable such as a
// redirect URL is a property of the gateway, not of the configuration version, so editing it in
// the control plane has to take effect on the next apply without forcing a re-capture. When the
// control plane cannot be reached the snapshot is used unchanged, so an apply is still possible.
func (s *Service) resolveVariables(ctx context.Context, env model.Gateway,
	version model.Version) map[string]string {
	values := map[string]string{}
	// The deployment's own URL, which the captured bundle refers to in place of the audience it was
	// captured with. It sits underneath the configured variables so an operator can still override it.
	if url := strings.TrimSpace(env.Target.BaseURL); url != "" {
		values[bundle.DeploymentURLVariable] = strings.TrimRight(url, "/")
	}
	for k, v := range version.Variables {
		values[k] = v
	}
	client, err := s.workspaceClient(ctx)
	if err != nil {
		return values
	}
	live, err := client.GatewayVariables(ctx, env.ID)
	if err != nil {
		return values
	}
	for k, v := range live {
		values[k] = v
	}
	return values
}

// VariableStatus reports how an gateway's next apply would resolve its placeholders.
type VariableStatus struct {
	GatewayID string `json:"gatewayId"`
	Seq       int    `json:"seq"`
	// Required is every placeholder the version references.
	Required []string `json:"required"`
	// Missing is the subset with no value configured and no backing secret.
	Missing []string `json:"missing"`
	// SecretBacked placeholders are supplied by the data plane, so they need no value here.
	SecretBacked []string `json:"secretBacked"`
	// MissingSecrets are the secret backed placeholders the data plane's secret service does not hold.
	// Applying with these unresolved leaves a credential that rejects every attempt, so they are
	// reported before the apply rather than diagnosed after a login fails.
	MissingSecrets []string `json:"missingSecrets"`
	// SecretsChecked reports whether the secret service could be consulted at all. When false,
	// MissingSecrets is not a judgement: nothing is known either way.
	SecretsChecked bool `json:"secretsChecked"`
}

// missingSecrets reports which of the secret backed placeholders the data plane's secret service does
// not hold. The second return value is false when the service is not configured or cannot be reached,
// so a caller can tell "nothing missing" apart from "nothing known".
func (s *Service) missingSecrets(ctx context.Context, env model.Gateway,
	secretKeys []string) ([]string, bool) {
	if len(secretKeys) == 0 {
		return nil, false
	}
	plane, err := s.dataPlaneFor(env)
	if err != nil {
		return nil, false
	}
	names, err := plane.SecretNames(ctx)
	if err != nil {
		return nil, false
	}

	held := make(map[string]bool, len(names))
	for _, name := range names {
		held[name] = true
	}
	missing := []string{}
	for _, key := range secretKeys {
		if !held[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing, true
}

// CheckVariables reports which placeholders a version would fail to resolve, so a caller can fix them
// before applying rather than discovering a silently emptied field afterwards.
func (s *Service) CheckVariables(ctx context.Context, envID, versionRef string) (VariableStatus, error) {
	env, err := s.store.GetGateway(ctx, envID)
	if err != nil {
		return VariableStatus{}, err
	}
	seq, err := s.resolveSeq(ctx, env, defaultRef(versionRef, "latest"))
	if err != nil {
		return VariableStatus{}, err
	}
	if seq == 0 {
		return VariableStatus{}, ErrNoVersions
	}
	version, err := s.store.GetVersion(ctx, seq)
	if err != nil {
		return VariableStatus{}, err
	}

	scalars, arrays := bundle.RequiredVariables(version.Resources)
	required := append(append([]string{}, scalars...), arrays...)
	sort.Strings(required)

	secretBacked := secretKeysOf(version)
	missing := bundle.MissingVariables(version.Resources, s.resolveVariables(ctx, env, version), secretBacked)
	if missing == nil {
		missing = []string{}
	}
	missingSecrets, checked := s.missingSecrets(ctx, env, secretBacked)
	if missingSecrets == nil {
		missingSecrets = []string{}
	}
	return VariableStatus{
		GatewayID: envID, Seq: seq, Required: required, Missing: missing, SecretBacked: secretBacked,
		MissingSecrets: missingSecrets, SecretsChecked: checked,
	}, nil
}

// Apply applies a version (default latest) to the gateway's data-plane target. It diffs the
// target version against what is currently applied and drives the import API with the full target
// bundle (idempotent upsert) plus deletions for resources the diff shows were removed.
func (s *Service) Apply(ctx context.Context, envID, versionRef string, dryRun bool) (ApplyResult, error) {
	env, err := s.store.GetGateway(ctx, envID)
	if err != nil {
		return ApplyResult{}, err
	}
	targetSeq, err := s.resolveSeq(ctx, env, defaultRef(versionRef, "latest"))
	if err != nil {
		return ApplyResult{}, err
	}
	if targetSeq == 0 {
		return ApplyResult{}, ErrNoVersions
	}
	target, err := s.store.GetVersion(ctx, targetSeq)
	if err != nil {
		return ApplyResult{}, err
	}

	var appliedRes []bundle.Resource
	if env.AppliedSeq > 0 {
		if applied, err := s.store.GetVersion(ctx, env.AppliedSeq); err == nil {
			appliedRes = withoutExcluded(bundle.Parse(applied.Resources), env.Excluded)
		}
	}
	// A resource held back from this gateway is not pushed to its data plane either, so what runs
	// there matches what was agreed rather than quietly reappearing on the next apply. Both sides of
	// the comparison are filtered: dropping it from only one would read as a deletion, and holding a
	// resource back means leaving it alone, not removing it from the data plane.
	targetRes := withoutExcluded(bundle.Parse(target.Resources), env.Excluded)
	d := diff.Compute(appliedRes, targetRes)

	values := s.resolveVariables(ctx, env, target)
	req := thunder.ImportRequest{
		Content:   bundle.Marshal(targetRes),
		Variables: bundle.BuildTemplateVariables(target.Resources, values, secretKeysOf(target)),
		DryRun:    dryRun,
		Deletions: deletionsFromDiff(d),
		// Everything here comes from the control plane, so the data plane records it as owned there and
		// refuses local edits to it. Its own resources, written by other means, stay editable.
		Options: &thunder.ImportOptions{MarkManaged: true},
	}
	// The import is queued rather than sent, and delivered by whichever pod holds this data plane's
	// connection, which may not be this one. When it is this one the whole thing finishes here and
	// the caller is answered in this response; otherwise the caller collects the answer by job id.
	payload, err := json.Marshal(importPayload{
		Request: req, GatewayID: env.ID, TargetSeq: targetSeq, DryRun: dryRun,
	})
	if err != nil {
		return ApplyResult{}, fmt.Errorf("failed to prepare the import: %w", err)
	}
	job, err := s.dispatch(ctx, env, store.JobTypeImport, payload, false)
	if err != nil {
		return ApplyResult{}, err
	}

	result := ApplyResult{
		JobID:            job.ID,
		Status:           job.Status,
		TargetSeq:        targetSeq,
		Diff:             d,
		DryRun:           dryRun,
		MissingVariables: bundle.MissingVariables(target.Resources, values, secretKeysOf(target)),
	}
	if job.Status == store.JobFailed {
		return result, fmt.Errorf("import failed: %s", job.Error)
	}
	if job.Status == store.JobDone && job.Result != "" {
		var resp thunder.ImportResponse
		if err := json.Unmarshal([]byte(job.Result), &resp); err != nil {
			return result, fmt.Errorf("failed to read the import result: %w", err)
		}
		result.Import = &resp
	}
	return result, nil
}

// ApplyAllResult reports one gateway's outcome from an apply across every gateway.
type ApplyAllResult struct {
	GatewayID   string       `json:"gatewayId"`
	GatewayName string       `json:"gatewayName"`
	Applied     *ApplyResult `json:"applied,omitempty"`
	// Error explains why this gateway was skipped or failed. The others are still attempted, so a
	// single unreachable data plane does not block the rest.
	Error string `json:"error,omitempty"`
}

// ApplyAll applies each gateway's latest version to its data plane.
//
// This exists for the case where a value the configuration references changes rather than the
// configuration itself, for example a redirect URL edited on the control plane. The stored versions are
// untouched, but every data plane holding the old value needs the new one, and re-applying is what
// pushes it. Gateways with no version are skipped.
func (s *Service) ApplyAll(ctx context.Context, dryRun bool) []ApplyAllResult {
	envs, err := s.store.ListGateways(ctx)
	if err != nil {
		return []ApplyAllResult{{Error: err.Error()}}
	}
	results := make([]ApplyAllResult, 0, len(envs))

	for _, env := range envs {
		outcome := ApplyAllResult{GatewayID: env.ID, GatewayName: env.Name}

		latest, err := s.store.LatestSeq(ctx)
		if err != nil {
			outcome.Error = err.Error()
			results = append(results, outcome)
			continue
		}
		if latest == 0 {
			outcome.Error = ErrNoVersions.Error()
			results = append(results, outcome)
			continue
		}

		applied, err := s.Apply(ctx, env.ID, strconv.Itoa(latest), dryRun)
		if err != nil {
			outcome.Error = err.Error()
		} else {
			outcome.Applied = &applied
		}
		results = append(results, outcome)
	}
	return results
}

// ---- promote ----

// PromoteInput is the input to Promote.
type PromoteInput struct {
	FromGatewayID string
	ToGatewayID   string
	VersionRef    string   // organization version to move onto; default what the source is running
	Selection     []string // resource keys to promote, honored only when SelectionProvided
	// SelectionProvided distinguishes "the user chose these" from "the user expressed no preference".
	// Without it an empty selection could not mean "hold everything back", because it would be
	// indistinguishable from a caller that simply did not send the field.
	SelectionProvided bool
	Apply             bool
	DryRun            bool
	Note              string
}

// PromoteResult reports a promotion.
type PromoteResult struct {
	Preview diff.Diff `json:"preview"`
	// Seq is the organization version the target was moved onto. Promoting creates no version, so
	// there is a sequence here rather than a new one.
	Seq int `json:"seq"`
	// Secrets reports what happened to the target gateway's credentials.
	Secrets targetSecretOutcome `json:"secrets"`
	Applied *ApplyResult        `json:"applied,omitempty"`
}

// PromotePreview returns the diff between what the target gateway is running and the version being
// promoted onto it, without writing anything. This is what a caller reviews (and selects from)
// before promoting.
func (s *Service) PromotePreview(ctx context.Context,
	fromGatewayID, toGatewayID, versionRef string) (diff.Diff, error) {
	sourceRes, targetRes, _, err := s.promoteInputs(ctx, fromGatewayID, toGatewayID, versionRef)
	if err != nil {
		return diff.Diff{}, err
	}
	return diff.Compute(targetRes, sourceRes), nil
}

// Promote moves the target gateway onto the version the source gateway is running.
//
// It creates no version. A version belongs to the organization and already exists; promoting decides
// which gateway runs it, which is why the result names a sequence rather than a new version. What
// differs between two gateways running the same version is what each holds back, so a selection is
// remembered against the target as its exclusions rather than baked into a version of its own.
//
// Any gateway may be moved onto any version here. Which moves an organization actually permits comes
// from its environment hierarchy, which is held outside this server: the caller that knows the
// hierarchy decides, and this carries out the move it asks for.
func (s *Service) Promote(ctx context.Context, in PromoteInput) (PromoteResult, error) {
	sourceRes, targetRes, sourceSeq, err := s.promoteInputs(ctx, in.FromGatewayID, in.ToGatewayID, in.VersionRef)
	if err != nil {
		return PromoteResult{}, err
	}
	preview := diff.Compute(targetRes, sourceRes)

	targetEnv, err := s.store.GetGateway(ctx, in.ToGatewayID)
	if err != nil {
		return PromoteResult{}, err
	}
	// A resource held back on an earlier run stays held back unless this run deliberately selects it.
	if in.SelectionProvided {
		if err := s.rememberSelection(ctx, targetEnv, preview, in.Selection); err != nil {
			return PromoteResult{}, err
		}
	}

	// No credential travels with a promotion. The destination's control plane holds none, and its data
	// plane's secret service is where they live and where an operator sets them; inventing one here
	// would reach that data plane through capture and replace a credential already in use.
	source, err := s.store.GetVersion(ctx, sourceSeq)
	if err != nil {
		return PromoteResult{}, err
	}
	result := PromoteResult{
		Preview: preview,
		Seq:     sourceSeq,
		Secrets: targetSecretOutcome{Skipped: secretKeysOf(source)},
	}

	if in.Apply {
		applied, err := s.Apply(ctx, in.ToGatewayID, strconv.Itoa(sourceSeq), in.DryRun)
		if err != nil {
			return result, err
		}
		result.Applied = &applied
	}
	return result, nil
}

// ---- revert ----

// RevertInput is the input to Revert. ToRef accepts an organization version number or "previous",
// which is what this gateway was running before its current version.
type RevertInput struct {
	GatewayID string
	ToRef     string
	Apply     bool
	DryRun    bool
	Note      string
}

// RevertResult reports a revert.
type RevertResult struct {
	Preview diff.Diff `json:"preview"`
	// Seq is the organization version the gateway was moved back onto.
	Seq     int          `json:"seq"`
	Applied *ApplyResult `json:"applied,omitempty"`
}

// Revert moves a gateway back onto a version it ran before, optionally applying it.
//
// Nothing is created and nothing is deleted. A version belongs to the organization and going back is
// a change of which one this gateway runs, so its history gains an entry rather than the stream
// gaining a version. That is also what keeps the version it returns to from being pruned.
func (s *Service) Revert(ctx context.Context, in RevertInput) (RevertResult, error) {
	// Going back a version at a time is a decision an operator can reason about: the gateway returns
	// to the state immediately before this one. Jumping to an arbitrary version takes everything
	// applied in between with it, which is why a deployment can refuse it. Refused before anything is
	// read, because the answer does not depend on the gateway.
	ref := defaultRef(in.ToRef, "previous")
	if revertToPreviousOnly() && ref != "previous" {
		return RevertResult{}, ErrRevertNotPrevious
	}
	env, err := s.store.GetGateway(ctx, in.GatewayID)
	if err != nil {
		return RevertResult{}, err
	}
	toSeq, err := s.resolveSeq(ctx, env, ref)
	if err != nil {
		return RevertResult{}, err
	}
	if toSeq == 0 {
		return RevertResult{}, ErrNoVersions
	}
	target, err := s.store.GetVersion(ctx, toSeq)
	if err != nil {
		return RevertResult{}, err
	}

	var currentRes []bundle.Resource
	if env.AppliedSeq > 0 {
		if current, err := s.store.GetVersion(ctx, env.AppliedSeq); err == nil {
			currentRes = bundle.Parse(current.Resources)
		}
	}
	result := RevertResult{
		Preview: diff.Compute(currentRes, bundle.Parse(target.Resources)),
		Seq:     toSeq,
	}

	if in.Apply {
		applied, err := s.Apply(ctx, in.GatewayID, strconv.Itoa(toSeq), in.DryRun)
		if err != nil {
			return result, err
		}
		result.Applied = &applied
	}
	return result, nil
}

// ---- helpers ----

// promoteInputs resolves the version the source gateway is running and what the target is running,
// as the two sides of the comparison a promotion is reviewed against.
//
// Both are taken to be what each gateway is actually running rather than what has been captured, so
// the diff is between two states that exist rather than between two drafts.
func (s *Service) promoteInputs(ctx context.Context, fromGatewayID, toGatewayID, versionRef string) (
	sourceRes, targetRes []bundle.Resource, sourceSeq int, err error) {
	fromGateway, err := s.store.GetGateway(ctx, fromGatewayID)
	if err != nil {
		return nil, nil, 0, err
	}
	toGateway, err := s.store.GetGateway(ctx, toGatewayID)
	if err != nil {
		return nil, nil, 0, err
	}

	// What the source is running is what gets promoted. A source that has run nothing yet falls back
	// to the organization's newest version, so a freshly registered gateway can still promote.
	sourceSeq, err = s.resolveSeq(ctx, fromGateway, defaultRef(versionRef, "applied"))
	if err != nil {
		return nil, nil, 0, err
	}
	if sourceSeq == 0 {
		if sourceSeq, err = s.store.LatestSeq(ctx); err != nil {
			return nil, nil, 0, err
		}
	}
	if sourceSeq == 0 {
		return nil, nil, 0, ErrNoVersions
	}
	source, err := s.store.GetVersion(ctx, sourceSeq)
	if err != nil {
		return nil, nil, 0, err
	}

	if toGateway.AppliedSeq > 0 {
		if current, err := s.store.GetVersion(ctx, toGateway.AppliedSeq); err == nil {
			targetRes = bundle.Parse(current.Resources)
		}
	}
	return bundle.Parse(source.Resources), targetRes, sourceSeq, nil
}

// resolveResources resolves a version reference to its parsed resources.
func (s *Service) resolveResources(ctx context.Context, env model.Gateway, ref string) ([]bundle.Resource, error) {
	seq, err := s.resolveSeq(ctx, env, ref)
	if err != nil {
		return nil, err
	}
	if seq == 0 {
		return nil, nil
	}
	v, err := s.store.GetVersion(ctx, seq)
	if err != nil {
		return nil, err
	}
	return bundle.Parse(v.Resources), nil
}

// resolveSeq resolves a reference ("latest", "previous", "applied", or a number) to a version
// sequence. It returns 0 when there is nothing to resolve to (no versions / nothing applied).
func (s *Service) resolveSeq(ctx context.Context, env model.Gateway, ref string) (int, error) {
	switch ref {
	case "", "latest":
		return s.store.LatestSeq(ctx)
	case "previous":
		// What this gateway was running before its current version, read from its own history rather
		// than from the organization's stream: going back means returning to a state this gateway was
		// actually in, which is not the same as the version captured before the current one.
		history, err := s.store.ListApplies(ctx, env.ID)
		if err != nil {
			return 0, err
		}
		for _, entry := range history {
			if entry.Seq != env.AppliedSeq {
				return entry.Seq, nil
			}
		}
		return 0, ErrNoPreviousVersion
	case "applied":
		return env.AppliedSeq, nil
	default:
		seq, err := strconv.Atoi(ref)
		if err != nil || seq <= 0 {
			return 0, ErrBadRef
		}
		return seq, nil
	}
}

func defaultRef(ref, fallback string) string {
	if strings.TrimSpace(ref) == "" {
		return fallback
	}
	return ref
}

// deletionsFromDiff turns deleted resources into import deletion requests. Resources without an id
// (translations and server configuration, which are keyed by language and section) are still sent so
// the import API reports an explicit outcome rather than the prune silently skipping them.
func deletionsFromDiff(d diff.Diff) []thunder.ResourceDeletion {
	out := make([]thunder.ResourceDeletion, 0, len(d.Changes))
	for _, c := range d.Changes {
		if c.Change != diff.Deleted {
			continue
		}
		out = append(out, thunder.ResourceDeletion{ResourceType: c.Type, ID: c.ID, Category: c.Category})
	}
	return out
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "-0000000000000000"
	}
	return prefix + "-" + hex.EncodeToString(buf)
}

// mergeKeys unions two key lists into a sorted list with no duplicates.
func mergeKeys(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, key := range list {
			if key != "" && !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	sort.Strings(out)
	return out
}

// secretKeysOf returns the secret backed placeholders of a version.
//
// The stored list is only a record of what the control plane knew at capture time, so it is merged
// with what the bundle itself says. That keeps a version captured before a credential was classified
// as a secret from reporting it forever as a variable with no value.
func secretKeysOf(version model.Version) []string {
	return mergeKeys(version.SecretKeys, bundle.SecretVariables(version.Resources))
}

// revertToPreviousOnly reports whether this deployment limits a gateway to stepping back one version
// at a time.
func revertToPreviousOnly() bool {
	if !config.IsServerRuntimeInitialized() {
		return false
	}
	return config.GetServerRuntime().Config.Versions.RevertToPreviousOnly
}
