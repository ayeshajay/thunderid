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
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/thunder-id/thunderid/internal/gateway/model"
	"github.com/thunder-id/thunderid/internal/gateway/store"
	"github.com/thunder-id/thunderid/internal/gateway/thunder"
)

// SecretSealer encrypts a queued payload that carries a credential, and opens it again when it is
// delivered. A credential waiting for a Data Plane is never held in the clear.
type SecretSealer interface {
	Seal(ctx context.Context, plaintext []byte) ([]byte, error)
	Open(ctx context.Context, sealed []byte) ([]byte, error)
}

// SetSecretSealer installs what encrypts a queued credential. It is separate from New because the
// server's crypto is built after this service.
func (s *Service) SetSecretSealer(sealer SecretSealer) { s.sealer = sealer }

// secretPutPayload is a queued Secret.Put.
type secretPutPayload struct {
	Name string                 `json:"name"`
	Body map[string]interface{} `json:"body"`
}

// podID names this pod in a claim, so a job stuck in flight can be traced to the process holding it.
func podID() string {
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	return "unknown"
}

// dispatch records work for an gateway's Data Plane and then tries to carry it out here.
//
// The write comes first and always: a pod holding no connection to that Data Plane can still accept
// the request, and whichever pod holds one delivers it. Trying immediately afterwards is what keeps
// the common case fast, where this pod does hold the connection and the caller gets its answer in
// the same response rather than after a poll.
func (s *Service) dispatch(ctx context.Context, env model.Gateway, jobType string,
	payload []byte, carriesSecret bool) (store.Job, error) {
	if carriesSecret {
		if s.sealer == nil {
			return store.Job{}, errors.New("no encryption is configured, so a credential cannot be queued")
		}
		sealed, err := s.sealer.Seal(ctx, payload)
		if err != nil {
			return store.Job{}, fmt.Errorf("failed to encrypt the queued credential: %w", err)
		}
		payload = sealed
	}

	job, err := s.store.EnqueueJob(ctx, store.Job{
		DataPlaneID: env.Target.DataPlaneID,
		GatewayID:   env.ID,
		Type:        jobType,
		Payload:     payload,
		Encrypted:   carriesSecret,
	})
	if err != nil {
		return store.Job{}, err
	}

	// Best effort: a failure here leaves the job pending for another pod, which is the whole point.
	_ = s.DeliverNext(ctx, env.Target.DataPlaneID)

	return s.store.GetJob(ctx, job.ID)
}

// DeliverNext carries out one queued job for a Data Plane, when this pod holds its connection and
// there is work waiting. It reports nothing when there is nothing to do.
//
// It is called immediately after queueing, and again on a timer, so a job queued on a pod with no
// connection is picked up by one that has it.
func (s *Service) DeliverNext(ctx context.Context, dataPlaneID string) error {
	job, claimed, err := s.store.ClaimNextJob(ctx, dataPlaneID, podID())
	if err != nil || !claimed {
		return err
	}

	plane, err := s.dataPlanes.For(dataPlaneID)
	if err != nil {
		// Not this pod's to deliver. Put it back rather than record a failure: nothing was attempted.
		return s.store.ReleaseJob(ctx, job.DeploymentID, job.ID)
	}

	result, failure := s.runJob(ctx, plane, job)
	return s.store.CompleteJob(ctx, job.DeploymentID, job.ID, result, failure)
}

