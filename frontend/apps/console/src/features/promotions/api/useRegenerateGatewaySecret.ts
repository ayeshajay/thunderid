// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useGatewayApiUrl from './useGatewayApiUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {RegeneratedSecret} from '../models/promotion';

/** Variables for regenerating one credential. */
export interface RegenerateSecretVariables {
  gatewayId: string;
  name: string;
}

/**
 * Issues a fresh credential and stores it on the gateway's Data Plane.
 *
 * Only a hashed credential can be regenerated, because a value the Data Plane replays to a third party
 * is issued by that party. The response is the one and only time the new value can be read.
 */
export default function useRegenerateGatewaySecret(): UseMutationResult<
  RegeneratedSecret,
  Error,
  RegenerateSecretVariables
> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useGatewayApiUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<RegeneratedSecret, Error, RegenerateSecretVariables>({
    mutationFn: async (variables: RegenerateSecretVariables): Promise<RegeneratedSecret> => {
      const url = `${baseUrl}/gateways/${variables.gatewayId}/secrets/${encodeURIComponent(variables.name)}/regenerate`;
      const response: {data: RegeneratedSecret} = await http.request({
        url,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data: RegeneratedSecret, variables: RegenerateSecretVariables) => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.SECRETS, variables.gatewayId]}).catch(() => {
        // Ignore invalidation errors.
      });
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.VARIABLES, variables.gatewayId]}).catch(() => {
        // Ignore invalidation errors.
      });
    },
    onError: (error: Error) => {
      showToast(error.message || t('secrets.regenerateError', 'Failed to regenerate the secret'), 'error');
    },
  });
}
