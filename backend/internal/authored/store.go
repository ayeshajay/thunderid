// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authored

import (
	"context"
	"fmt"
	"time"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

var queryUpsertResource = dbmodel.DBQuery{
	ID: "AUQ-AUTHORED_MGT-01",
	Query: `INSERT INTO "PARAMETERISED_RESOURCE" (ID, RESOURCE_TYPE, NAME, PAYLOAD, DEPLOYMENT_ID)
	        VALUES ($1, $2, $3, $4, $5)
	        ON CONFLICT (DEPLOYMENT_ID, RESOURCE_TYPE, NAME)
	        DO UPDATE SET PAYLOAD = excluded.PAYLOAD, UPDATED_AT = NOW()`,
	SQLiteQuery: `INSERT INTO "PARAMETERISED_RESOURCE" (ID, RESOURCE_TYPE, NAME, PAYLOAD, DEPLOYMENT_ID)
	              VALUES ($1, $2, $3, $4, $5)
	              ON CONFLICT (DEPLOYMENT_ID, RESOURCE_TYPE, NAME)
	              DO UPDATE SET PAYLOAD = excluded.PAYLOAD, UPDATED_AT = datetime('now')`,
}

var queryGetResource = dbmodel.DBQuery{
	ID: "AUQ-AUTHORED_MGT-02",
	Query: `SELECT ID, RESOURCE_TYPE, NAME, PAYLOAD, CREATED_AT, UPDATED_AT
	        FROM "PARAMETERISED_RESOURCE"
	        WHERE RESOURCE_TYPE = $1 AND NAME = $2 AND DEPLOYMENT_ID = $3`,
}

// queryListResources omits the payload: a listing reports what has been authored, not what is in it.
var queryListResources = dbmodel.DBQuery{
	ID: "AUQ-AUTHORED_MGT-03",
	Query: `SELECT ID, RESOURCE_TYPE, NAME, CREATED_AT, UPDATED_AT
	        FROM "PARAMETERISED_RESOURCE"
	        WHERE DEPLOYMENT_ID = $1 ORDER BY RESOURCE_TYPE, NAME`,
}

var queryDeleteResource = dbmodel.DBQuery{
	ID: "AUQ-AUTHORED_MGT-04",
	Query: `DELETE FROM "PARAMETERISED_RESOURCE"
	        WHERE RESOURCE_TYPE = $1 AND NAME = $2 AND DEPLOYMENT_ID = $3`,
}

// storeInterface is the persistence this package needs.
type storeInterface interface {
	Upsert(ctx context.Context, r Resource) error
	Get(ctx context.Context, resourceType, name string) (Resource, bool, error)
	List(ctx context.Context) ([]Resource, error)
	Delete(ctx context.Context, resourceType, name string) error
}

type store struct {
	dbProvider provider.DBProviderInterface
}

// getDBProvider is a package-level indirection to allow test override.
var getDBProvider = provider.GetDBProvider

func newStore() storeInterface { return &store{dbProvider: getDBProvider()} }

func (s *store) scope(ctx context.Context) string { return deployment.Resolve(ctx) }

func (s *store) Upsert(ctx context.Context, r Resource) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryUpsertResource,
		r.ID, r.ResourceType, r.Name, r.Payload, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to store the authored resource: %w", err)
	}
	return nil
}

func (s *store) Get(ctx context.Context, resourceType, name string) (Resource, bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return Resource{}, false, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetResource, resourceType, name, s.scope(ctx))
	if err != nil {
		return Resource{}, false, fmt.Errorf("failed to read the authored resource: %w", err)
	}
	if len(rows) == 0 {
		return Resource{}, false, nil
	}
	r, err := rowToResource(rows[0])
	return r, err == nil, err
}

func (s *store) List(ctx context.Context) ([]Resource, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryListResources, s.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list authored resources: %w", err)
	}
	resources := make([]Resource, 0, len(rows))
	for _, row := range rows {
		r, err := rowToResource(row)
		if err != nil {
			return nil, err
		}
		resources = append(resources, r)
	}
	return resources, nil
}

func (s *store) Delete(ctx context.Context, resourceType, name string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteResource,
		resourceType, name, s.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete the authored resource: %w", err)
	}
	return nil
}

// rowToResource converts one result row. The payload is absent from a listing row, so it is read
// only when the column is there.
func rowToResource(row map[string]interface{}) (Resource, error) {
	r := Resource{}
	var ok bool
	if r.ID, ok = row["id"].(string); !ok {
		return Resource{}, fmt.Errorf("failed to parse the authored resource id")
	}
	if r.ResourceType, ok = row["resource_type"].(string); !ok {
		return Resource{}, fmt.Errorf("failed to parse the authored resource type")
	}
	if r.Name, ok = row["name"].(string); !ok {
		return Resource{}, fmt.Errorf("failed to parse the authored resource name")
	}
	if payload, present := row["payload"]; present {
		if r.Payload, ok = payload.(string); !ok {
			return Resource{}, fmt.Errorf("failed to parse the authored resource payload")
		}
	}
	r.CreatedAt = timeString(row["created_at"])
	r.UpdatedAt = timeString(row["updated_at"])
	return r, nil
}

// timeString renders a timestamp a driver may hand back as a time or as text.
func timeString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case time.Time:
		return typed.UTC().Format(time.RFC3339)
	default:
		return ""
	}
}
