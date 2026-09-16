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

package thunder

import (
	"encoding/json"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/export"
)

// The export API's field names are its contract, and this client has to decode exactly what that
// API encodes. Decoding a response built by the server type is what catches a rename on either side:
// a mismatch decodes silently to an empty string, which would leave every captured version with no
// variable values and report every placeholder as missing at apply.
func TestExportResponseDecodesWhatTheExportAPIEncodes(t *testing.T) {
	encoded, err := json.Marshal(export.JSONExportResponse{
		Resources:            "resource_type: application",
		EnvironmentVariables: "APPLICATION_TEST_CLIENT_ID=abc\n",
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var got jsonExportResponse
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Resources != "resource_type: application" {
		t.Fatalf("resources did not decode, got %q", got.Resources)
	}
	if got.EnvironmentVariables != "APPLICATION_TEST_CLIENT_ID=abc\n" {
		t.Fatalf("the .env body did not decode, got %q: the field names have drifted apart",
			got.EnvironmentVariables)
	}
}
