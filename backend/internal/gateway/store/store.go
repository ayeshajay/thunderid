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

// Package store persists an organization's gateways, its captured versions, and what each gateway
// has run.
//
// Both are bounded. The organization keeps the newest versions its configuration allows, and each
// gateway keeps its
// newest KeepApplies entries. A version an entry still names is kept whatever its age, so going back
// never lands on a version that has been pruned away.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/thunder-id/thunderid/internal/gateway/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// KeepVersions is how many of the organization's versions are retained, from configuration. A
// deployment that has not said falls back to the product default.
//
// Read per prune rather than captured at construction, because a Control Plane replica is long lived
// and the number is small enough that reading it is free.
func KeepVersions() int {
	if !config.IsServerRuntimeInitialized() {
		return config.DefaultKeepVersions
	}
	return config.GetServerRuntime().Config.Versions.KeepVersions()
}

// KeepApplies is how many entries a gateway's history retains, which is how far back it can be
// returned to.
const KeepApplies = 3

// ErrNotFound is returned when an gateway or version does not exist.
var ErrNotFound = errors.New("not found")

// Store holds one deployment's gateways and their captured versions.
//
// The deployment is the organization, not one of its gateways: a deployment id names an
// gateway as "<org>:<env>", and promotion compares one gateway against another, so the
// whole chain an organization promotes through belongs to a single store.
//
// Rows are the shared state every Control Plane replica reads and writes, so nothing is cached here.
type Store struct {
	deploymentID string
}

// New returns a store scoped to a deployment.
func New(deploymentID string) (*Store, error) {
	if deploymentID == "" {
		return nil, errors.New("a deployment id is required to open an gateway store")
	}
	return &Store{deploymentID: deploymentID}, nil
}

// client resolves the gateway datasource. It is resolved per call rather than held, so building
// a store for a deployment does not open a connection before anything asks it for one.
func (s *Store) client() (provider.DBClientInterface, error) {
	dbClient, err := provider.GetDBProvider().GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

// Deployments lists every deployment that has at least one gateway.
//
// It reaches across deployments on purpose, and is the only thing here that does: seeding a new
// tenant means finding which organization's chain already manages the tenant being copied from,
// which cannot be answered from inside one store.
func Deployments(ctx context.Context) ([]string, error) {
	dbClient, err := provider.GetDBProvider().GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryListDeployments)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	ids := make([]string, 0, len(results))
	for _, row := range results {
		if id := string(toBytes(row["deployment_id"])); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ---- gateways ----

// SaveGateway inserts or updates an gateway.
func (s *Store) SaveGateway(ctx context.Context, env model.Gateway) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to encode gateway: %w", err)
	}

	dbClient, err := s.client()
	if err != nil {
		return err
	}

	if _, err := dbClient.QueryContext(ctx, querySaveGateway,
		s.deploymentID, env.ID, string(raw)); err != nil {
		return fmt.Errorf("failed to save gateway: %w", err)
	}
	return nil
}

