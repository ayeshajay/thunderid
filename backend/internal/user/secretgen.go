// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/secretgen"
)

// A generated password has to survive being typed, read aloud and pasted, which a client secret does
// not. So it is built from characters rather than from raw bytes, and from a set that avoids the
// pairs people confuse: no O and 0, no l, I and 1.
const (
	passwordLength = 20

	passwordUpper  = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	passwordLower  = "abcdefghijkmnopqrstuvwxyz"
	passwordDigits = "23456789"
	passwordSymbol = "!@#$%^&*-_=+"
)

// passwordGenerator makes a user's password.
//
// It differs from a client secret in what it is for. A client secret is read by a machine once and
// stored, so it can be any opaque string. A password is held by a person and checked against
// whatever rules the deployment applies to one, so it is generated to satisfy the usual four classes
// rather than as raw entropy.
type passwordGenerator struct{}

// ResourceType names the type this generator serves.
func (passwordGenerator) ResourceType() string { return resourceTypeUser }

// Generate returns a new password, with at least one character from each class and the rest drawn
// from all of them.
//
// It does not consult the deployment's password policy, because a policy is a property of the
// deployment a user ends up on and this may run on a plane that is not it. A gateway that refuses
// the result will say so, which is a better failure than quietly generating something it would
// reject.
func (passwordGenerator) Generate(context.Context) (string, error) {
	classes := []string{passwordUpper, passwordLower, passwordDigits, passwordSymbol}
	all := strings.Join(classes, "")

	password := make([]byte, 0, passwordLength)
	for _, class := range classes {
		c, err := pick(class)
		if err != nil {
			return "", err
		}
		password = append(password, c)
	}
	for len(password) < passwordLength {
		c, err := pick(all)
		if err != nil {
			return "", err
		}
		password = append(password, c)
	}

	// The first characters are one per class, in order, so without this the shape of every password
	// would be the same and the first four would each be drawn from a quarter of the alphabet.
	if err := shuffle(password); err != nil {
		return "", err
	}
	return string(password), nil
}

// pick returns one character from the set, chosen uniformly.
func pick(set string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, fmt.Errorf("failed to generate a password: %w", err)
	}
	return set[n.Int64()], nil
}

// shuffle reorders the password in place, so the class each position came from is not fixed.
func shuffle(password []byte) error {
	for i := len(password) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("failed to generate a password: %w", err)
		}
		j := n.Int64()
		password[i], password[j] = password[j], password[i]
	}
	return nil
}

// registerSecretGenerator plugs this resource type's credential rule into the registry.
func registerSecretGenerator() error {
	return secretgen.Register(passwordGenerator{})
}
