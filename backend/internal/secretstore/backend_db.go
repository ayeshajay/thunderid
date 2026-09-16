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
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// Sealer encrypts a credential for storage and reads it back.
//
// It is an interface so the store does not depend on how the server manages keys, and so a test can
// exercise the storage without one. The database holds only what Seal returns.
type Sealer interface {
	Seal(ctx context.Context, plaintext []byte) ([]byte, error)
	Open(ctx context.Context, sealed []byte) ([]byte, error)
}

// dbBackend keeps secrets in the configuration database, encrypted.
//
// The database is shared by every instance of a deployment, so a credential set through one is
// immediately usable by the others. That is what makes it the right default: a file backing store
// belongs to one instance, and a deployment that grows a second one silently stops agreeing with
// itself about what it holds.
type dbBackend struct {
	dbProvider   provider.DBProviderInterface
	sealer       Sealer
	deploymentID string
	// cacheTTL is how long the store above may serve a load. Non-zero because another instance writing
	// to the same rows is otherwise invisible here for the life of the process.
	cacheTTL time.Duration
}

// dbBackendCacheTTL bounds how stale a cached read may be. A credential is read on every
// authentication, so reloading per read would put a query in that path; a short window keeps the cost
// down while making another instance's write visible quickly.
const dbBackendCacheTTL = 30 * time.Second

// NewDBBackend builds a backend over the configuration database.
//
// The sealer is required: storing a credential in the clear is refused rather than done quietly,
// because a database dump would then carry every credential the deployment holds.
func NewDBBackend(dbProvider provider.DBProviderInterface, sealer Sealer,
	deploymentID string) (Backend, error) {
	// The sealer is checked first, and on its own: refusing to store plaintext is the invariant here,
	// and it should not depend on having got as far as a database.
	if sealer == nil {
		return nil, fmt.Errorf("a database-backed secret store needs a sealer, so nothing is stored in the clear")
	}
	if dbProvider == nil {
		return nil, fmt.Errorf("a database-backed secret store needs a database provider")
	}
	return &dbBackend{
		dbProvider:   dbProvider,
		sealer:       sealer,
		deploymentID: deploymentID,
		cacheTTL:     dbBackendCacheTTL,
	}, nil
}

func (b *dbBackend) Name() string { return "db" }

// CacheTTL bounds how long a load may be served, because other instances write here too.
func (b *dbBackend) CacheTTL() time.Duration { return b.cacheTTL }

// Load reads and decrypts every secret this deployment holds.
//
// An entry that cannot be decrypted is left out rather than failing the whole load: one unreadable
// row would otherwise take every other credential down with it, and the store's own missing-secret
// handling already reports the one that cannot be resolved.
func (b *dbBackend) Load(ctx context.Context) (map[string]Secret, error) {
	dbClient, err := b.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	rows, err := dbClient.QueryContext(ctx, queryListSecrets, b.resolveDeployment(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets: %w", err)
	}

	secrets := make(map[string]Secret, len(rows))
	for _, row := range rows {
		secret, err := b.rowToSecret(ctx, row)
		if err != nil {
			continue
		}
		secrets[secret.Name] = secret
	}
	return secrets, nil
}

// Put stores a secret, encrypting the value first.
func (b *dbBackend) Put(ctx context.Context, secret Secret) error {
	dbClient, err := b.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	sealed, err := b.sealer.Seal(ctx, []byte(secret.Value))
	if err != nil {
		return fmt.Errorf("failed to encrypt the secret: %w", err)
	}

	parameters := ""
	if secret.Kind == KindHash {
		encoded, err := json.Marshal(secret.Parameters)
		if err != nil {
			return fmt.Errorf("failed to encode the hash parameters: %w", err)
		}
		parameters = string(encoded)
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return fmt.Errorf("failed to generate a secret id: %w", err)
	}

	_, err = dbClient.QueryContext(ctx, queryUpsertSecret, id, secret.Name, string(secret.Kind),
		string(sealed), secret.Algorithm, parameters, secret.Description, b.resolveDeployment(ctx))
	if err != nil {
		return fmt.Errorf("failed to store the secret: %w", err)
	}
	return nil
}

// Delete removes a secret. Removing an absent name is not an error.
func (b *dbBackend) Delete(ctx context.Context, name string) error {
	dbClient, err := b.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteSecret, name, b.resolveDeployment(ctx)); err != nil {
		return fmt.Errorf("failed to delete the secret: %w", err)
	}
	return nil
}

// rowToSecret decodes one row, decrypting the value.
func (b *dbBackend) rowToSecret(ctx context.Context, row map[string]interface{}) (Secret, error) {
	secret := Secret{
		Name:        stringField(row, "name"),
		Kind:        Kind(stringField(row, "kind")),
		Algorithm:   stringField(row, "algorithm"),
		Description: stringField(row, "description"),
	}

	opened, err := b.sealer.Open(ctx, []byte(stringField(row, "value")))
	if err != nil {
		return Secret{}, fmt.Errorf("failed to decrypt the secret %q: %w", secret.Name, err)
	}
	secret.Value = string(opened)

	if raw := stringField(row, "parameters"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &secret.Parameters); err != nil {
			return Secret{}, fmt.Errorf("failed to decode the hash parameters of %q: %w", secret.Name, err)
		}
	}
	return secret, nil
}

// resolveDeployment is the deployment the request belongs to, falling back to the one this backend was
// built for when a caller carries none.
func (b *dbBackend) resolveDeployment(ctx context.Context) string {
	return deployment.ResolveOr(ctx, b.deploymentID)
}

// stringField reads a column as a string, tolerating the driver returning bytes.
func stringField(row map[string]interface{}, column string) string {
	switch v := row[column].(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
