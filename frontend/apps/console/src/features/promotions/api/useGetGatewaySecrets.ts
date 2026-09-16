// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {SecretList} from '../models/promotion';

/**
 * Lists every credential an gateway's configuration needs and whether its Data Plane holds it.
 *
 * The list comes from the configuration rather than from the secret service, so a credential that was
 * never captured still appears. That is the case worth seeing: a missing credential applies without
 * error and then rejects every attempt to use it.
 */
export default function useGetGatewaySecrets(gatewayId: string): UseQueryResult<SecretList> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();

  return useQuery<SecretList>({
    queryKey: [PromotionQueryKeys.SECRETS, gatewayId],
    enabled: Boolean(baseUrl) && Boolean(gatewayId),
    queryFn: async (): Promise<SecretList> => {
      const response: {data: SecretList} = await http.request({
        url: `${baseUrl}/gateways/${gatewayId}/secrets`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    retry: false,
    // Held status is read live from the Data Plane, so a credential set elsewhere shows up here without
    // a manual refresh.
    staleTime: 0,
    refetchOnMount: 'always',
  });
}
