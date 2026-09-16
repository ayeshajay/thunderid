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
	"net/http"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const tenantHandlerLoggerComponentName = "TenantHandler"

// tenantHandler serves the tenant self-management HTTP endpoints.
type tenantHandler struct {
	tenantService TenantServiceInterface
}

func newTenantHandler(tenantService TenantServiceInterface) *tenantHandler {
	return &tenantHandler{tenantService: tenantService}
}

// HandleTenantPostRequest provisions the caller's own workspace.
func (h *tenantHandler) HandleTenantPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, tenantHandlerLoggerComponentName))

	createRequest, err := sysutils.DecodeJSONBody[CreateTenantRequest](r)
	if err != nil {
		var valErr *sysutils.ValidationError
		if errors.As(err, &valErr) {
			sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
			return
		}
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidRequestFormat.Code,
			Message:     ErrorInvalidRequestFormat.Error,
			Description: ErrorInvalidRequestFormat.ErrorDescription,
		}
		sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, errResp)
		return
	}

	createRequest.Name = sysutils.SanitizeString(createRequest.Name)

	created, svcErr := h.tenantService.CreateTenant(ctx, *createRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
	logger.Debug(ctx, "Successfully provisioned tenant", log.String("deploymentID", created.DeploymentID))
}

// HandleTenantGetRequest returns the caller's own tenant.
func (h *tenantHandler) HandleTenantGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenant, svcErr := h.tenantService.GetTenant(ctx)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, tenant)
}

// HandleTenantDeleteRequest deprovisions the caller's own workspace.
func (h *tenantHandler) HandleTenantDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, tenantHandlerLoggerComponentName))

	svcErr := h.tenantService.DeleteTenant(ctx)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "Successfully deprovisioned tenant")
}

// handleError converts a ServiceError to the appropriate HTTP response.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
		switch svcErr.Code {
		case ErrorTenantNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorTenantConflict.Code:
			statusCode = http.StatusConflict
		case ErrorNoTenantInToken.Code:
			statusCode = http.StatusForbidden
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}
	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
