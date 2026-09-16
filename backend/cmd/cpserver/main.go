// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the ThunderID control plane.
package main

import (
	"github.com/thunder-id/thunderid/internal/controlplane"
	"github.com/thunder-id/thunderid/internal/server"
)

func main() {
	server.Run(controlplane.Plane())
}
