// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {VersionListResponse} from '../models/promotion';

/**
 * Lists the organization's retained configuration versions, newest first.
 *
 * These belong to the organization, not to any gateway: the same version is what every gateway can
 * be moved onto.
 */
export default function useGetVersions(): UseQueryResult<VersionListResponse> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();

  return useQuery<VersionListResponse>({
    queryKey: [PromotionQueryKeys.VERSIONS],
    enabled: Boolean(baseUrl),
    queryFn: async (): Promise<VersionListResponse> => {
      const response: {data: VersionListResponse} = await http.request({
        url: `${baseUrl}/versions`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
