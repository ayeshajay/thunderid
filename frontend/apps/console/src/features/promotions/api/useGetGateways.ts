// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {GatewayListResponse} from '../models/promotion';

/**
 * Lists the organization's gateways, ordered by name.
 */
export default function useGetGateways(): UseQueryResult<GatewayListResponse> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();

  return useQuery<GatewayListResponse>({
    queryKey: [PromotionQueryKeys.GATEWAYS],
    enabled: Boolean(baseUrl),
    queryFn: async (): Promise<GatewayListResponse> => {
      const response: {data: GatewayListResponse} = await http.request({
        url: `${baseUrl}/gateways`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
