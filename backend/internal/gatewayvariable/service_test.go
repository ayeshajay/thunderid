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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeStore is an in-memory gatewayVariableStoreInterface.
//
// It keys rows by gateway, because that is the isolation the real store enforces: two
// gateways may each hold a variable under the same key, and neither may read the other's.
type fakeStore struct {
	byID  map[string]GatewayVariable
	envOf map[string]string
	order []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{byID: map[string]GatewayVariable{}, envOf: map[string]string{}}
}

// idsIn lists the ids belonging to one gateway, in insertion order.
func (s *fakeStore) idsIn(envID string) []string {
	out := []string{}
	for _, id := range s.order {
		if s.envOf[id] == envID {
			out = append(out, id)
		}
	}
	return out
}

func (s *fakeStore) CreateGatewayVariable(_ context.Context, envID string, ev GatewayVariable) error {
	s.byID[ev.ID] = ev
	s.envOf[ev.ID] = envID
	s.order = append(s.order, ev.ID)
	return nil
}

func (s *fakeStore) GetGatewayVariableCount(_ context.Context, envID string) (int, error) {
	return len(s.idsIn(envID)), nil
}

func (s *fakeStore) GetGatewayVariableList(_ context.Context, envID string, limit,
	offset int) ([]GatewayVariable, error) {
	out := []GatewayVariable{}
	for i, id := range s.idsIn(envID) {
		if i < offset {
			continue
		}
		if len(out) >= limit {
			break
		}
		out = append(out, s.byID[id])
	}
	return out, nil
}

func (s *fakeStore) GetGatewayVariableByID(_ context.Context, envID, id string) (GatewayVariable, error) {
	ev, ok := s.byID[id]
	if !ok || s.envOf[id] != envID {
		return GatewayVariable{}, errGatewayVariableNotFound
	}
	return ev, nil
}

func (s *fakeStore) GetGatewayVariableByKey(_ context.Context, envID, key string) (GatewayVariable, error) {
	for _, id := range s.idsIn(envID) {
		if s.byID[id].Key == key {
			return s.byID[id], nil
		}
	}
	return GatewayVariable{}, errGatewayVariableNotFound
}

func (s *fakeStore) UpdateGatewayVariableByID(_ context.Context, envID, id, description,
	value string) error {
	ev, ok := s.byID[id]
	if !ok || s.envOf[id] != envID {
		return errGatewayVariableNotFound
	}
	ev.Description = description
	ev.Value = value
	s.byID[id] = ev
	return nil
}

func (s *fakeStore) DeleteGatewayVariableByID(_ context.Context, envID, id string) error {
	if _, ok := s.byID[id]; !ok || s.envOf[id] != envID {
		return errGatewayVariableNotFound
	}
	delete(s.byID, id)
	delete(s.envOf, id)
	return nil
}

func (s *fakeStore) GetGatewayVariableValues(_ context.Context, envID string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range s.idsIn(envID) {
		ev := s.byID[id]
		out[ev.Key] = ev.Value
	}
	return out, nil
}

// failingStore fails every persistence call with an error other than the not-found sentinel, so the
// service must surface the generic internal error.
type failingStore struct{}

func (s *failingStore) CreateGatewayVariable(_ context.Context, _ string, _ GatewayVariable) error {
	return errStoreFailure
}

func (s *failingStore) GetGatewayVariableCount(_ context.Context, _ string) (int, error) {
	return 0, errStoreFailure
}

func (s *failingStore) GetGatewayVariableList(_ context.Context, _ string, _,
	_ int) ([]GatewayVariable, error) {
	return nil, errStoreFailure
}

func (s *failingStore) GetGatewayVariableByID(_ context.Context, _, _ string) (GatewayVariable, error) {
	return GatewayVariable{}, errStoreFailure
}

func (s *failingStore) GetGatewayVariableByKey(_ context.Context, _, _ string) (GatewayVariable, error) {
	return GatewayVariable{}, errStoreFailure
}

func (s *failingStore) UpdateGatewayVariableByID(_ context.Context, _, _, _, _ string) error {
	return errStoreFailure
}

func (s *failingStore) DeleteGatewayVariableByID(_ context.Context, _, _ string) error {
	return errStoreFailure
}

