// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayVariableQueryKeys from '../constants/gateway-variable-query-keys';
import type {GatewayVariable} from '../models/gateway-variable';

export default function useGetGatewayVariable(gatewayId: string, id: string): UseQueryResult<GatewayVariable> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<GatewayVariable>({
    queryKey: [GatewayVariableQueryKeys.GATEWAY_VARIABLE, gatewayId, id],
    enabled: Boolean(gatewayId) && Boolean(id),
    queryFn: async (): Promise<GatewayVariable> => {
      const serverUrl: string = getServerUrl();
      const response: {data: GatewayVariable} = await http.request({
        url: `${serverUrl}/gateways/${gatewayId}/variables/${id}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
