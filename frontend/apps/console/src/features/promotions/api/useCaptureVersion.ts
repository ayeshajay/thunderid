// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {Version} from '../models/promotion';

/** Variables for capturing the organization's configuration as a version. */
export interface CaptureVersionVariables {
  note?: string;
}

/**
 * Captures the organization's current configuration as a new version.
 *
 * It names no gateway. A version belongs to the organization, and a gateway receives one by having
 * it applied rather than by producing one of its own.
 */
export default function useCaptureVersion(): UseMutationResult<Version, Error, CaptureVersionVariables> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<Version, Error, CaptureVersionVariables>({
    mutationFn: async (variables: CaptureVersionVariables): Promise<Version> => {
      const response: {data: Version} = await http.request({
        url: `${baseUrl}/versions`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        credentials: 'same-origin',
        data: {mode: 'capture', note: variables.note},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.GATEWAYS]}).catch(() => {
        // Ignore invalidation errors.
      });
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.VERSIONS]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('capture.success', 'Configuration captured as a new version'), 'success');
    },
    onError: () => {
      showToast(t('capture.error', 'Failed to capture the configuration. Please try again.'), 'error');
    },
  });
}
