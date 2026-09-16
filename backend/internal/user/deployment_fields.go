// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/resourcevalidation"
	"github.com/thunder-id/thunderid/internal/system/varname"
)

// documentFieldPassword is the credential a user document carries, spelled as the document spells
// it. That spelling is what an export derives a variable name from.
const documentFieldPassword = "password"

// deploymentFields declares which part of a user belongs to the deployment rather than to the user
// being designed.
//
// A password is the clearest case in the product. It is never carried between deployments even in
// principle: it is kept as a one way hash, so there is nothing to copy, and each deployment's copy of
// a user has its own. Authoring one would be authoring a value that cannot travel.
//
// Only the password. A passkey and the other device bound credentials mean nothing on another
// deployment at all, so they are not authored and not referenced either.
type deploymentFields struct{}

// ResourceType names the type these fields belong to.
func (deploymentFields) ResourceType() string { return resourceTypeUser }

// Fill replaces a user's password with a reference to the one its gateway holds.
//
// Whether a user has a password at all is the author's to decide, and the document says so by
// carrying a credentials block: a user authored without one signs in some other way, and no password
// is asked of any gateway it is applied to. A user authored with one needs a password wherever it
// lands, and this fills in the reference that says so.
//
// The author says "give this user a password" by naming the field and leaving it empty, or by
// leaving the field out of a credentials block they wrote. Both mean the same thing, because the one
// thing they cannot do is choose the value.
//
// An actual value is refused rather than overwritten, for the same reason as anywhere else:
// accepting it and discarding it would tell the caller their password was stored when it was not.
// A reference already there is left alone, so a document read back and written again round trips.
func (deploymentFields) Fill(_ context.Context, name string, document map[string]interface{}) error {
	credentials, ok := document["credentials"].(map[string]interface{})
	if !ok {
		// A user authored without credentials is signing in some other way, or not yet at all.
		return nil
	}

	if supplied, present := credentials[documentFieldPassword]; present && !asksForOne(supplied) {
		password, isText := supplied.(string)
		if !isText || !varname.IsReference(password) {
			return fmt.Errorf(
				"a password is not accepted here: it belongs to the gateway this user is applied " +
					"to, and a reference is stored in its place")
		}
		return nil
	}

	credentials[documentFieldPassword] = varname.SecretReference(
		varname.DeriveVariableName(resourceTypeUser, name, documentFieldPassword))
	return nil
}

// asksForOne reports whether a supplied password is empty, which asks for a password without
// choosing one. An author writing the field out by hand reaches for null or the empty string, and
// neither is a value anyone could have meant to keep.
func asksForOne(supplied interface{}) bool {
	if supplied == nil {
		return true
	}
	text, isText := supplied.(string)
	return isText && text == ""
}

// registerDeploymentFields declares which of this type's fields belong to the deployment.
func registerDeploymentFields() error {
	return resourcevalidation.RegisterDeploymentFields(deploymentFields{})
}
