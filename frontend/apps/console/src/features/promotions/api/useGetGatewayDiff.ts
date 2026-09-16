// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useGatewayApiUrl from './useGatewayApiUrl';
import type {Diff} from '../models/promotion';

/**
 * Reads the diff between two versions of one gateway.
 *
 * The apply flow uses it with the defaults, applied to the version being applied, so the change can
 * be reviewed before it reaches a data plane rather than being read from the result afterwards.
 * Resources held back from the gateway are filtered out server side, so the preview shows what
 * the apply would actually do.
 */
export default function useGetGatewayDiff(
  gatewayId: string,
  from = 'applied',
  to = 'latest',
): UseQueryResult<Diff, Error> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();

  return useQuery<Diff, Error>({
    queryKey: ['gateway-diff', gatewayId, from, to],
    enabled: Boolean(gatewayId) && Boolean(baseUrl),
    queryFn: async (): Promise<Diff> => {
      const response = await http.request({
        url: `${baseUrl}/gateways/${gatewayId}/diff?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
        method: 'GET',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);

      return (response as {data: Diff}).data;
    },
  });
}
