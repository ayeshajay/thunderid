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
	"context"
	"fmt"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// fakeDB is an in-memory stand-in for the configuration database, holding the SECRET rows the backend
// writes so a test can inspect exactly what was stored.
type fakeDB struct {
	rows map[string]map[string]interface{}
}

func newFakeDB() *fakeDB {
	return &fakeDB{rows: map[string]map[string]interface{}{}}
}

// storedValue returns the VALUE column as written, which is what a database dump would hold.
func (d *fakeDB) storedValue(name string) string {
	row, ok := d.rows[name]
	if !ok {
		return ""
	}
	value, _ := row["value"].(string)
	return value
}

// GetConfigDBClient hands back the fake itself, which implements the client.
func (d *fakeDB) GetConfigDBClient() (provider.DBClientInterface, error) { return d, nil }

func (d *fakeDB) GetRuntimeTransientDBClient() (provider.DBClientInterface, error) {
	return nil, fmt.Errorf("not used")
}

func (d *fakeDB) GetEntityDBClient() (provider.DBClientInterface, error) {
	return nil, fmt.Errorf("not used")
}

func (d *fakeDB) GetRuntimePersistentDBClient() (provider.DBClientInterface, error) {
	return nil, fmt.Errorf("not used")
}

func (d *fakeDB) GetConfigDBTransactioner() (providers.Transactioner, error) {
	return nil, fmt.Errorf("not used")
}

func (d *fakeDB) GetEntityDBTransactioner() (providers.Transactioner, error) {
	return nil, fmt.Errorf("not used")
}

func (d *fakeDB) GetRuntimeTransientDBTransactioner() (providers.Transactioner, error) {
	return nil, fmt.Errorf("not used")
}

func (d *fakeDB) GetRuntimePersistentDBTransactioner() (providers.Transactioner, error) {
	return nil, fmt.Errorf("not used")
}

// QueryContext serves the list query and records an upsert, which is the only write the backend makes
// through this path.
func (d *fakeDB) QueryContext(_ context.Context, query dbmodel.DBQuery,
	args ...interface{}) ([]map[string]interface{}, error) {
	switch query.ID {
	case queryListSecrets.ID:
		out := make([]map[string]interface{}, 0, len(d.rows))
		for _, row := range d.rows {
			out = append(out, row)
		}
		return out, nil
	case queryUpsertSecret.ID:
		// (id, name, kind, value, algorithm, parameters, description, deploymentID)
		name, _ := args[1].(string)
		d.rows[name] = map[string]interface{}{
			"name": name, "kind": args[2], "value": args[3],
			"algorithm": args[4], "parameters": args[5], "description": args[6],
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected query %s", query.ID)
	}
}

func (d *fakeDB) Query(query dbmodel.DBQuery, args ...interface{}) ([]map[string]interface{}, error) {
	return d.QueryContext(context.Background(), query, args...)
}

// ExecuteContext serves the delete.
func (d *fakeDB) ExecuteContext(_ context.Context, query dbmodel.DBQuery,
	args ...interface{}) (int64, error) {
	if query.ID != queryDeleteSecret.ID {
		return 0, fmt.Errorf("unexpected query %s", query.ID)
	}
	name, _ := args[0].(string)
	if _, ok := d.rows[name]; !ok {
		return 0, nil
	}
	delete(d.rows, name)
	return 1, nil
}

func (d *fakeDB) Execute(query dbmodel.DBQuery, args ...interface{}) (int64, error) {
	return d.ExecuteContext(context.Background(), query, args...)
}

func (d *fakeDB) BeginTx() (dbmodel.TxInterface, error) { return nil, fmt.Errorf("not used") }

func (d *fakeDB) GetTransactioner() (providers.Transactioner, error) {
	return nil, fmt.Errorf("not used")
}