// GetGateway returns an gateway by id.
func (s *Store) GetGateway(ctx context.Context, id string) (model.Gateway, error) {
	dbClient, err := s.client()
	if err != nil {
		return model.Gateway{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetGateway, s.deploymentID, id)
	if err != nil {
		return model.Gateway{}, fmt.Errorf("failed to read gateway: %w", err)
	}
	if len(results) == 0 {
		return model.Gateway{}, ErrNotFound
	}
	return decodeGateway(results[0]["data"])
}

// ListGateways returns all gateways ordered by name.
func (s *Store) ListGateways(ctx context.Context) ([]model.Gateway, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListGateways, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list gateways: %w", err)
	}

	out := make([]model.Gateway, 0, len(results))
	for _, row := range results {
		env, err := decodeGateway(row["data"])
		if err != nil {
			return nil, err
		}
		out = append(out, env)
	}
	// Gateways are unordered, so they are listed by name: a stable order a reader can predict.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// DeleteGateway removes an gateway and all of its versions.
func (s *Store) DeleteGateway(ctx context.Context, id string) error {
	if _, err := s.GetGateway(ctx, id); err != nil {
		return err
	}

	dbClient, err := s.client()
	if err != nil {
		return err
	}

	// Versions are removed by the foreign key, which cascades.
	if _, err := dbClient.QueryContext(ctx, queryDeleteGateway, s.deploymentID, id); err != nil {
		return fmt.Errorf("failed to delete gateway: %w", err)
	}
	return nil
}

// ---- versions ----

// AddVersion stores a new version for the organization, assigning the next sequence.
//
// The sequence is read and then written rather than assigned by the database. Two captures at the
// same instant therefore collide on the primary key, and the second is refused: the history stays
// correct and the caller can capture again.
func (s *Store) AddVersion(ctx context.Context, v model.Version) (model.Version, error) {
	seqs, err := s.versionSeqs(ctx)
	if err != nil {
		return model.Version{}, err
	}
	next := 1
	if len(seqs) > 0 {
		next = seqs[len(seqs)-1] + 1
	}
	v.Seq = next

	raw, err := json.Marshal(v)
	if err != nil {
		return model.Version{}, fmt.Errorf("failed to encode version: %w", err)
	}

	dbClient, err := s.client()
	if err != nil {
		return model.Version{}, err
	}
	if _, err := dbClient.QueryContext(ctx, queryInsertVersion, s.deploymentID, v.Seq, string(raw)); err != nil {
		return model.Version{}, fmt.Errorf("failed to store version: %w", err)
	}

	if err := s.prune(ctx); err != nil {
		return model.Version{}, err
	}
	return v, nil
}

// GetVersion returns a full version (including resources and variables).
func (s *Store) GetVersion(ctx context.Context, seq int) (model.Version, error) {
	dbClient, err := s.client()
	if err != nil {
		return model.Version{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetVersion, s.deploymentID, seq)
	if err != nil {
		return model.Version{}, fmt.Errorf("failed to read version: %w", err)
	}
	if len(results) == 0 {
		return model.Version{}, ErrNotFound
	}
	return decodeVersion(results[0]["data"])
}

// ListVersions returns version metadata (payload stripped) newest first.
func (s *Store) ListVersions(ctx context.Context) ([]model.Version, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListVersions, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	out := make([]model.Version, 0, len(results))
	for _, row := range results {
		v, err := decodeVersion(row["data"])
		if err != nil {
			return nil, err
		}
		v.Resources = ""
		v.Variables = nil
		out = append(out, v)
	}
	return out, nil
}

// LatestSeq returns the organization's highest version sequence, or 0 if none exist.
func (s *Store) LatestSeq(ctx context.Context) (int, error) {
	seqs, err := s.versionSeqs(ctx)
	if err != nil {
		return 0, err
	}
	if len(seqs) == 0 {
		return 0, nil
	}
	return seqs[len(seqs)-1], nil
}

// ---- gateway history ----

// RecordApply appends to a gateway's history that it was moved onto a version.
func (s *Store) RecordApply(ctx context.Context, gatewayID string, seq int) (model.Apply, error) {
	history, err := s.ListApplies(ctx, gatewayID)
	if err != nil {
		return model.Apply{}, err
	}
	next := 1
	if len(history) > 0 {
		next = history[0].Ordinal + 1
	}

	dbClient, err := s.client()
	if err != nil {
		return model.Apply{}, err
	}
	if _, err := dbClient.QueryContext(ctx, queryInsertApply,
		s.deploymentID, gatewayID, next, seq); err != nil {
		return model.Apply{}, fmt.Errorf("failed to record the apply: %w", err)
	}
	// Older entries fall off the end, so a gateway can be returned to what it ran recently rather
	// than to everything it has ever run.
	if _, err := dbClient.ExecuteContext(ctx, queryTrimApplies,
		s.deploymentID, gatewayID, next-KeepApplies); err != nil {
		return model.Apply{}, fmt.Errorf("failed to trim the gateway history: %w", err)
	}
	return model.Apply{Ordinal: next, Seq: seq}, nil
}

// ListApplies returns what a gateway has run, newest first.
func (s *Store) ListApplies(ctx context.Context, gatewayID string) ([]model.Apply, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListApplies, s.deploymentID, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway history: %w", err)
	}

	out := make([]model.Apply, 0, len(results))
	for _, row := range results {
		out = append(out, model.Apply{
			Ordinal:   parseInt(row["ordinal"]),
			Seq:       parseInt(row["seq"]),
			AppliedAt: parseTime(row["applied_at"]),
		})
	}
	return out, nil
}

// ---- internals ----

// versionSeqs returns the organization's version sequences in ascending order.
func (s *Store) versionSeqs(ctx context.Context) ([]int, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryVersionSeqs, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list version sequences: %w", err)
	}

	seqs := make([]int, 0, len(results))
	for _, row := range results {
		seqs = append(seqs, parseInt(row["seq"]))
	}
	return seqs, nil
}

// prune keeps the organization's newest configured number of versions, plus any version a gateway's
// history still
// names.
//
// A version named by a history entry is kept however old it is: that history is what "go back to
// what this gateway was running before" reads, and pruning a version out from under it would leave
// the entry pointing at nothing. Histories are themselves bounded, so what this holds beyond the
// window is bounded too.
func (s *Store) prune(ctx context.Context) error {
	seqs, err := s.versionSeqs(ctx)
	if err != nil {
		return err
	}
	keep := map[int]bool{}
	retain := KeepVersions()
	for i := len(seqs) - 1; i >= 0 && len(keep) < retain; i-- {
		keep[seqs[i]] = true
	}

	dbClient, err := s.client()
	if err != nil {
		return err
	}
	applied, err := dbClient.QueryContext(ctx, queryAppliedSeqs, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to read what the gateways have run: %w", err)
	}
	for _, row := range applied {
		keep[parseInt(row["seq"])] = true
	}

	for _, seq := range seqs {
		if keep[seq] {
			continue
		}
		if _, err := dbClient.QueryContext(ctx, queryDeleteVersion, s.deploymentID, seq); err != nil {
			return fmt.Errorf("failed to prune version %d: %w", seq, err)
		}
	}
	return nil
}

// decodeGateway parses a stored gateway document.
func decodeGateway(value interface{}) (model.Gateway, error) {
	var env model.Gateway
	if err := json.Unmarshal(toBytes(value), &env); err != nil {
		return model.Gateway{}, fmt.Errorf("failed to parse gateway: %w", err)
	}
	return env, nil
}

// decodeVersion parses a stored version document.
func decodeVersion(value interface{}) (model.Version, error) {
	var v model.Version
	if err := json.Unmarshal(toBytes(value), &v); err != nil {
		return model.Version{}, fmt.Errorf("failed to parse version: %w", err)
	}
	return v, nil
}

// toBytes coerces a text column value, which a driver may hand back as either form.
func toBytes(value interface{}) []byte {
	switch v := value.(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return nil
	}
}

// parseTime coerces a timestamp column value, which a driver hands back as a time or as text.
func parseTime(value interface{}) time.Time {
	switch v := value.(type) {
	case time.Time:
		return v.UTC()
	case []byte, string:
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, string(toBytes(value))); err == nil {
				return t.UTC()
			}
		}
		return time.Time{}
	default:
		return time.Time{}
	}
}

// parseInt coerces an integer column value, which a driver may widen.
func parseInt(value interface{}) int {
	switch v := value.(type) {
	case int64:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