// runJob performs one job against a connected Data Plane and returns what to record: the answer as
// JSON, or the reason it failed.
func (s *Service) runJob(ctx context.Context, plane DataPlane, job store.Job) (string, string) {
	payload := job.Payload
	if job.Encrypted {
		if s.sealer == nil {
			return "", "no encryption is configured, so the queued credential cannot be read"
		}
		opened, err := s.sealer.Open(ctx, payload)
		if err != nil {
			return "", fmt.Sprintf("failed to decrypt the queued credential: %v", err)
		}
		payload = opened
	}

	switch job.Type {
	case store.JobTypeImport:
		var in importPayload
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Sprintf("failed to read the queued import: %v", err)
		}
		resp, err := plane.Import(ctx, in.Request)
		if err != nil {
			return "", fmt.Sprintf("import failed: %v", err)
		}
		// The gateway is marked applied here rather than where the apply was requested, because
		// until this moment the data plane had not taken the configuration. A dry run changes nothing.
		if !in.DryRun {
			if err := s.recordApplied(ctx, in.GatewayID, in.TargetSeq); err != nil {
				return "", fmt.Sprintf("the import succeeded but the gateway could not be updated: %v", err)
			}
		}
		encoded, err := json.Marshal(resp)
		if err != nil {
			return "", fmt.Sprintf("failed to record the import result: %v", err)
		}
		return string(encoded), ""

	case store.JobTypeSecretNames:
		names, err := plane.SecretNames(ctx)
		if err != nil {
			return "", fmt.Sprintf("failed to read the credentials held: %v", err)
		}
		// Record it against the gateway as well, so the listing that prompted this answers from
		// the record on whichever pod serves the next request.
		if err := s.recordSecretNames(ctx, job.GatewayID, names); err != nil {
			return "", fmt.Sprintf("failed to record the credentials held: %v", err)
		}
		encoded, err := json.Marshal(names)
		if err != nil {
			return "", fmt.Sprintf("failed to record the credentials held: %v", err)
		}
		return string(encoded), ""

	case store.JobTypeSecretPut:
		var put secretPutPayload
		if err := json.Unmarshal(payload, &put); err != nil {
			return "", fmt.Sprintf("failed to read the queued credential: %v", err)
		}
		if err := plane.PutSecret(ctx, put.Name, put.Body); err != nil {
			return "", fmt.Sprintf("failed to store the credential: %v", err)
		}
		return "{}", ""

	default:
		return "", fmt.Sprintf("unrecognized job type %q", job.Type)
	}
}

// JobStatus returns a queued job, which is how a caller collects the answer for the id it was given.
func (s *Service) JobStatus(ctx context.Context, id string) (store.Job, error) {
	return s.store.GetJob(ctx, id)
}

// importPayload is a queued Import.Run, with what the delivering pod needs to record afterwards.
//
// The gateway and sequence travel with the request because the pod that delivers it is not
// necessarily the one that accepted it, and marking the gateway applied is only correct once the
// data plane has actually taken the configuration.
type importPayload struct {
	Request   thunder.ImportRequest `json:"request"`
	GatewayID string                `json:"gatewayId"`
	TargetSeq int                   `json:"targetSeq"`
	DryRun    bool                  `json:"dryRun"`
}

// recordApplied marks a gateway as running a version, once its data plane has taken it, and appends
// that to the gateway's history.
//
// Both happen here rather than where the apply was requested, because only a delivered apply is one
// the gateway actually ran: a history that recorded intent would offer a caller a state to go back
// to that never existed.
func (s *Service) recordApplied(ctx context.Context, envID string, seq int) error {
	env, err := s.store.GetGateway(ctx, envID)
	if err != nil {
		return err
	}
	env.AppliedSeq = seq
	env.UpdatedAt = s.now().UTC()
	if err := s.store.SaveGateway(ctx, env); err != nil {
		return err
	}
	_, err = s.store.RecordApply(ctx, envID, seq)
	return err
}

// queueSecret records a credential for an gateway's data plane, encrypted, and delivers it here
// when this pod holds the connection.
func (s *Service) queueSecret(ctx context.Context, env model.Gateway, name string,
	body map[string]interface{}) (store.Job, error) {
	payload, err := json.Marshal(secretPutPayload{Name: name, Body: body})
	if err != nil {
		return store.Job{}, fmt.Errorf("failed to prepare the credential: %w", err)
	}
	return s.dispatch(ctx, env, store.JobTypeSecretPut, payload, true)
}

// recordSecretNames stores what a data plane reported holding, against its gateway.
func (s *Service) recordSecretNames(ctx context.Context, envID string, names []string) error {
	env, err := s.store.GetGateway(ctx, envID)
	if err != nil {
		return err
	}
	env.SecretNames = names
	env.SecretNamesAt = s.now().UTC()
	return s.store.SaveGateway(ctx, env)
}

// queueSecretNames asks a data plane which credentials it holds, through the queue, for a pod that
// cannot ask it directly. The caller is given the job to follow.
func (s *Service) queueSecretNames(ctx context.Context, env model.Gateway) (store.Job, error) {
	return s.dispatch(ctx, env, store.JobTypeSecretNames, []byte("{}"), false)
}
