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

package secretstore

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryListSecrets reads every secret of a deployment.
	queryListSecrets = dbmodel.DBQuery{
		ID: "SSQ-SECRET-001",
		Query: `SELECT NAME, KIND, VALUE, ALGORITHM, PARAMETERS, DESCRIPTION FROM "SECRET" ` +
			`WHERE DEPLOYMENT_ID = $1 ORDER BY NAME`,
	}

	// queryUpsertSecret stores a secret, replacing any entry the deployment already holds under that
	// name. A name is the whole identity of a credential, so storing one twice is a replacement rather
	// than a second row.
	queryUpsertSecret = dbmodel.DBQuery{
		ID: "SSQ-SECRET-002",
		Query: `INSERT INTO "SECRET" (ID, NAME, KIND, VALUE, ALGORITHM, PARAMETERS, DESCRIPTION, ` +
			`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) DO UPDATE SET KIND = EXCLUDED.KIND, ` +
			`VALUE = EXCLUDED.VALUE, ALGORITHM = EXCLUDED.ALGORITHM, ` +
			`PARAMETERS = EXCLUDED.PARAMETERS, DESCRIPTION = EXCLUDED.DESCRIPTION, UPDATED_AT = NOW()`,
		SQLiteQuery: `INSERT INTO "SECRET" (ID, NAME, KIND, VALUE, ALGORITHM, PARAMETERS, DESCRIPTION, ` +
			`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) DO UPDATE SET KIND = EXCLUDED.KIND, ` +
			`VALUE = EXCLUDED.VALUE, ALGORITHM = EXCLUDED.ALGORITHM, ` +
			`PARAMETERS = EXCLUDED.PARAMETERS, DESCRIPTION = EXCLUDED.DESCRIPTION, ` +
			`UPDATED_AT = datetime('now')`,
	}

	// queryDeleteSecret removes a secret by name.
	queryDeleteSecret = dbmodel.DBQuery{
		ID:    "SSQ-SECRET-003",
		Query: `DELETE FROM "SECRET" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
)
