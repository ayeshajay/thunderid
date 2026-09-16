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

package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/gateway/model"
	"github.com/thunder-id/thunderid/internal/gateway/store"
)

// memStore is an in-memory Store for the service tests.
//
// It mirrors what the database-backed store guarantees, including the version pruning, so these
// tests exercise the service rather than a database.
type memStore struct {
	envs map[string]model.Gateway
	// versions belong to the organization, so they are keyed by sequence alone.
	versions map[int]model.Version
	// applies is each gateway's history, newest first.
	applies map[string][]model.Apply
	jobs    map[string]store.Job
	jobSeq  int
}

func newMemStore() *memStore {
	return &memStore{
		envs:     map[string]model.Gateway{},
		versions: map[int]model.Version{},
		applies:  map[string][]model.Apply{},
		jobs:     map[string]store.Job{},
	}
}

func (m *memStore) SaveGateway(_ context.Context, env model.Gateway) error {
	m.envs[env.ID] = env
	return nil
}

func (m *memStore) GetGateway(_ context.Context, id string) (model.Gateway, error) {
	env, ok := m.envs[id]
	if !ok {
		return model.Gateway{}, store.ErrNotFound
	}
	return env, nil
}

func (m *memStore) ListGateways(_ context.Context) ([]model.Gateway, error) {
	out := make([]model.Gateway, 0, len(m.envs))
	for _, env := range m.envs {
		out = append(out, env)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *memStore) DeleteGateway(_ context.Context, id string) error {
	if _, ok := m.envs[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.envs, id)
	delete(m.applies, id)
	return nil
}

func (m *memStore) AddVersion(_ context.Context, v model.Version) (model.Version, error) {
	seqs := m.seqs()
	next := 1
	if len(seqs) > 0 {
		next = seqs[len(seqs)-1] + 1
	}
	v.Seq = next
	m.versions[v.Seq] = v

	keep := map[int]bool{}
	all := m.seqs()
	for i := len(all) - 1; i >= 0 && len(keep) < store.KeepVersions(); i-- {
		keep[all[i]] = true
	}
	// A version some gateway has run stays, so its history never points at nothing.
	for _, history := range m.applies {
		for _, entry := range history {
			keep[entry.Seq] = true
		}
	}
	for _, seq := range all {
		if !keep[seq] {
			delete(m.versions, seq)
		}
	}
	return v, nil
}

func (m *memStore) GetVersion(_ context.Context, seq int) (model.Version, error) {
	v, ok := m.versions[seq]
	if !ok {
		return model.Version{}, store.ErrNotFound
	}
	return v, nil
}

func (m *memStore) ListVersions(_ context.Context) ([]model.Version, error) {
	seqs := m.seqs()
	out := make([]model.Version, 0, len(seqs))
	for i := len(seqs) - 1; i >= 0; i-- {
		v := m.versions[seqs[i]]
		v.Resources = ""
		v.Variables = nil
		out = append(out, v)
	}
	return out, nil
}

func (m *memStore) LatestSeq(_ context.Context) (int, error) {
	seqs := m.seqs()
	if len(seqs) == 0 {
		return 0, nil
	}
	return seqs[len(seqs)-1], nil
}

func (m *memStore) RecordApply(_ context.Context, gatewayID string, seq int) (model.Apply, error) {
	next := 1
	if existing := m.applies[gatewayID]; len(existing) > 0 {
		next = existing[0].Ordinal + 1
	}
	entry := model.Apply{Ordinal: next, Seq: seq}
	history := append([]model.Apply{entry}, m.applies[gatewayID]...)
	if len(history) > store.KeepApplies {
		history = history[:store.KeepApplies]
	}
	m.applies[gatewayID] = history
	return entry, nil
}

func (m *memStore) ListApplies(_ context.Context, gatewayID string) ([]model.Apply, error) {
	return append([]model.Apply(nil), m.applies[gatewayID]...), nil
}

// seqs returns the organization's stored sequences in ascending order.
func (m *memStore) seqs() []int {
	seqs := make([]int, 0, len(m.versions))
	for seq := range m.versions {
		seqs = append(seqs, seq)
	}
	sort.Ints(seqs)
	return seqs
}

// ---- queued work ----

func (m *memStore) EnqueueJob(_ context.Context, job store.Job) (store.Job, error) {
	if job.ID == "" {
		m.jobSeq++
		job.ID = fmt.Sprintf("job-%d", m.jobSeq)
	}
	job.Status = store.JobPending
	if m.jobs == nil {
		m.jobs = map[string]store.Job{}
	}
	m.jobs[job.ID] = job
	return job, nil
}

func (m *memStore) GetJob(_ context.Context, id string) (store.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return store.Job{}, store.ErrNotFound
	}
	return job, nil
}

func (m *memStore) ClaimNextJob(_ context.Context, dataPlaneID, claimedBy string) (store.Job, bool, error) {
	for id, job := range m.jobs {
		if job.DataPlaneID != dataPlaneID || job.Status != store.JobPending {
			continue
		}
		job.Status = store.JobClaimed
		job.Attempts++
		m.jobs[id] = job
		return job, true, nil
	}
	return store.Job{}, false, nil
}

func (m *memStore) CompleteJob(_ context.Context, _, id, result, failure string) error {
	job, ok := m.jobs[id]
	if !ok {
		return store.ErrNotFound
	}
	job.Status = store.JobDone
	if failure != "" {
		job.Status = store.JobFailed
	}
	job.Result, job.Error = result, failure
	m.jobs[id] = job
	return nil
}

func (m *memStore) ReleaseJob(_ context.Context, _, id string) error {
	if job, ok := m.jobs[id]; ok {
		job.Status = store.JobPending
		m.jobs[id] = job
	}
	return nil
}

// fakeSealer stands in for the server's encryption. It reverses the bytes, which is enough to prove
// a queued credential is transformed on the way in and restored on the way out.
type fakeSealer struct{}

func (fakeSealer) Seal(_ context.Context, plaintext []byte) ([]byte, error) {
	return reversed(plaintext), nil
}

func (fakeSealer) Open(_ context.Context, sealed []byte) ([]byte, error) {
	return reversed(sealed), nil
}

func reversed(in []byte) []byte {
	out := make([]byte, len(in))
	for i, b := range in {
		out[len(in)-1-i] = b
	}
	return out
}
