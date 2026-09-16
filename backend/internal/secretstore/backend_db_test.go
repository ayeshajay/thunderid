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
	"strings"
	"testing"
)

// reversingSealer stands in for the server's configuration crypto. Reversing is not encryption, but it
// is enough to show that what reaches the database is not the plaintext and that Load undoes whatever
// Put did.
type reversingSealer struct{}

func (reversingSealer) Seal(_ context.Context, plaintext []byte) ([]byte, error) {
	return reverse(plaintext), nil
}

func (reversingSealer) Open(_ context.Context, sealed []byte) ([]byte, error) {
	return reverse(sealed), nil
}

func reverse(in []byte) []byte {
	out := make([]byte, len(in))
	for i, b := range in {
		out[len(in)-1-i] = b
	}
	return out
}

// A credential reaches the database only as ciphertext, and comes back as itself.
func TestTheDatabaseNeverSeesTheCredential(t *testing.T) {
	const value = "the-client-secret"
	db := newFakeDB()
	backend, err := NewDBBackend(db, reversingSealer{}, "acme")
	if err != nil {
		t.Fatalf("build backend: %v", err)
	}

	err = backend.Put(context.Background(), Secret{
		Name: "APP_A_CLIENT_SECRET", Kind: KindValue, Value: value,
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	stored := db.storedValue("APP_A_CLIENT_SECRET")
	if stored == "" {
		t.Fatal("expected the secret to have been written")
	}
	if strings.Contains(stored, value) {
		t.Fatalf("the credential must not reach the database in the clear, got %q", stored)
	}

	loaded, err := backend.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := loaded["APP_A_CLIENT_SECRET"]
	if !ok {
		t.Fatalf("expected the secret to load back, got %v", loaded)
	}
	if got.Value != value {
		t.Fatalf("expected the credential to load back as itself, got %q", got.Value)
	}
}

// testHashAlgorithm names the hashing used in these tests.
const testHashAlgorithm = "PBKDF2"

// A hash keeps the parameters a verifier needs across a store and load.
func TestAHashKeepsWhatVerifyingItNeeds(t *testing.T) {
	db := newFakeDB()
	backend, _ := NewDBBackend(db, reversingSealer{}, "acme")

	err := backend.Put(context.Background(), Secret{
		Name: "PASSWORD", Kind: KindHash, Value: "the-hash", Algorithm: testHashAlgorithm,
		Parameters: HashParameters{Salt: "s", Iterations: 1000, KeySize: 32},
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	loaded, err := backend.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := loaded["PASSWORD"]
	if got.Algorithm != testHashAlgorithm || got.Parameters.Iterations != 1000 || got.Parameters.Salt != "s" {
		t.Fatalf("expected the hash parameters to survive, got %+v", got)
	}
}

// A row that cannot be decrypted is left out rather than failing every other credential with it.
func TestAnUnreadableRowDoesNotTakeTheOthersDown(t *testing.T) {
	db := newFakeDB()
	backend, _ := NewDBBackend(db, failingOpenSealer{}, "acme")
	_ = backend.Put(context.Background(), Secret{Name: "A", Kind: KindValue, Value: "a"})

	loaded, err := backend.Load(context.Background())

	if err != nil {
		t.Fatalf("an unreadable row should not fail the load: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected the unreadable row to be skipped, got %v", loaded)
	}
}

// failingOpenSealer seals but cannot open, standing in for a row encrypted under a key this process
// no longer has.
type failingOpenSealer struct{}

func (failingOpenSealer) Seal(_ context.Context, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (failingOpenSealer) Open(context.Context, []byte) ([]byte, error) {
	return nil, context.DeadlineExceeded
}
