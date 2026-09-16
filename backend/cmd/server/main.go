// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the ThunderID data plane.
package main

import (
	"github.com/thunder-id/thunderid/internal/dataplane"
	"github.com/thunder-id/thunderid/internal/server"
)

func main() {
	server.Run(dataplane.Plane())
}
