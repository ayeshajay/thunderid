// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A non-secret value substituted into declarative configuration when it is applied to a Data Plane,
 * such as an application's redirect URLs. Unlike a secret, the value is readable.
 */
export interface GatewayVariable {
  id: string;
  key: string;
  value: string;
  description?: string;
  createdAt: string;
  updatedAt: string;
}

export interface GatewayVariableListResponse {
  totalResults: number;
  count: number;
  gatewayVariables: GatewayVariable[];
}

export interface CreateGatewayVariableRequest {
  key: string;
  value: string;
  description?: string;
}

export interface UpdateGatewayVariableRequest {
  value: string;
  description?: string;
}

export interface UpdateGatewayVariableVariables {
  id: string;
  data: UpdateGatewayVariableRequest;
}

export interface GatewayVariableListParams {
  limit?: number;
  offset?: number;
}
