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
	"fmt"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// A Control Plane configures no runtime datasource, because it holds no runtime state. It is also
// where a tenant is deprovisioned from, so an absent datasource is nothing to purge rather than a
// reason to fail a deprovision that has already removed everything there was.
func TestPurgingAnUnconfiguredRuntimeDatasourceIsNothingToDo(t *testing.T) {
	store := &tenantStore{}

	err := store.purgeRuntime(context.Background(),
		func() (provider.DBClientInterface, error) {
			return nil, fmt.Errorf("%w: runtime_persistent", provider.ErrDataSourceNotConfigured)
		},
		[]string{"REVOKED_TOKEN"}, "runtime-persistent", "acme")

	if err != nil {
		t.Fatalf("an unconfigured datasource should be skipped, got %v", err)
	}
}

// A datasource that is configured but unreachable is a real failure: silently skipping it would
// report a tenant as deprovisioned while its runtime rows were still there.
func TestPurgingAnUnreachableRuntimeDatasourceFails(t *testing.T) {
	store := &tenantStore{}
	broken := errors.New("connection refused")

	err := store.purgeRuntime(context.Background(),
		func() (provider.DBClientInterface, error) { return nil, broken },
		[]string{"REVOKED_TOKEN"}, "runtime-persistent", "acme")

	if !errors.Is(err, broken) {
		t.Fatalf("expected the underlying failure to surface, got %v", err)
	}
}