func (s *failingStore) GetGatewayVariableValues(_ context.Context, _ string) (map[string]string, error) {
	return nil, errStoreFailure
}

var errStoreFailure = errors.New("store failure")

// missingID is an id that is never created, used to exercise the not-found paths.
const missingID = "missing"

func newTestService() (*fakeStore, GatewayVariableServiceInterface) {
	store := newFakeStore()
	return store, newGatewayVariableService(store)
}

func TestCreateGatewayVariable(t *testing.T) {
	tests := []struct {
		name        string
		existingKey string
		request     CreateGatewayVariableRequest
		expectedErr string
	}{
		{
			name:    "Success",
			request: CreateGatewayVariableRequest{Key: "MY_APP_REDIRECT_URL", Value: "https://app/cb"},
		},
		{
			name:        "KeyStartingWithDigit",
			request:     CreateGatewayVariableRequest{Key: "1_BAD", Value: "v"},
			expectedErr: ErrorInvalidGatewayVariableRequest.Code,
		},
		{
			name:        "KeyWithSpace",
			request:     CreateGatewayVariableRequest{Key: "BAD KEY", Value: "v"},
			expectedErr: ErrorInvalidGatewayVariableRequest.Code,
		},
		{
			name:        "EmptyKey",
			request:     CreateGatewayVariableRequest{Key: "", Value: "v"},
			expectedErr: ErrorInvalidGatewayVariableRequest.Code,
		},
		{
			name:        "DuplicateKey",
			existingKey: "DUP_KEY",
			request:     CreateGatewayVariableRequest{Key: "DUP_KEY", Value: "second"},
			expectedErr: ErrorGatewayVariableKeyConflict.Code,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, svc := newTestService()
			ctx := context.Background()

			if test.existingKey != "" {
				_, svcErr := svc.CreateGatewayVariable(ctx, "env-1", CreateGatewayVariableRequest{
					Key: test.existingKey, Value: "first",
				})
				require.Nil(t, svcErr)
			}

			created, svcErr := svc.CreateGatewayVariable(ctx, "env-1", test.request)

			if test.expectedErr != "" {
				require.NotNil(t, svcErr)
				assert.Equal(t, test.expectedErr, svcErr.Code)
				return
			}

			require.Nil(t, svcErr)
			require.NotNil(t, created)
			assert.NotEmpty(t, created.ID)
			assert.Equal(t, test.request.Key, created.Key)
			// Values are non-secret, so they are stored and returned in plaintext.
			assert.Equal(t, test.request.Value, created.Value)
			assert.Equal(t, test.request.Value, store.byID[created.ID].Value)
		})
	}
}

func TestGetGatewayVariable(t *testing.T) {
	tests := []struct {
		name        string
		create      bool
		expectedErr string
	}{
		{name: "Success", create: true},
		{name: "NotFound", expectedErr: ErrorGatewayVariableNotFound.Code},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, svc := newTestService()
			ctx := context.Background()

			id := missingID
			if test.create {
				created, svcErr := svc.CreateGatewayVariable(ctx, "env-1", CreateGatewayVariableRequest{
					Key: "REDIRECT_URL", Value: "https://app/cb", Description: "callback",
				})
				require.Nil(t, svcErr)
				id = created.ID
			}

			got, svcErr := svc.GetGatewayVariable(ctx, "env-1", id)

			if test.expectedErr != "" {
				require.NotNil(t, svcErr)
				assert.Equal(t, test.expectedErr, svcErr.Code)
				return
			}

			require.Nil(t, svcErr)
			assert.Equal(t, "REDIRECT_URL", got.Key)
			assert.Equal(t, "https://app/cb", got.Value)
			assert.Equal(t, "callback", got.Description)
		})
	}
}

