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

package gatewayvariable

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const gatewayVariableHandlerLoggerComponentName = "GatewayVariableHandler"

// gatewayVariableHandler serves the gateway variable management HTTP endpoints.
type gatewayVariableHandler struct {
	gatewayVariableService GatewayVariableServiceInterface
}

// newGatewayVariableHandler creates a new gatewayVariableHandler.
func newGatewayVariableHandler(service GatewayVariableServiceInterface) *gatewayVariableHandler {
	return &gatewayVariableHandler{gatewayVariableService: service}
}

// HandleGatewayVariablePostRequest handles gateway variable creation.
func (h *gatewayVariableHandler) HandleGatewayVariablePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, gatewayVariableHandlerLoggerComponentName))

	createRequest, err := sysutils.DecodeJSONBody[CreateGatewayVariableRequest](r)
	if err != nil {
		writeParseError(ctx, w, err)
		return
	}

	createRequest.Key = sysutils.SanitizeString(createRequest.Key)
	createRequest.Description = sysutils.SanitizeString(createRequest.Description)

	envID, failed := extractAndValidateGatewayID(w, r)
	if failed {
		return
	}

	created, svcErr := h.gatewayVariableService.CreateGatewayVariable(ctx, envID, *createRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, created)
	logger.Debug(ctx, "Successfully created gateway variable",
		log.String("gatewayVariableID", created.ID))
}

// HandleGatewayVariableListRequest handles listing gateway variables.
func (h *gatewayVariableHandler) HandleGatewayVariableListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	envID, failed := extractAndValidateGatewayID(w, r)
	if failed {
		return
	}

	listResponse, svcErr := h.gatewayVariableService.GetGatewayVariableList(ctx, envID, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, listResponse)
}

// HandleGatewayVariableResolveRequest returns every key mapped to its value. It backs the config
// export/apply tooling that substitutes declarative placeholders for a Data Plane.
func (h *gatewayVariableHandler) HandleGatewayVariableResolveRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	envID, failed := extractAndValidateGatewayID(w, r)
	if failed {
		return
	}

	values, svcErr := h.gatewayVariableService.ResolveGatewayVariables(ctx, envID)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, GatewayVariableResolveResponse{Variables: values})
}

// HandleGatewayVariableGetRequest handles retrieving a single gateway variable.
func (h *gatewayVariableHandler) HandleGatewayVariableGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	envID, failed := extractAndValidateGatewayID(w, r)
	if failed {
		return
	}
	id, failed := extractAndValidateID(w, r)
	if failed {
		return
	}

	result, svcErr := h.gatewayVariableService.GetGatewayVariable(ctx, envID, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, result)
}

// HandleGatewayVariablePutRequest handles updating an gateway variable.
func (h *gatewayVariableHandler) HandleGatewayVariablePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, gatewayVariableHandlerLoggerComponentName))

	envID, failed := extractAndValidateGatewayID(w, r)
	if failed {
		return
	}
	id, failed := extractAndValidateID(w, r)
	if failed {
		return
	}

	updateRequest, err := sysutils.DecodeJSONBody[UpdateGatewayVariableRequest](r)
	if err != nil {
		writeParseError(ctx, w, err)
		return
	}
	updateRequest.Description = sysutils.SanitizeString(updateRequest.Description)

	updated, svcErr := h.gatewayVariableService.UpdateGatewayVariable(ctx, envID, id, *updateRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, updated)
	logger.Debug(ctx, "Successfully updated gateway variable", log.String("gatewayVariableID", id))
}

// HandleGatewayVariableDeleteRequest handles deleting an gateway variable.
func (h *gatewayVariableHandler) HandleGatewayVariableDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, gatewayVariableHandlerLoggerComponentName))

	envID, failed := extractAndValidateGatewayID(w, r)
	if failed {
		return
	}
	id, failed := extractAndValidateID(w, r)
	if failed {
		return
	}

	svcErr := h.gatewayVariableService.DeleteGatewayVariable(ctx, envID, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "Successfully deleted gateway variable", log.String("gatewayVariableID", id))
}

// parsePaginationParams parses limit and offset from query parameters.
func parsePaginationParams(query map[string][]string) (int, int, *tidcommon.ServiceError) {
	var limit, offset int
	var err error

	if limitStr := query["limit"]; len(limitStr) > 0 && limitStr[0] != "" {
		limit, err = strconv.Atoi(sysutils.SanitizeString(limitStr[0]))
		if err != nil || limit <= 0 {
			return 0, 0, &ErrorInvalidGatewayVariableRequest
		}
	}

	if offsetStr := query["offset"]; len(offsetStr) > 0 && offsetStr[0] != "" {
		offset, err = strconv.Atoi(sysutils.SanitizeString(offsetStr[0]))
		if err != nil || offset < 0 {
			return 0, 0, &ErrorInvalidGatewayVariableRequest
		}
	}

	return limit, offset, nil
}

// extractAndValidateID extracts and validates the gateway variable id from the URL path.
func extractAndValidateID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidGatewayVariableRequest.Code,
			Message:     ErrorInvalidGatewayVariableRequest.Error,
			Description: ErrorInvalidGatewayVariableRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, errResp)
		return "", true
	}
	return id, false
}

// extractAndValidateGatewayID reads the gateway a variable belongs to from the path.
func extractAndValidateGatewayID(w http.ResponseWriter, r *http.Request) (string, bool) {
	envID := r.PathValue("gatewayId")
	if envID == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidGatewayVariableRequest.Code,
			Message:     ErrorInvalidGatewayVariableRequest.Error,
			Description: ErrorInvalidGatewayVariableRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, errResp)
		return "", true
	}
	return envID, false
}

// writeParseError writes a validation or parse-failure response for a malformed request body.
func writeParseError(ctx context.Context, w http.ResponseWriter, err error) {
	var valErr *sysutils.ValidationError
	if errors.As(err, &valErr) {
		sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
		return
	}
	errResp := apierror.ErrorResponse{
		Code:        ErrorInvalidGatewayVariableRequest.Code,
		Message:     ErrorInvalidGatewayVariableRequest.Error,
		Description: ErrorInvalidGatewayVariableRequest.ErrorDescription,
	}
	sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, errResp)
}

// handleError converts a ServiceError to the appropriate HTTP response.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
		switch svcErr.Code {
		case ErrorGatewayVariableNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorGatewayVariableKeyConflict.Code:
			statusCode = http.StatusConflict
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}
	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
