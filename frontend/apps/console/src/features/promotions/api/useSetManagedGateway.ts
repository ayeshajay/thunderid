// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {Gateway} from '../models/promotion';

/**
 * Moves the mark for the gateway the Control Plane administers directly.
 *
 * Exactly one gateway of an organization holds it, so this moves the mark rather than toggling
 * it: an organization is never left without one, which would strand every credential created
 * afterwards.
 */
export default function useSetManagedGateway(): UseMutationResult<Gateway, Error, string> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<Gateway, Error, string>({
    mutationFn: async (gatewayId: string): Promise<Gateway> => {
      const response: {data: Gateway} = await http.request({
        url: `${baseUrl}/gateways/${gatewayId}/managed`,
        method: 'POST',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (result: Gateway) => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.GATEWAYS]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('managed.success', '{{name}} is now managed by the Control Plane', {name: result.name}), 'success');
    },
    onError: () => {
      showToast(t('managed.error', 'Failed to change the managed gateway. Please try again.'), 'error');
    },
  });
}
