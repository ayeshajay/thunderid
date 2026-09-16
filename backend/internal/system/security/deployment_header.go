// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// trustsHeaders records whether this server accepts a deployment id named in a request header.
//
// It is declared by the binary rather than read from configuration, because trusting a header is a
// property of the plane: a control plane fronts many tenants and can have an intermediary acting for
// them, while a gateway serves exactly one deployment and has nothing an intermediary could name.
// Leaving it to configuration alone would let a gateway be talked into honoring the header by a
// stray setting.
var trustsHeaders atomic.Bool

// TrustDeploymentHeaders declares that this server may take the deployment id from a request header
// when the caller also presents the configured key.
func TrustDeploymentHeaders() {
	trustsHeaders.Store(true)
}

// headerDeploymentID returns the deployment id a trusted caller named in the request headers, and
// whether the request presented the key at all.
//
// The three answers it distinguishes:
//   - ("", false)    the caller named nothing, so the token decides.
//   - (id, true)     the caller proved the key and named a deployment.
//   - ("", true)     the caller presented a key that is wrong, or named no deployment with it. The
//     request is refused rather than quietly falling back to the token: presenting
//     the key is a claim to act for another deployment, and a claim that does not
//     hold up should be answered, not ignored.
func headerDeploymentID(r *http.Request) (string, bool) {
	if !trustsHeaders.Load() || !config.IsServerRuntimeInitialized() {
		return "", false
	}
	cfg := config.GetServerRuntime().Config.Server.DeploymentIDHeader
	if !cfg.Honors() {
		return "", false
	}

	presented := strings.TrimSpace(r.Header.Get(cfg.KeyHeaderName()))
	if presented == "" {
		return "", false
	}
	// Constant time, so a wrong key cannot be discovered by how quickly it is rejected.
	if subtle.ConstantTimeCompare([]byte(presented), []byte(strings.TrimSpace(cfg.Key))) != 1 {
		return "", true
	}
	return strings.TrimSpace(r.Header.Get(cfg.IDHeaderName())), true
}
