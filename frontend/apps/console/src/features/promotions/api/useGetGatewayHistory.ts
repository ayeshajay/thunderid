// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {GatewayHistoryResponse} from '../models/promotion';

/**
 * Lists what a gateway has run, newest first.
 *
 * This is the gateway's own record, as distinct from the organization's versions: the versions that
 * actually reached this data plane, and the states it can be returned to.
 */
export default function useGetGatewayHistory(gatewayId: string): UseQueryResult<GatewayHistoryResponse> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();

  return useQuery<GatewayHistoryResponse>({
    queryKey: [PromotionQueryKeys.HISTORY, gatewayId],
    enabled: Boolean(baseUrl) && Boolean(gatewayId),
    queryFn: async (): Promise<GatewayHistoryResponse> => {
      const response: {data: GatewayHistoryResponse} = await http.request({
        url: `${baseUrl}/gateways/${gatewayId}/history`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
