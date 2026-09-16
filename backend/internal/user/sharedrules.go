// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/resourcevalidation"
	"github.com/thunder-id/thunderid/internal/system/varname"
)

// sharedRules are the user rules that hold on any plane.
//
// A user is mostly its attributes, and which attributes a user may carry is decided by its user
// type, which each deployment holds its own. That is a question about the world, so it is not asked
// here: a control plane designing a user for a gateway cannot see that gateway's types, and the
// gateway checks them at import against the ones it has.
//
// What is left is the shape a user document has whatever it is for: it names a type, it carries
// attributes, and the attributes that have a known form have it.
type sharedRules struct{}

// ResourceType names the type these rules are for.
func (sharedRules) ResourceType() string { return resourceTypeUser }

// Validate applies the shared rules to a user payload.
func (sharedRules) Validate(_ context.Context, payload []byte) error {
	var document struct {
		Type       string                 `json:"type"`
		Attributes map[string]interface{} `json:"attributes"`
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return fmt.Errorf("the payload is not a user: %w", err)
	}

	if strings.TrimSpace(document.Type) == "" {
		return fmt.Errorf("a user type is required")
	}
	if len(document.Attributes) == 0 {
		return fmt.Errorf("a user needs attributes")
	}

	return checkEmail(document.Attributes)
}

// emailPattern is a deliberately loose check: an address either has the shape of one or it does not,
// and deciding whether it receives mail is not something any plane can do from the document.
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// checkEmail confirms an email attribute looks like an address, unless it stands for one a gateway
// supplies.
func checkEmail(attributes map[string]interface{}) error {
	value, present := attributes["email"]
	if !present {
		return nil
	}
	email, ok := value.(string)
	if !ok {
		return fmt.Errorf("email must be text")
	}
	if email == "" || varname.IsReference(email) {
		return nil
	}
	if !emailPattern.MatchString(email) {
		return fmt.Errorf("email is not a valid address")
	}
	return nil
}

// registerSharedRules plugs this resource type's plane-independent rules into the registry.
func registerSharedRules() error {
	return resourcevalidation.Register(sharedRules{})
}
