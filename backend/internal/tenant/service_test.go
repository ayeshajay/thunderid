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

package tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/bootstrap"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/importer"
)

// fakeStore is an in-memory tenantStoreInterface.
type fakeStore struct {
	provisioned map[string]bool
	registry    map[string]Tenant
	purged      []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{provisioned: map[string]bool{}, registry: map[string]Tenant{}}
}

func (s *fakeStore) CreateTenant(_ context.Context, t Tenant) error {
	s.registry[t.DeploymentID] = t
	s.provisioned[t.DeploymentID] = true
	return nil
}

func (s *fakeStore) GetTenant(_ context.Context, deploymentID string) (Tenant, error) {
	t, ok := s.registry[deploymentID]
	if !ok {
		return Tenant{}, errTenantNotFound
	}
	return t, nil
}

func (s *fakeStore) DeleteTenantRecord(_ context.Context, deploymentID string) error {
	delete(s.registry, deploymentID)
	return nil
}

func (s *fakeStore) IsProvisioned(_ context.Context, deploymentID string) (bool, error) {
	return s.provisioned[deploymentID], nil
}

func (s *fakeStore) PurgeTenantData(_ context.Context, deploymentID string) error {
	s.purged = append(s.purged, deploymentID)
	delete(s.provisioned, deploymentID)
	return nil
}

func newTestService(store tenantStoreInterface, run func(context.Context, importer.ImportServiceInterface,
	bootstrap.Options) error) *tenantService {
	return &tenantService{
		store:        store,
		publicURL:    "https://cp.example",
		bootstrapRun: run,
	}
}

// callerCtx is a request from a token naming the given deployment.
func callerCtx(id string) context.Context {
	return deployment.WithID(context.Background(), id)
}

func noopRun(_ context.Context, _ importer.ImportServiceInterface, _ bootstrap.Options) error {
	return nil
}

func TestCreateTenantProvisionsTheCallersOwnWorkspace(t *testing.T) {
	store := newFakeStore()
	var provisioned []string
	svc := newTestService(store, func(_ context.Context, _ importer.ImportServiceInterface,
		opts bootstrap.Options) error {
		provisioned = append(provisioned, opts.DeploymentID)
		return nil
	})

	created, svcErr := svc.CreateTenant(callerCtx("acme"), CreateTenantRequest{Name: "Acme"})

	require.Nil(t, svcErr)
	assert.Equal(t, "acme", created.DeploymentID)
	assert.Equal(t, "Acme", created.Name)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, []string{"acme"}, provisioned, "the baseline is provisioned into the caller's deployment")
	assert.Equal(t, "acme", store.registry["acme"].DeploymentID)
}

// The organization owns the workspace, so a token naming one of its gateways still provisions
// (and later reads and deletes) the organization's single workspace rather than a per-gateway one.
func TestCreateTenantUsesTheOrganizationWhenTheTokenNamesAGateway(t *testing.T) {
	store := newFakeStore()
	var provisioned []string
	svc := newTestService(store, func(_ context.Context, _ importer.ImportServiceInterface,
		opts bootstrap.Options) error {
		provisioned = append(provisioned, opts.DeploymentID)
		return nil
	})

	created, svcErr := svc.CreateTenant(callerCtx("acme:dev"), CreateTenantRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, "acme", created.DeploymentID)
	assert.Equal(t, []string{"acme"}, provisioned)
}

// A caller cannot name the organization it provisions, so the only way to reach another one is a
// token for it. This is what replaces the former system-tenant gate.
func TestCreateTenantCannotReachAnotherOrganization(t *testing.T) {
	store := newFakeStore()
	store.provisioned["other"] = true
	svc := newTestService(store, noopRun)

	created, svcErr := svc.CreateTenant(callerCtx("acme"), CreateTenantRequest{})

	require.Nil(t, svcErr)
	assert.Equal(t, "acme", created.DeploymentID)
	assert.NotContains(t, store.registry, "other", "the other organization is untouched")
}

func TestCreateTenantRefusesATokenNamingNoDeployment(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	_, svcErr := svc.CreateTenant(context.Background(), CreateTenantRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorNoTenantInToken.Code, svcErr.Code)
}

func TestCreateTenantRefusesAnUnsafeDeploymentID(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	_, svcErr := svc.CreateTenant(callerCtx("bad/id"), CreateTenantRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidDeploymentID.Code, svcErr.Code)
}

func TestCreateTenantIsRefusedWhenAlreadyProvisioned(t *testing.T) {
	store := newFakeStore()
	store.provisioned["acme"] = true
	svc := newTestService(store, noopRun)

	_, svcErr := svc.CreateTenant(callerCtx("acme"), CreateTenantRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorTenantConflict.Code, svcErr.Code)
}

// Nothing is recorded when the baseline could not be imported, so a failed attempt can be retried
// rather than leaving a registry row for a workspace that has no resources.
func TestCreateTenantRecordsNothingWhenProvisioningFails(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store, func(_ context.Context, _ importer.ImportServiceInterface,
		_ bootstrap.Options) error {
		return errors.New("bundle is unreadable")
	})

	_, svcErr := svc.CreateTenant(callerCtx("acme"), CreateTenantRequest{})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorInternalServer.Code, svcErr.Code)
	assert.NotContains(t, store.registry, "acme")
}

func TestGetTenantReturnsTheCallersOwnTenant(t *testing.T) {
	store := newFakeStore()
	store.registry["acme"] = Tenant{ID: "t-1", DeploymentID: "acme", Name: "Acme"}
	svc := newTestService(store, noopRun)

	tenant, svcErr := svc.GetTenant(callerCtx("acme"))

	require.Nil(t, svcErr)
	assert.Equal(t, "t-1", tenant.ID)
	assert.Equal(t, "acme", tenant.DeploymentID)
}

func TestGetTenantIsNotFoundBeforeProvisioning(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	_, svcErr := svc.GetTenant(callerCtx("acme"))

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorTenantNotFound.Code, svcErr.Code)
}

func TestDeleteTenantPurgesTheCallersOwnData(t *testing.T) {
	store := newFakeStore()
	store.provisioned["acme"] = true
	store.registry["acme"] = Tenant{ID: "t-1", DeploymentID: "acme"}
	svc := newTestService(store, noopRun)

	svcErr := svc.DeleteTenant(callerCtx("acme"))

	require.Nil(t, svcErr)
	assert.Equal(t, []string{"acme"}, store.purged)
	assert.NotContains(t, store.registry, "acme")
}

func TestDeleteTenantIsNotFoundWhenNothingWasProvisioned(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	svcErr := svc.DeleteTenant(callerCtx("acme"))

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorTenantNotFound.Code, svcErr.Code)
}

// A deprovision that left a registry row behind is completed rather than reported as missing, so the
// purge is idempotent and a partial previous run can be finished.
func TestDeleteTenantFinishesAPartialPreviousRun(t *testing.T) {
	store := newFakeStore()
	store.registry["acme"] = Tenant{ID: "t-1", DeploymentID: "acme"}
	svc := newTestService(store, noopRun)

	svcErr := svc.DeleteTenant(callerCtx("acme"))

	require.Nil(t, svcErr)
	assert.Equal(t, []string{"acme"}, store.purged)
	assert.NotContains(t, store.registry, "acme")
}

func TestDeleteTenantRefusesATokenNamingNoDeployment(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	svcErr := svc.DeleteTenant(context.Background())

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorNoTenantInToken.Code, svcErr.Code)
}
