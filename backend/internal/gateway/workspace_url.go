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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gateway

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// WorkspaceURL is where this server answers, preferring the public URL it advertises and falling back
// to its own host and port. It is the organization workspace a capture reads.
func WorkspaceURL(cfg config.Config) string {
	if url := strings.TrimSpace(cfg.Server.PublicURL); url != "" {
		return url
	}
	return fmt.Sprintf("https://%s:%d", cfg.Server.Hostname, cfg.Server.Port)
}

// WorkspaceCA is the certificate a capture trusts when it reads the workspace.
//
// The workspace is this very server, so what it presents is this server's own certificate. Trusting
// it explicitly is what lets a control plane serving a certificate from a private CA read its own
// configuration, rather than turning verification off to get there.
func WorkspaceCA(cfg config.Config, serverHome string) string {
	certFile := strings.TrimSpace(cfg.TLS.CertFile)
	if certFile == "" || filepath.IsAbs(certFile) {
		return certFile
	}
	return filepath.Join(serverHome, certFile)
}