func TestGetGatewayVariableList(t *testing.T) {
	tests := []struct {
		name          string
		keys          []string
		limit         int
		offset        int
		expectedTotal int
		expectedCount int
	}{
		{name: "Empty", limit: 10, expectedTotal: 0, expectedCount: 0},
		{name: "AllWithinLimit", keys: []string{"A", "B"}, limit: 10, expectedTotal: 2, expectedCount: 2},
		{name: "LimitTruncates", keys: []string{"A", "B", "C"}, limit: 2, expectedTotal: 3, expectedCount: 2},
		{name: "OffsetSkips", keys: []string{"A", "B", "C"}, limit: 10, offset: 2, expectedTotal: 3, expectedCount: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, svc := newTestService()
			ctx := context.Background()

			for _, key := range test.keys {
				_, svcErr := svc.CreateGatewayVariable(ctx, "env-1", CreateGatewayVariableRequest{
					Key: key, Value: "v" + key,
				})
				require.Nil(t, svcErr)
			}

			resp, svcErr := svc.GetGatewayVariableList(ctx, "env-1", test.limit, test.offset)

			require.Nil(t, svcErr)
			assert.Equal(t, test.expectedTotal, resp.TotalResults)
			assert.Equal(t, test.expectedCount, resp.Count)
			assert.Len(t, resp.GatewayVariables, test.expectedCount)
		})
	}
}

func TestUpdateGatewayVariable(t *testing.T) {
	tests := []struct {
		name        string
		create      bool
		request     UpdateGatewayVariableRequest
		expectedErr string
	}{
		{
			name:    "Success",
			create:  true,
			request: UpdateGatewayVariableRequest{Value: "https://app/new", Description: "updated"},
		},
		{
			name:        "NotFound",
			request:     UpdateGatewayVariableRequest{Value: "x"},
			expectedErr: ErrorGatewayVariableNotFound.Code,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, svc := newTestService()
			ctx := context.Background()

			id := missingID
			if test.create {
				created, svcErr := svc.CreateGatewayVariable(ctx, "env-1", CreateGatewayVariableRequest{
					Key: "REDIRECT_URL", Value: "https://app/old",
				})
				require.Nil(t, svcErr)
				id = created.ID
			}

			updated, svcErr := svc.UpdateGatewayVariable(ctx, "env-1", id, test.request)

			if test.expectedErr != "" {
				require.NotNil(t, svcErr)
				assert.Equal(t, test.expectedErr, svcErr.Code)
				return
			}

			require.Nil(t, svcErr)
			assert.Equal(t, test.request.Value, updated.Value)
			assert.Equal(t, test.request.Description, updated.Description)
			assert.Equal(t, test.request.Value, store.byID[id].Value)
		})
	}
}

func TestDeleteGatewayVariable(t *testing.T) {
	tests := []struct {
		name        string
		create      bool
		expectedErr string
	}{
		{name: "Success", create: true},
		{name: "NotFound", expectedErr: ErrorGatewayVariableNotFound.Code},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, svc := newTestService()
			ctx := context.Background()

			id := missingID
			if test.create {
				created, svcErr := svc.CreateGatewayVariable(ctx, "env-1", CreateGatewayVariableRequest{
					Key: "REDIRECT_URL", Value: "v",
				})
				require.Nil(t, svcErr)
				id = created.ID
			}

			svcErr := svc.DeleteGatewayVariable(ctx, "env-1", id)

			if test.expectedErr != "" {
				require.NotNil(t, svcErr)
				assert.Equal(t, test.expectedErr, svcErr.Code)
				return
			}

			require.Nil(t, svcErr)
			_, svcErr = svc.GetGatewayVariable(ctx, "env-1", id)
			require.NotNil(t, svcErr)
			assert.Equal(t, ErrorGatewayVariableNotFound.Code, svcErr.Code)
		})
	}
}

func TestResolveGatewayVariables(t *testing.T) {
	tests := []struct {
		name     string
		created  map[string]string
		expected map[string]string
	}{
		{name: "Empty", expected: map[string]string{}},
		{
			name:     "AllKeysAndValues",
			created:  map[string]string{"A": "alpha", "B": "beta"},
			expected: map[string]string{"A": "alpha", "B": "beta"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, svc := newTestService()
			ctx := context.Background()

			for key, value := range test.created {
				_, svcErr := svc.CreateGatewayVariable(ctx, "env-1", CreateGatewayVariableRequest{
					Key: key, Value: value,
				})
				require.Nil(t, svcErr)
			}

			values, svcErr := svc.ResolveGatewayVariables(ctx, "env-1")

			require.Nil(t, svcErr)
			assert.Equal(t, test.expected, values)
		})
	}
}

