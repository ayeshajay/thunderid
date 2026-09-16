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

package store

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// querySaveGateway inserts an gateway, or replaces the document of one already there.
	querySaveGateway = dbmodel.DBQuery{
		ID: "EMQ-ENV-001",
		Query: `INSERT INTO "GATEWAY" (DEPLOYMENT_ID, ID, DATA) VALUES ($1, $2, $3) ` +
			`ON CONFLICT (DEPLOYMENT_ID, ID) ` +
			`DO UPDATE SET DATA = EXCLUDED.DATA, UPDATED_AT = CURRENT_TIMESTAMP`,
	}

	// queryListDeployments retrieves every deployment that has an gateway. It is the one query
	// that is not scoped to a deployment: seeding a new tenant has to find which organization's chain
	// already manages the tenant it is copied from.
	queryListDeployments = dbmodel.DBQuery{
		ID:    "EMQ-ENV-010",
		Query: `SELECT DISTINCT DEPLOYMENT_ID FROM "GATEWAY" ORDER BY DEPLOYMENT_ID`,
	}

	// queryGetGateway retrieves a single gateway document.
	queryGetGateway = dbmodel.DBQuery{
		ID:    "EMQ-ENV-002",
		Query: `SELECT DATA FROM "GATEWAY" WHERE DEPLOYMENT_ID = $1 AND ID = $2`,
	}

	// queryListGateways retrieves every gateway document for the deployment. Ordering is
	// resolved in the server, which sorts by name, which is not a column.
	queryListGateways = dbmodel.DBQuery{
		ID:    "EMQ-ENV-003",
		Query: `SELECT DATA FROM "GATEWAY" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryDeleteGateway removes an gateway. Its versions go with it, through the foreign
	// key that cascades.
	queryDeleteGateway = dbmodel.DBQuery{
		ID:    "EMQ-ENV-004",
		Query: `DELETE FROM "GATEWAY" WHERE DEPLOYMENT_ID = $1 AND ID = $2`,
	}

	// queryInsertVersion stores one captured version at an already-assigned sequence.
	queryInsertVersion = dbmodel.DBQuery{
		ID:    "EMQ-ENV-005",
		Query: `INSERT INTO "VERSION" (DEPLOYMENT_ID, SEQ, DATA) VALUES ($1, $2, $3)`,
	}

	// queryGetVersion retrieves one version document.
	queryGetVersion = dbmodel.DBQuery{
		ID:    "EMQ-ENV-006",
		Query: `SELECT DATA FROM "VERSION" WHERE DEPLOYMENT_ID = $1 AND SEQ = $2`,
	}

	// queryListVersions retrieves the organization's versions, newest first.
	queryListVersions = dbmodel.DBQuery{
		ID:    "EMQ-ENV-007",
		Query: `SELECT SEQ, DATA FROM "VERSION" WHERE DEPLOYMENT_ID = $1 ORDER BY SEQ DESC`,
	}

	// queryVersionSeqs retrieves the organization's version sequences, oldest first. Used to assign
	// the next sequence and to decide what pruning removes.
	queryVersionSeqs = dbmodel.DBQuery{
		ID:    "EMQ-ENV-008",
		Query: `SELECT SEQ FROM "VERSION" WHERE DEPLOYMENT_ID = $1 ORDER BY SEQ ASC`,
	}

	// queryDeleteVersion removes a single pruned version.
	queryDeleteVersion = dbmodel.DBQuery{
		ID:    "EMQ-ENV-009",
		Query: `DELETE FROM "VERSION" WHERE DEPLOYMENT_ID = $1 AND SEQ = $2`,
	}

	// queryInsertApply records that a gateway was moved onto a version.
	queryInsertApply = dbmodel.DBQuery{
		ID: "EMQ-ENV-010",
		Query: `INSERT INTO "GATEWAY_APPLY" (DEPLOYMENT_ID, GATEWAY_ID, ORDINAL, SEQ) ` +
			`VALUES ($1, $2, $3, $4)`,
	}

	// queryListApplies retrieves a gateway's history, newest first.
	queryListApplies = dbmodel.DBQuery{
		ID: "EMQ-ENV-011",
		Query: `SELECT ORDINAL, SEQ, APPLIED_AT FROM "GATEWAY_APPLY" ` +
			`WHERE DEPLOYMENT_ID = $1 AND GATEWAY_ID = $2 ORDER BY ORDINAL DESC`,
	}

	// queryTrimApplies drops a gateway's history entries older than the retained window.
	queryTrimApplies = dbmodel.DBQuery{
		ID: "EMQ-ENV-013",
		Query: `DELETE FROM "GATEWAY_APPLY" ` +
			`WHERE DEPLOYMENT_ID = $1 AND GATEWAY_ID = $2 AND ORDINAL <= $3`,
	}

	// queryAppliedSeqs retrieves every version any gateway of the organization has run, so pruning
	// keeps what some gateway can still be returned to.
	queryAppliedSeqs = dbmodel.DBQuery{
		ID:    "EMQ-ENV-012",
		Query: `SELECT DISTINCT SEQ FROM "GATEWAY_APPLY" WHERE DEPLOYMENT_ID = $1`,
	}
)
