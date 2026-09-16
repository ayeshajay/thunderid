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
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateGatewayVariable inserts a new gateway variable.
	queryCreateGatewayVariable = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-001",
		Query: `INSERT INTO "GATEWAY_VARIABLE" (ID, GATEWAY_ID, KEY, VALUE, DESCRIPTION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	// queryGetGatewayVariableCount retrieves the total count of an gateway's variables.
	queryGetGatewayVariableCount = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-002",
		Query: `SELECT COUNT(*) AS total FROM "GATEWAY_VARIABLE" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetGatewayVariableList retrieves a paginated list of gateway variables.
	queryGetGatewayVariableList = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-003",
		Query: `SELECT ID, KEY, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "GATEWAY_VARIABLE" ` +
			`WHERE GATEWAY_ID = $3 AND DEPLOYMENT_ID = $4 ORDER BY KEY LIMIT $1 OFFSET $2`,
	}

	// queryGetGatewayVariableByID retrieves a single gateway variable by id.
	queryGetGatewayVariableByID = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-004",
		Query: `SELECT ID, KEY, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT ` +
			`FROM "GATEWAY_VARIABLE" WHERE ID = $1 AND GATEWAY_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryGetGatewayVariableByKey retrieves a single gateway variable by key.
	queryGetGatewayVariableByKey = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-005",
		Query: `SELECT ID, KEY, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT ` +
			`FROM "GATEWAY_VARIABLE" WHERE KEY = $1 AND GATEWAY_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryUpdateGatewayVariableByID updates an gateway variable's description and value by id.
	queryUpdateGatewayVariableByID = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-006",
		Query: `UPDATE "GATEWAY_VARIABLE" SET DESCRIPTION = $1, VALUE = $2, UPDATED_AT = NOW() ` +
			`WHERE ID = $3 AND GATEWAY_ID = $4 AND DEPLOYMENT_ID = $5`,
		SQLiteQuery: `UPDATE "GATEWAY_VARIABLE" SET DESCRIPTION = $1, VALUE = $2, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $3 AND GATEWAY_ID = $4 AND DEPLOYMENT_ID = $5`,
	}

	// queryDeleteGatewayVariableByID deletes an gateway variable by id.
	queryDeleteGatewayVariableByID = dbmodel.DBQuery{
		ID:    "EVQ-ENVVAR-007",
		Query: `DELETE FROM "GATEWAY_VARIABLE" WHERE ID = $1 AND GATEWAY_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryGetGatewayVariableValues retrieves every key and value of one gateway. Used by the
	// resolve path that substitutes declarative placeholders at export/apply time.
	queryGetGatewayVariableValues = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-008",
		Query: `SELECT KEY, VALUE FROM "GATEWAY_VARIABLE" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 ` +
			`ORDER BY KEY`,
	}
)
