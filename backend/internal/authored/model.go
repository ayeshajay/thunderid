// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authored holds configuration a control plane has designed but not run.
//
// This is a separate stack from the resource services, on purpose. A control plane designs
// configuration to be applied to gateways: what it holds is a payload with references in it, not a
// working resource, and it has no runtime that could use one. Keeping it in its own store means a
// control plane plugs in this and not the resource services, rather than running the whole product
// in order to hold a document.
//
// Nothing here models a resource. The payload is stored as it was authored, and the service that
// does it knows only that the document is well formed and that its references are ones a gateway
// could resolve. What the document means is the resource type's business, and it is settled on the
// gateway, at import, against real values.
package authored

// Resource is one authored configuration document.
type Resource struct {
	ID string `json:"id,omitempty"`
	// ResourceType is what the payload describes, as the declarative resources name it. It is
	// carried beside the payload rather than read out of it, so a listing does not have to parse
	// every document to know what is in it.
	ResourceType string `json:"resourceType"`
	// Name identifies the document within its type. A second document of the same type and name
	// replaces the first: authoring the same resource twice is an edit, not a duplicate.
	Name string `json:"name"`
	// Payload is the document exactly as authored, references included.
	Payload   string `json:"payload"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Summary is how an authored resource is listed. The payload is absent: a listing says what has been
// authored, and a document runs to more than a listing should carry.
type Summary struct {
	ID           string `json:"id"`
	ResourceType string `json:"resourceType"`
	Name         string `json:"name"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// summarize drops the payload from a stored resource.
func summarize(r Resource) Summary {
	return Summary{
		ID: r.ID, ResourceType: r.ResourceType, Name: r.Name,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
