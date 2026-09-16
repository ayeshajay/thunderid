// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Button, PageContent, PageTitle, Stack} from '@wso2/oxygen-ui';
import {Plus, Upload} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useApplyAll from '../../promotions/api/useApplyAll';
import GatewayVariablesList from '../components/GatewayVariablesList';

/**
 * The gateway variables with their actions.
 *
 * Rendered on its own page and also inside the gateway it belongs to, which is where it is normally
 * reached: a variable has no meaning apart from the gateway that resolves it. Embedded, it drops the
 * page furniture and carries its actions above the list instead.
 */
export default function GatewayVariablesListPage({embedded = false}: {embedded?: boolean}): JSX.Element {
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const applyAll = useApplyAll();

  const actions: JSX.Element = (
    <>
      <Button
        startIcon={<Upload size={18} />}
        disabled={applyAll.isPending}
        onClick={() => {
          applyAll.mutate();
        }}
      >
        {applyAll.isPending
          ? t('promotions:applyAll.inProgress', 'Applying...')
          : t('promotions:applyAll.action', 'Apply to Data Planes')}
      </Button>
      <Button
        variant="contained"
        startIcon={<Plus size={18} />}
        onClick={() => {
          void navigate(`/promotions/${gatewayId}/variables/create`);
        }}
      >
        {t('gatewayVariables:listing.add', 'Add Variable')}
      </Button>
    </>
  );

  const notice: JSX.Element = (
    <Alert severity="info" sx={{mb: 2}}>
      {t(
        'gatewayVariables:applyNotice',
        'A change here reaches a Data Plane only when configuration is applied. Use Apply to Data Planes to push it now.',
      )}
    </Alert>
  );

  if (embedded) {
    return (
      <>
        <Stack direction="row" spacing={1} justifyContent="flex-end" sx={{mb: 2}}>
          {actions}
        </Stack>
        {notice}
        <GatewayVariablesList />
      </>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('gatewayVariables:listing.title', 'Gateway Variables')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'gatewayVariables:listing.subtitle',
            'Non-secret values substituted into configuration when it is applied to a Data Plane',
          )}
        </PageTitle.SubHeader>
        <PageTitle.Actions>{actions}</PageTitle.Actions>
      </PageTitle>
      {notice}
      <GatewayVariablesList />
    </PageContent>
  );
}
