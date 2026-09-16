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
	"os"
	"regexp"
	"sync"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/bootstrap"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const tenantLoggerComponentName = "TenantService"

// deploymentIDPattern restricts a tenant's deployment id to a safe, portable character set.
var deploymentIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// TenantServiceInterface defines the operations a tenant performs on its own workspace.
type TenantServiceInterface interface {
	CreateTenant(ctx context.Context, request CreateTenantRequest) (*Tenant, *tidcommon.ServiceError)
	GetTenant(ctx context.Context) (*Tenant, *tidcommon.ServiceError)
	DeleteTenant(ctx context.Context) *tidcommon.ServiceError
}

// tenantService is the default implementation of TenantServiceInterface.
type tenantService struct {
	store       tenantStoreInterface
	importSvc   importer.ImportServiceInterface
	defaultsDir string
	publicURL   string
	// bootstrapRun provisions a tenant's baseline. It defaults to bootstrap.Run and is a field so
	// tests can substitute it.
	bootstrapRun func(ctx context.Context, importSvc importer.ImportServiceInterface, opts bootstrap.Options) error
	// provisionMu serializes provisioning because it sets process-global env vars for the bootstrap
	// bundle's template substitution and runs the bootstrap import.
	provisionMu sync.Mutex
}

func newTenantService(store tenantStoreInterface, importSvc importer.ImportServiceInterface,
	defaultsDir, publicURL string) TenantServiceInterface {
	return &tenantService{
		store:        store,
		importSvc:    importSvc,
		defaultsDir:  defaultsDir,
		publicURL:    publicURL,
		bootstrapRun: bootstrap.Run,
	}
}

// callerOrganization is the organization the request acts on: the one its token names, and the only
// one it can reach. It comes from the token rather than the request, so there is nothing to authorize
// beyond the claim already being there.
//
// A server that takes no deployment from the token has a single tenant provisioned at install time,
// and nothing here applies to it.
func (s *tenantService) callerOrganization(ctx context.Context) (string, *tidcommon.ServiceError) {
	id, ok := deployment.IDFromContext(ctx)
	if !ok {
		return "", &ErrorNoTenantInToken
	}
	// The deployment claim may name a gateway ("<org>:<gateway>"). The workspace is the
	// organization's, so everything the organization owns stays in one partition.
	org := deployment.OrganizationOf(id)
	if !deploymentIDPattern.MatchString(org) {
		return "", &ErrorInvalidDeploymentID
	}
	return org, nil
}

// CreateTenant provisions the caller's own workspace from the bootstrap baseline.
//
// The workspace holds the organization's configuration. Its gateways are resources inside it rather
// than workspaces of their own, and are created through the gateway API once this exists.
func (s *tenantService) CreateTenant(ctx context.Context,
	request CreateTenantRequest) (*Tenant, *tidcommon.ServiceError) {
	deploymentID, svcErr := s.callerOrganization(ctx)
	if svcErr != nil {
		return nil, svcErr
	}

	provisioned, err := s.store.IsProvisioned(ctx, deploymentID)
	if err != nil {
		return nil, s.internalError(ctx, "failed to check tenant provisioning state", err)
	}
	if provisioned {
		return nil, &ErrorTenantConflict
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return nil, s.internalError(ctx, "failed to generate tenant id", err)
	}

	if svcErr := s.provision(ctx, deploymentID); svcErr != nil {
		return nil, svcErr
	}

	tenant := Tenant{ID: id, DeploymentID: deploymentID, Name: request.Name}
	if err := s.store.CreateTenant(ctx, tenant); err != nil {
		return nil, s.internalError(ctx, "failed to record tenant", err)
	}
	return &tenant, nil
}

// provision runs the bootstrap import scoped to the caller's deployment id. It is serialized because
// it sets process-global env vars that the bootstrap bundle's placeholders resolve from.
func (s *tenantService) provision(ctx context.Context, deploymentID string) *tidcommon.ServiceError {
	s.provisionMu.Lock()
	defer s.provisionMu.Unlock()

	// No administrator credentials: a tenant is provisioned without a local administrator, because
	// whoever administers it signs in against the trusted issuer instead.
	for key, value := range map[string]string{
		"PUBLIC_URL":              s.publicURL,
		"CONSOLE_REDIRECT_URIS_0": s.publicURL + "/console",
	} {
		if err := os.Setenv(key, value); err != nil {
			return s.internalError(ctx, "failed to set bootstrap environment", err)
		}
	}

	if err := s.bootstrapRun(ctx, s.importSvc, bootstrap.Options{
		DefaultsDir:  s.defaultsDir,
		DeploymentID: deploymentID,
	}); err != nil {
		return s.internalError(ctx, "failed to provision tenant baseline", err)
	}
	return nil
}

// GetTenant returns the caller's own registry row.
func (s *tenantService) GetTenant(ctx context.Context) (*Tenant, *tidcommon.ServiceError) {
	deploymentID, svcErr := s.callerOrganization(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	tenant, err := s.store.GetTenant(ctx, deploymentID)
	if err != nil {
		if errors.Is(err, errTenantNotFound) {
			return nil, &ErrorTenantNotFound
		}
		return nil, s.internalError(ctx, "failed to read tenant", err)
	}
	return &tenant, nil
}

// DeleteTenant deprovisions the caller's own workspace: purges all of its data and removes its
// registry row.
func (s *tenantService) DeleteTenant(ctx context.Context) *tidcommon.ServiceError {
	deploymentID, svcErr := s.callerOrganization(ctx)
	if svcErr != nil {
		return svcErr
	}

	provisioned, err := s.store.IsProvisioned(ctx, deploymentID)
	if err != nil {
		return s.internalError(ctx, "failed to check tenant provisioning state", err)
	}
	if !provisioned {
		if _, getErr := s.store.GetTenant(ctx, deploymentID); errors.Is(getErr, errTenantNotFound) {
			return &ErrorTenantNotFound
		}
	}

	if err := s.store.PurgeTenantData(ctx, deploymentID); err != nil {
		return s.internalError(ctx, "failed to purge tenant data", err)
	}
	if err := s.store.DeleteTenantRecord(ctx, deploymentID); err != nil {
		return s.internalError(ctx, "failed to delete tenant record", err)
	}
	return nil
}

// internalError logs the underlying error and returns the generic server-side ServiceError.
func (s *tenantService) internalError(ctx context.Context, msg string, err error) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, tenantLoggerComponentName))
	logger.Error(ctx, msg, log.Error(err))
	return &ErrorInternalServer
}
