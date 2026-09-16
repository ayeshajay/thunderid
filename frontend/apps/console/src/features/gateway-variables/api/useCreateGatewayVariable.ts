// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GatewayVariableQueryKeys from '../constants/gateway-variable-query-keys';
import type {CreateGatewayVariableRequest, GatewayVariable} from '../models/gateway-variable';

export default function useCreateGatewayVariable(
  gatewayId: string,
): UseMutationResult<GatewayVariable, Error, CreateGatewayVariableRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('gatewayVariables');
  const {showToast} = useToast();

  return useMutation<GatewayVariable, Error, CreateGatewayVariableRequest>({
    mutationFn: async (data: CreateGatewayVariableRequest): Promise<GatewayVariable> => {
      const serverUrl: string = getServerUrl();
      const response: {data: GatewayVariable} = await http.request({
        url: `${serverUrl}/gateways/${gatewayId}/variables`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [GatewayVariableQueryKeys.GATEWAY_VARIABLES, gatewayId]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('create.success', 'Gateway variable created successfully'), 'success');
    },
    onError: () => {
      showToast(t('create.error', 'Failed to create the gateway variable. Please try again.'), 'error');
    },
  });
}
