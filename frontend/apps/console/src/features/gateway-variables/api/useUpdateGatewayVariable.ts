// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GatewayVariableQueryKeys from '../constants/gateway-variable-query-keys';
import type {GatewayVariable, UpdateGatewayVariableVariables} from '../models/gateway-variable';

export default function useUpdateGatewayVariable(
  gatewayId: string,
): UseMutationResult<GatewayVariable, Error, UpdateGatewayVariableVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('gatewayVariables');
  const {showToast} = useToast();

  return useMutation<GatewayVariable, Error, UpdateGatewayVariableVariables>({
    mutationFn: async (variables: UpdateGatewayVariableVariables): Promise<GatewayVariable> => {
      const serverUrl: string = getServerUrl();
      const response: {data: GatewayVariable} = await http.request({
        url: `${serverUrl}/gateways/${gatewayId}/variables/${variables.id}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: variables.data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_result: GatewayVariable, variables: UpdateGatewayVariableVariables) => {
      queryClient
        .invalidateQueries({queryKey: [GatewayVariableQueryKeys.GATEWAY_VARIABLE, gatewayId, variables.id]})
        .catch(() => {
          // Ignore invalidation errors.
        });
      queryClient.invalidateQueries({queryKey: [GatewayVariableQueryKeys.GATEWAY_VARIABLES, gatewayId]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('update.success', 'Gateway variable updated successfully'), 'success');
    },
    onError: () => {
      showToast(t('update.error', 'Failed to update the gateway variable. Please try again.'), 'error');
    },
  });
}
