// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import usePromotionServiceUrl from './usePromotionServiceUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {Diff} from '../models/promotion';

/**
 * Previews what promoting a source gateway's version into a target gateway would change.
 * This is the diff the user reviews, and selects from, before promoting.
 */
export default function useGetPromotionPreview(
  fromGateway: string,
  toGateway: string,
  version?: string,
): UseQueryResult<Diff> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = usePromotionServiceUrl();

  return useQuery<Diff>({
    queryKey: [PromotionQueryKeys.PROMOTION_PREVIEW, fromGateway, toGateway, version ?? 'latest'],
    enabled: Boolean(baseUrl) && Boolean(fromGateway) && Boolean(toGateway),
    queryFn: async (): Promise<Diff> => {
      const params = new URLSearchParams({fromGateway, toGateway});
      if (version) {
        params.set('version', version);
      }
      const response: {data: Diff} = await http.request({
        url: `${baseUrl}/promotions/preview?${params.toString()}`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
