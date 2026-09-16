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
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorInvalidRequestFormat is returned when the request body cannot be parsed.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorTenantNotFound is returned when the caller's workspace has not been provisioned.
	ErrorTenantNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_not_found",
			DefaultValue: "Tenant not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_not_found_description",
			DefaultValue: "This organization has no provisioned workspace",
		},
	}
	// ErrorTenantConflict is returned when the caller's workspace is already provisioned.
	ErrorTenantConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_conflict",
			DefaultValue: "Tenant already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_conflict_description",
			DefaultValue: "This organization already has a provisioned workspace",
		},
	}
	// ErrorInvalidDeploymentID is returned when the requested deployment id is invalid.
	ErrorInvalidDeploymentID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_deployment_id",
			DefaultValue: "Invalid deployment id",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.tenantservice.invalid_deployment_id_description",
			DefaultValue: "The deployment named by the access token must contain only letters, digits, " +
				"'-', '_', or '.'",
		},
	}
	// ErrorNoTenantInToken is returned when the caller's token names no deployment, so there is no
	// organization to act on. Every operation here acts on the caller's own workspace, which the token
	// is the only thing that names.
	ErrorNoTenantInToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.no_tenant_in_token",
			DefaultValue: "Not authorized",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.no_tenant_in_token_description",
			DefaultValue: "The access token names no deployment, so there is no workspace to act on",
		},
	}
	// ErrorInternalServer is returned for unexpected server-side errors.
	ErrorInternalServer = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "TNT-5001",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.internal_server_error",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.internal_server_error_description",
			DefaultValue: "An unexpected error occurred while managing the tenant",
		},
	}
)

// errTenantNotFound is the sentinel error returned by the store when a registry row is absent.
var errTenantNotFound = errors.New("tenant not found")
