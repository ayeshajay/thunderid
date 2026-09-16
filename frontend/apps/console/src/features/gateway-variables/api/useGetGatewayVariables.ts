// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayVariableQueryKeys from '../constants/gateway-variable-query-keys';
import type {GatewayVariableListParams, GatewayVariableListResponse} from '../models/gateway-variable';

export default function useGetGatewayVariables(
  gatewayId: string,
  params?: GatewayVariableListParams,
): UseQueryResult<GatewayVariableListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<GatewayVariableListResponse>({
    queryKey: [GatewayVariableQueryKeys.GATEWAY_VARIABLES, gatewayId, {limit, offset}],
    enabled: Boolean(gatewayId),
    queryFn: async (): Promise<GatewayVariableListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams = new URLSearchParams({limit: limit.toString(), offset: offset.toString()});
      const response: {
        data: GatewayVariableListResponse;
      } = await http.request({
        url: `${serverUrl}/gateways/${gatewayId}/variables?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