func TestServiceStoreFailuresReturnInternalError(t *testing.T) {
	tests := []struct {
		name string
		call func(ctx context.Context, svc GatewayVariableServiceInterface) *tidcommon.ServiceError
	}{
		{
			name: "Create",
			call: func(ctx context.Context, svc GatewayVariableServiceInterface) *tidcommon.ServiceError {
				_, svcErr := svc.CreateGatewayVariable(ctx, "env-1",
					CreateGatewayVariableRequest{Key: "K", Value: "v"})
				return svcErr
			},
		},
		{
			name: "Get",
			call: func(ctx context.Context, svc GatewayVariableServiceInterface) *tidcommon.ServiceError {
				_, svcErr := svc.GetGatewayVariable(ctx, "env-1", missingID)
				return svcErr
			},
		},
		{
			name: "List",
			call: func(ctx context.Context, svc GatewayVariableServiceInterface) *tidcommon.ServiceError {
				_, svcErr := svc.GetGatewayVariableList(ctx, "env-1", 10, 0)
				return svcErr
			},
		},
		{
			name: "Update",
			call: func(ctx context.Context, svc GatewayVariableServiceInterface) *tidcommon.ServiceError {
				_, svcErr := svc.UpdateGatewayVariable(ctx, "env-1", missingID,
					UpdateGatewayVariableRequest{Value: "v"})
				return svcErr
			},
		},
		{
			name: "Delete",
			call: func(ctx context.Context, svc GatewayVariableServiceInterface) *tidcommon.ServiceError {
				return svc.DeleteGatewayVariable(ctx, "env-1", missingID)
			},
		},
		{
			name: "Resolve",
			call: func(ctx context.Context, svc GatewayVariableServiceInterface) *tidcommon.ServiceError {
				_, svcErr := svc.ResolveGatewayVariables(ctx, "env-1")
				return svcErr
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := newGatewayVariableService(&failingStore{})

			svcErr := test.call(context.Background(), svc)

			require.NotNil(t, svcErr)
			assert.Equal(t, ErrorInternalServer.Code, svcErr.Code)
		})
	}
}

// The same key means different things in different gateways: a redirect URL points at dev in one
// and at prod in the next. Keys are therefore unique within an gateway, not across the
// organization, and one gateway's variables are invisible to another.
func TestGatewayVariablesAreIsolatedPerGateway(t *testing.T) {
	ctx := context.Background()
	svc := newGatewayVariableService(newFakeStore())

	dev, svcErr := svc.CreateGatewayVariable(ctx, "env-dev", CreateGatewayVariableRequest{
		Key: "APP_REDIRECT_URL", Value: "https://dev.example.com/callback",
	})
	if svcErr != nil {
		t.Fatalf("create in dev: %v", svcErr)
	}
	prod, svcErr := svc.CreateGatewayVariable(ctx, "env-prod", CreateGatewayVariableRequest{
		Key: "APP_REDIRECT_URL", Value: "https://prod.example.com/callback",
	})
	if svcErr != nil {
		t.Fatalf("the same key must be allowed in another gateway: %v", svcErr)
	}

	values, svcErr := svc.ResolveGatewayVariables(ctx, "env-dev")
	if svcErr != nil {
		t.Fatalf("resolve dev: %v", svcErr)
	}
	if values["APP_REDIRECT_URL"] != "https://dev.example.com/callback" {
		t.Fatalf("dev must resolve its own value, got %q", values["APP_REDIRECT_URL"])
	}

	// A second variable under the same key in the same gateway is still a conflict.
	if _, svcErr := svc.CreateGatewayVariable(ctx, "env-dev", CreateGatewayVariableRequest{
		Key: "APP_REDIRECT_URL", Value: "https://other.example.com/callback",
	}); svcErr == nil {
		t.Fatal("a duplicate key within one gateway must be refused")
	}

	// Neither gateway can reach the other's variable by id.
	if _, svcErr := svc.GetGatewayVariable(ctx, "env-dev", prod.ID); svcErr == nil {
		t.Fatal("dev must not be able to read prod's variable")
	}
	if _, svcErr := svc.GetGatewayVariable(ctx, "env-prod", dev.ID); svcErr == nil {
		t.Fatal("prod must not be able to read dev's variable")
	}
}
