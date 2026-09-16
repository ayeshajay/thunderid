// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';

/**
 * Removes an gateway and its stored configuration versions from the gateway manager. The
 * data planes themselves are left untouched.
 */
export default function useDeleteGateway(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (gatewayId: string): Promise<void> => {
      await http.request({
        url: `${baseUrl}/gateways/${gatewayId}`,
        method: 'DELETE',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_result: void, gatewayId: string) => {
      queryClient.removeQueries({queryKey: [PromotionQueryKeys.VERSIONS, gatewayId]});
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.GATEWAYS]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('gateway.deleteSuccess', 'Gateway removed'), 'success');
    },
    onError: () => {
      showToast(t('gateway.deleteError', 'Failed to remove the gateway. Please try again.'), 'error');
    },
  });
}
