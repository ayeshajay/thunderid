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

// Package model holds the core domain types for the gateway-management service.
package model

import (
	"time"
)

// Origin describes how a version came to exist.
type Origin string

const (
	// OriginCaptured means the version was captured from the organization's configuration.
	OriginCaptured Origin = "captured"
	// OriginUploaded means the version's payload was supplied directly by the caller.
	OriginUploaded Origin = "uploaded"
)

// Apply is one entry in a gateway's history: an organization version that was applied to it, and
// when. A gateway's history is these entries in order, and going back to what it ran before means
// applying the version an earlier entry names.
type Apply struct {
	// Ordinal rises by one per gateway. The highest is what the gateway is running.
	Ordinal   int       `json:"ordinal"`
	Seq       int       `json:"seq"`
	AppliedAt time.Time `json:"appliedAt"`
}

// Target identifies a data plane that a version is applied to.
//
// The data plane is named, not addressed. It dials the control plane and holds that connection open,
// so the control plane reaches it over that channel rather than over its management API: there is no
// URL to call and no credential to hold. DataPlaneID is the id the data plane presents when it
// connects, and BaseURL is kept only to show an operator where that deployment serves.
type Target struct {
	DataPlaneID string `json:"dataPlaneId"`
	BaseURL     string `json:"baseUrl,omitempty"`
}

// DataPlaneStatus reports whether a data plane is connected to this control plane. It is part of the
// gateway as read, not as stored: an operator about to promote needs to know the destination can
// be reached, and nothing can be applied to a data plane that is not.
type DataPlaneStatus struct {
	Connected bool      `json:"connected"`
	LastSeen  time.Time `json:"lastSeen,omitempty"`
}

// Gateway is one gateway an organization's configuration can be applied to.
//
// It is a resource of the organization rather than a deployment of its own: the organization has a
// single workspace that every gateway captures from, and a gateway holds the versions, variables and
// secrets that reach its own data plane.
//
// Gateways are unordered. Which one may be promoted into which is not modeled here: that hierarchy
// belongs to the organization and is held outside this server, so the set here is a flat one.
type Gateway struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Target Target `json:"target"`
	// ManagedByControlPlane marks the one gateway the control plane administers directly, rather
	// than only promotes into. Editing configuration in the organization's workspace is editing this
	// gateway; every other gateway receives that configuration when it is promoted and
	// applied.
	//
	// It decides where a credential created in the workspace is issued. A credential is created once,
	// but each gateway holds its own, and sending it everywhere would set the credential running
	// in production from a change made while developing. It goes here alone; the others receive theirs
	// when one is set against them deliberately.
	//
	// Exactly one gateway of an organization holds this. Typically it is the development
	// gateway, but nothing requires that: the organization's first gateway takes the mark as it is
	// created, and it can be moved afterwards.
	ManagedByControlPlane bool `json:"managedByControlPlane,omitempty"`
	// Attributes is what the gateway manager records about this gateway, such as where it sits in
	// the organization's gateway hierarchy.
	//
	// This server never reads it. The hierarchy is not modeled here on purpose, and inventing typed
	// fields for another service's model would fix a shape that service is free to change; keeping it
	// opaque means it can record what it needs without a change here. It is written by the caller that
	// holds the hierarchy, and returned unchanged.
	Attributes map[string]string `json:"attributes,omitempty"`
	// Excluded lists resource keys a user chose not to promote into this gateway. The choice is
	// remembered rather than asked again on every run: a resource held back once stays held back until
	// it is deliberately selected again, at which point it is dropped from this list and promotes from
	// then on. A key here is skipped by both promotion and apply.
	Excluded []string `json:"excluded,omitempty"`
	// AppliedSeq is the version sequence last applied to Target (0 when nothing has been applied).
	AppliedSeq int `json:"appliedSeq"`
	// SecretNames is what this gateway's data plane last reported holding, and SecretNamesAt when
	// it said so. It is recorded because only the control plane pod holding that data plane's
	// connection can ask: any other pod answers from this instead of failing. Names only, never
	// values.
	SecretNames   []string  `json:"secretNames,omitempty"`
	SecretNamesAt time.Time `json:"secretNamesAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Version is an immutable snapshot of the organization's declarative configuration. The
// parameterized resource YAML and the externalized variable values are stored alongside the
// metadata.
//
// A version belongs to the organization rather than to any gateway. It names no gateway because it
// is not of one: the same version is what every gateway of the organization can be moved onto.
type Version struct {
	Seq       int               `json:"seq"`
	Origin    Origin            `json:"origin"`
	Note      string            `json:"note,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	Resources string            `json:"resources,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	// SecretKeys are the placeholders backed by a secret. Their values are deliberately absent: an
	// apply sends a ${KEY} placeholder for each, leaving the data plane to supply the real value, so
	// secrets never pass through this service.
	SecretKeys []string `json:"secretKeys,omitempty"`
}
