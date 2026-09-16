// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormLabel,
  PageContent,
  PageTitle,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useGetGatewayVariable from '../api/useGetGatewayVariable';
import useUpdateGatewayVariable from '../api/useUpdateGatewayVariable';
import GatewayVariableDeleteDialog from '../components/GatewayVariableDeleteDialog';

/**
 * Page for editing an gateway variable's value or description. The key is immutable, because it is
 * the placeholder that configuration references.
 */
export default function GatewayVariableEditPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {gatewayId = '', gatewayVariableId = ''} = useParams<{gatewayId: string; gatewayVariableId: string}>();

  const {data, isLoading, error} = useGetGatewayVariable(gatewayId, gatewayVariableId);
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);

  if (isLoading) {
    return (
      <PageContent>
        <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent>
        <Alert severity="error">
          {error.message || t('gatewayVariables:edit.error', 'Failed to load the gateway variable')}
        </Alert>
      </PageContent>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{data?.key ?? t('gatewayVariables:edit.title', 'Gateway Variable')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('gatewayVariables:edit.subtitle', 'Update the value applied to your Data Planes')}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            color="error"
            onClick={() => {
              setDeleteOpen(true);
            }}
          >
            {t('common:actions.delete', 'Delete')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {data && <GatewayVariableForm gatewayId={gatewayId} gatewayVariableId={gatewayVariableId} initial={data} />}

      <GatewayVariableDeleteDialog
        open={deleteOpen}
        gatewayVariableId={gatewayVariableId}
        onClose={() => {
          setDeleteOpen(false);
          void navigate(`/promotions/${gatewayId}/variables`);
        }}
      />
    </PageContent>
  );
}

/**
 * The edit form, seeded from the loaded variable.
 *
 * It is a separate component so its state initializes from props on mount, once the variable is
 * available. Seeding state from an effect instead would re-render on every load and would risk
 * discarding the user's edits on a background refetch.
 */
function GatewayVariableForm({
  gatewayId,
  gatewayVariableId,
  initial,
}: {
  gatewayId: string;
  gatewayVariableId: string;
  initial: {value: string; description?: string};
}): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const updateGatewayVariable = useUpdateGatewayVariable(gatewayId);

  const [value, setValue] = useState<string>(initial.value);
  const [description, setDescription] = useState<string>(initial.description ?? '');

  return (
    <Stack spacing={3} sx={{maxWidth: 640}}>
      <FormControl fullWidth>
        <FormLabel>{t('gatewayVariables:form.value.label', 'Value')}</FormLabel>
        <TextField
          value={value}
          onChange={(event) => {
            setValue(event.target.value);
          }}
          helperText={t(
            'gatewayVariables:form.value.help',
            'For a list, use a JSON array such as ["https://app.example.com/callback"].',
          )}
          fullWidth
        />
      </FormControl>

      <FormControl fullWidth>
        <FormLabel>{t('gatewayVariables:form.description.label', 'Description')}</FormLabel>
        <TextField
          value={description}
          onChange={(event) => {
            setDescription(event.target.value);
          }}
          fullWidth
        />
      </FormControl>

      <Stack direction="row" spacing={2} justifyContent="flex-end">
        <Button
          onClick={() => {
            void navigate(`/promotions/${gatewayId}/variables`);
          }}
        >
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          onClick={() => {
            updateGatewayVariable.mutate({id: gatewayVariableId, data: {value, description}});
          }}
          disabled={updateGatewayVariable.isPending || value.trim() === ''}
        >
          {updateGatewayVariable.isPending ? t('common:status.saving', 'Saving...') : t('common:actions.save', 'Save')}
        </Button>
      </Stack>
    </Stack>
  );
}
