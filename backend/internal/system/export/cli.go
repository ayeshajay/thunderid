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

package export

import (
	"context"
	"flag"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// Subcommand is the first positional argument that selects the in-process export one-shot instead of
// starting the long-running server.
const Subcommand = "export"

// IsInvocation reports whether the process was started as the export one-shot
// (e.g. `cpserver export --deployment-id <tenant> --out <dir>`).
func IsInvocation(firstArg string) bool {
	return firstArg == Subcommand
}

// RunCLI runs the export in-process for the requested deployment and writes the declarative bundle to
// the output directory.
//
// On a token-mode multi-tenant Control Plane, --deployment-id selects whose configuration is
// exported; the export reads through the same tenant-scoped stores, so the bundle holds exactly that
// deployment's resources. An empty --deployment-id exports the server-configured one.
//
// Tearing the server down afterwards is the caller's, because the caller is what started it.
func RunCLI(ctx context.Context, logger *log.Logger, exportSvc ExportServiceInterface, args []string) error {
	fs := flag.NewFlagSet(Subcommand, flag.ContinueOnError)
	deploymentID := fs.String("deployment-id", "", "Deployment id whose configuration to export")
	outDir := fs.String("out", "", "Directory to write the exported declarative bundle to")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *outDir == "" {
		return fmt.Errorf("no output directory supplied: set --out <dir>")
	}

	// Scope the export to the requested deployment so the stores resolve its id.
	if *deploymentID != "" {
		ctx = deployment.WithID(ctx, *deploymentID)
	}

	response, svcErr := exportSvc.ExportResources(ctx, EverythingRequest())
	if svcErr != nil {
		return fmt.Errorf("export failed [%s]: %s", svcErr.Code, svcErr.Error.DefaultValue)
	}
	if err := WriteBundle(*outDir, response); err != nil {
		return err
	}

	logger.Info(ctx, "In-process export completed",
		log.String("deploymentId", *deploymentID),
		log.String("out", *outDir),
		log.Int("files", len(response.Files)))
	return nil
}

// EverythingRequest asks for every resource type, dependencies included, as YAML.
func EverythingRequest() *ExportRequest {
	return &ExportRequest{
		Applications:      []string{"*"},
		Connections:       []string{"*"},
		UserTypes:         []string{"*"},
		OrganizationUnits: []string{"*"},
		Users:             []string{"*"},
		Groups:            []string{"*"},
		ResourceServers:   []string{"*"},
		Roles:             []string{"*"},
		Flows:             []string{"*"},
		Translations:      []string{"*"},
		Layouts:           []string{"*"},
		Themes:            []string{"*"},
		ServerConfigs:     []string{"*"},
		Options: &ExportOptions{
			IncludeDependencies: true,
			Format:              "yaml",
		},
	}
}
