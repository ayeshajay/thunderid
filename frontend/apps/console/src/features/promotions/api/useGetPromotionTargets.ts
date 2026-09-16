// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import useGetGateways from './useGetGateways';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {Gateway} from '../models/promotion';

/** One gateway a version may be promoted into. */
export interface PromotionTarget {
  gatewayId: string;
  name: string;
}

interface TargetsResponse {
  targets: PromotionTarget[];
}

/**
 * Resolves where a gateway's configuration may be promoted to.
 *
 * On its own the Control Plane keeps gateways as a flat set and permits any pair, so every other
 * gateway is a target. With an environment manager connected, that service holds the organization's
 * hierarchy and is the only thing that knows which moves it allows, so it is asked: a gateway at the
 * top of the hierarchy has nowhere above it and gets an empty list, which is what stops a promote
 * being offered where it cannot happen.
 */
export default function useGetPromotionTargets(gatewayId: string): UseQueryResult<PromotionTarget[]> {
  const {http, config} = {...useThunderID(), ...useConfig()};
  const managerUrl: string = config.env_manager?.public_url?.trim().replace(/\/+$/, '') ?? '';
  const {data: gatewayData} = useGetGateways();

  return useQuery<PromotionTarget[]>({
    queryKey: [PromotionQueryKeys.PROMOTION_TARGETS, gatewayId, managerUrl],
    enabled: Boolean(gatewayId),
    queryFn: async (): Promise<PromotionTarget[]> => {
      if (!managerUrl) {
        // No hierarchy to consult: any other gateway of the organization is a target.
        return (gatewayData?.gateways ?? [])
          .filter((gateway: Gateway) => gateway.id !== gatewayId)
          .map((gateway: Gateway) => ({gatewayId: gateway.id, name: gateway.name}));
      }
      const response: {data: TargetsResponse} = await http.request({
        url: `${managerUrl}/promotions/targets?fromGateway=${encodeURIComponent(gatewayId)}`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data.targets ?? [];
    },
  });
}
