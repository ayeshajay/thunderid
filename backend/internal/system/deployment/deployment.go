// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package deployment carries the per-request deployment identifier used to scope persistence.
//
// The id partitions every stored resource by the DEPLOYMENT_ID column. It is put on the request
// context at the edge, so a store reads it from one place rather than reaching for configuration.
// This server holds one deployment, so that id is the configured identifier.
package deployment

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// ctxKey is the private context key under which the per-request deployment id is stored.
type ctxKey struct{}

// WithID returns a context carrying the given deployment id. An empty id is ignored so callers can
// pass an unconditionally-extracted claim without having to branch on its presence.
func WithID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// fromContext returns the deployment id carried by the context, if any.
func fromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok && id != ""
}

// Resolve returns the deployment id a store should scope by for this request.
//
// The id is put on the context at the edge, so a request always carries one and that is what a
// store scopes by. Contexts that never passed through the edge, such as start-up tasks, background
// jobs and command line tooling, fall back to the configured identifier, which this package reads
// so that a store does not have to reach for configuration itself.
//
// The fallback is empty until the server runtime is loaded, which is the correct answer that early:
// there is no deployment to name yet.
func Resolve(ctx context.Context) string {
	if id, ok := fromContext(ctx); ok {
		return id
	}
	if !config.IsServerRuntimeInitialized() {
		return ""
	}
	return config.GetServerRuntime().Config.Server.Identifier
}

// IDFromContext returns the deployment id the context names, and whether it named one.
//
// The distinction matters where "no deployment" is not the same as "the server's own": declarative
// resources loaded at startup run before any request exists, and must see everything.
func IDFromContext(ctx context.Context) (string, bool) {
	return fromContext(ctx)
}

// ResolveDefault returns the deployment id for the request, falling back to the server's own
// identifier. It is for callers that scope by deployment but hold no configured value of their own,
// such as the cache layer.
func ResolveDefault(ctx context.Context) string {
	return Resolve(ctx)
}

// OrganizationOf returns the organization a deployment id belongs to.
//
// A deployment id names a gateway as "<org>:<gateway>", and everything an organization owns across
// its gateways is partitioned under the organization rather than under any one gateway. An id
// naming no organization is its own organization.
func OrganizationOf(id string) string {
	org, _, found := strings.Cut(id, ":")
	if !found || strings.TrimSpace(org) == "" {
		return id
	}
	return org
}

// ResolveOr returns the deployment id the context names, falling back to the given id.
//
// It is for stores that hold a configured deployment id of their own, where the fallback is that
// value rather than the server's. Stores that hold none use Resolve.
func ResolveOr(ctx context.Context, fallback string) string {
	if id, ok := fromContext(ctx); ok {
		return id
	}
	return fallback
}
