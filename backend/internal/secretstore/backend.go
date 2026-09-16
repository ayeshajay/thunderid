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

package secretstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// Backend is where a store's secrets are actually kept.
//
// It exists so that where secrets live is a deployment decision rather than a code one: a single
// instance can keep them in a file beside the server, while a deployment running several instances
// keeps them in a key vault the instances share. Everything above the store works the same either way.
type Backend interface {
	// Name identifies the backend in logs. It must not include a credential.
	Name() string
	// Load returns every secret the backend holds, keyed by name.
	Load(ctx context.Context) (map[string]Secret, error)
	// Put stores a secret, replacing any entry of the same name.
	Put(ctx context.Context, secret Secret) error
	// Delete removes a secret. Removing an absent name is not an error.
	Delete(ctx context.Context, name string) error
	// CacheTTL is how long the store may serve what Load returned before loading again.
	//
	// Zero means never: what was loaded stays authoritative until this process changes it, which is
	// correct only when nothing else can write to the backend. A shared backend must return a non-zero
	// TTL, because another instance's write is otherwise invisible here for the life of the process.
	CacheTTL() time.Duration
}

// Mode selects which backend a deployment's secrets live in.
type Mode string

const (
	// ModeDB keeps secrets in the configuration database, encrypted. The database is shared by every
	// instance of a deployment, so a credential set through one is usable by all of them, which a file
	// beside one instance cannot manage. It is what a Data Plane defaults to.
	ModeDB Mode = "db"
	// ModeFile keeps secrets in a JSON file beside the server.
	ModeFile Mode = "file"
	// ModeKV keeps secrets in an external key vault.
	ModeKV Mode = "kv"
	// ModeService reads from the standalone secret provider service, which owns its own storage. No
	// store is served in this mode, so there is no backend.
	ModeService Mode = "service"
)

// Config describes where a deployment keeps its secrets.
type Config struct {
	// Mode selects the backend. An empty mode disables the store entirely; which mode an unset one
	// should mean is the server's decision, not this package's, because it differs per plane.
	Mode Mode
	// FilePath backs ModeFile.
	FilePath string
	// KV backs ModeKV.
	KV KVConfig
	// DB backs ModeDB.
	DB DBConfig
}

// DBConfig is what the database-backed mode needs beyond the server's own database provider.
type DBConfig struct {
	// Provider opens the configuration database the secrets are stored in.
	Provider provider.DBProviderInterface
	// Sealer encrypts a value before it is stored. Required: the mode refuses to store plaintext.
	Sealer Sealer
	// DeploymentID scopes the rows when a caller carries no deployment of its own.
	DeploymentID string
}

// NewBackend builds the backend a configuration asks for.
//
// It returns a nil backend, and no error, for the modes that keep no store here: an empty mode, and
// ModeService, where the standalone service holds the secrets itself.
func NewBackend(cfg Config) (Backend, error) {
	switch cfg.Mode {
	case "":
		return nil, nil
	case ModeService:
		return nil, nil
	case ModeDB:
		return NewDBBackend(cfg.DB.Provider, cfg.DB.Sealer, cfg.DB.DeploymentID)
	case ModeFile:
		if strings.TrimSpace(cfg.FilePath) == "" {
			return nil, fmt.Errorf("secret store mode %q requires file.path", ModeFile)
		}
		return NewFileBackend(cfg.FilePath)
	case ModeKV:
		return NewKVBackend(cfg.KV)
	default:
		return nil, fmt.Errorf("unknown secret store mode %q, expected one of %s",
			cfg.Mode, strings.Join(modeNames(), ", "))
	}
}

// modeNames lists the configurable modes, for an error that has to say what was expected.
func modeNames() []string {
	names := []string{string(ModeDB), string(ModeFile), string(ModeKV), string(ModeService)}
	sort.Strings(names)
	return names
}
