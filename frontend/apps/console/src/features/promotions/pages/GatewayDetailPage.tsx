// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, Card, Chip, CircularProgress, PageContent, PageTitle, Stack, Tab, Tabs, Typography} from '@wso2/oxygen-ui';
import {Undo2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams, useSearchParams} from 'react-router';
import useApplyVersion from '../api/useApplyVersion';
import useCheckVariables from '../api/useCheckVariables';
import useGetGateways from '../api/useGetGateways';
import useGetGatewayHistory from '../api/useGetGatewayHistory';
import useGetVersions from '../api/useGetVersions';
import ApplyDialog from '../components/ApplyDialog';
import DataPlaneStatusChip from '../components/DataPlaneStatusChip';
import MissingVariablesNotice from '../components/MissingVariablesNotice';
import QueuedWorkNotice from '../components/QueuedWorkNotice';
import RevertDialog from '../components/RevertDialog';
import type {Gateway, GatewayApply, Version} from '../models/promotion';
import GatewayVariablesListPage from '../../gateway-variables/pages/GatewayVariablesListPage';
import GatewaySecretsPage from './GatewaySecretsPage';

/**
 * Page showing one gateway's version history, with apply and revert actions.
 *
 * What a gateway owns is which version is on it: the versions it has been given, which one is
 * applied, and moving between them. Producing a version is not a gateway's to do - a capture reads
 * the workspace - so that action lives with the set of gateways rather than here.
 */
export default function GatewayDetailPage(): JSX.Element {
  const {t} = useTranslation();
  const {gatewayId = ''} = useParams<{gatewayId: string}>();

  const {data: envData} = useGetGateways();
  const {data: versionData, isLoading, error} = useGetVersions();
  const {data: historyData} = useGetGatewayHistory(gatewayId);
  const [applyVersionSeq, setApplyVersionSeq] = useState<string | null>(null);
  // Work the data plane has not taken yet. An apply is delivered by the Control Plane pod holding
  // that data plane's connection, which is not always the one that accepted the request.
  const [queuedJobId, setQueuedJobId] = useState<string | undefined>(undefined);
  const applyVersion = useApplyVersion();
  const {data: variableStatus} = useCheckVariables(gatewayId);

  const [revertTo, setRevertTo] = useState<string | undefined>(undefined);

  const [searchParams, setSearchParams] = useSearchParams();
  const requested: string = searchParams.get('tab') ?? 'history';
  const tab: string = ['history', 'variables', 'secrets'].includes(requested) ? requested : 'history';

  const gateways: Gateway[] = useMemo(() => envData?.gateways ?? [], [envData]);
  const gateway: Gateway | undefined = gateways.find((env: Gateway) => env.id === gatewayId);
  const versions: Version[] = versionData?.versions ?? [];
  const history: GatewayApply[] = historyData?.history ?? [];

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{gateway?.name ?? t('promotions:detail.title', 'Gateway')}</PageTitle.Header>
        <PageTitle.SubHeader>
          <Stack direction="row" spacing={1} alignItems="center">
            <span>
              {t(
                'promotions:detail.subtitle',
                'Apply an organization version to this gateway, or go back to one it ran before.',
              )}
            </span>
            <DataPlaneStatusChip status={gateway?.dataPlane} />
          </Stack>
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={1}>
            <Button
              variant="contained"
              color="warning"
              startIcon={<Undo2 size={16} />}
              disabled={versions.length < 2}
              onClick={() => {
                setRevertTo('previous');
              }}
            >
              {t('promotions:detail.revertPrevious', 'Revert to previous')}
            </Button>
          </Stack>
        </PageTitle.Actions>
      </PageTitle>

      {/* A variable and a secret belong to the gateway that resolves them, so they are managed here
          rather than on pages of their own. */}
      <Tabs
        value={tab}
        onChange={(_event: unknown, value: string) => {
          setSearchParams(value === 'history' ? {} : {tab: value});
        }}
        aria-label="gateway tabs"
      >
        <Tab value="history" label={t('promotions:detail.tabHistory', 'History')} sx={{textTransform: 'none'}} />
        <Tab value="variables" label={t('promotions:detail.tabVariables', 'Variables')} sx={{textTransform: 'none'}} />
        <Tab value="secrets" label={t('promotions:detail.tabSecrets', 'Secrets')} sx={{textTransform: 'none'}} />
      </Tabs>

      {tab === 'history' && (
        <>

      <QueuedWorkNotice jobId={queuedJobId} onSettled={() => setQueuedJobId(undefined)} />

      <MissingVariablesNotice
        gatewayId={gatewayId}
        missing={variableStatus?.missing ?? []}
        missingSecrets={variableStatus?.missingSecrets ?? []}
      />

      {isLoading && (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
          <CircularProgress />
        </Box>
      )}

      {error && (
        <Alert severity="error">
          {error.message || t('promotions:detail.error', 'Failed to load the version history')}
        </Alert>
      )}

      {!isLoading && !error && versions.length === 0 && (
        <Alert severity="info">
          {t(
            'promotions:detail.empty',
            'This organization has captured no configuration yet. Capture a version to get started.',
          )}
        </Alert>
      )}

      {/* What this gateway has run. This is the gateway's own record: captures do not appear here,
          because capturing produces an organization version rather than a state of this gateway. */}
      <Typography variant="h6" sx={{mt: 2}}>
        {t('promotions:detail.historyTitle', 'History')}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {t('promotions:detail.historySubtitle', 'Versions this gateway has run, most recent first.')}
      </Typography>

      {history.length === 0 && (
        <Alert severity="info">{t('promotions:detail.historyEmpty', 'This gateway has not run anything yet.')}</Alert>
      )}

      <Stack spacing={2}>
        {history.map((entry: GatewayApply) => {
          const isCurrent: boolean = gateway?.appliedSeq === entry.seq;

          return (
            <Card key={entry.ordinal} sx={{p: 2}}>
              <Stack direction="row" spacing={2} alignItems="center">
                <Box sx={{flexGrow: 1, minWidth: 0}}>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Typography variant="subtitle1" sx={{fontWeight: 600}}>
                      {t('promotions:detail.version', 'Version {{seq}}', {seq: entry.seq})}
                    </Typography>
                    {isCurrent && (
                      <Chip size="small" color="success" label={t('promotions:detail.running', 'Running')} />
                    )}
                  </Stack>
                  <Typography variant="caption" color="text.secondary">
                    {new Date(entry.appliedAt).toLocaleString()}
                  </Typography>
                </Box>

                <Button
                  disabled={isCurrent}
                  onClick={() => {
                    setRevertTo(String(entry.seq));
                  }}
                >
                  {t('promotions:detail.goBackToThis', 'Go back to this')}
                </Button>
              </Stack>
            </Card>
          );
        })}
      </Stack>

      {/* The organization's versions, which any gateway can be moved onto. They belong to the
          organization rather than to this gateway, so they are listed apart from its history. */}
      <Typography variant="h6" sx={{mt: 4}}>
        {t('promotions:detail.availableTitle', 'Organization versions')}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {t('promotions:detail.availableSubtitle', 'Captured configuration this gateway can be moved onto.')}
      </Typography>

      <Stack spacing={2}>
        {versions.map((version: Version) => {
          const isRunning: boolean = gateway?.appliedSeq === version.seq;

          return (
            <Card key={version.seq} sx={{p: 2}}>
              <Stack direction="row" spacing={2} alignItems="center">
                <Box sx={{flexGrow: 1, minWidth: 0}}>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Typography variant="subtitle1" sx={{fontWeight: 600}}>
                      {t('promotions:detail.version', 'Version {{seq}}', {seq: version.seq})}
                    </Typography>
                    <Chip size="small" label={version.origin} />
                    {isRunning && (
                      <Chip size="small" color="success" label={t('promotions:detail.running', 'Running')} />
                    )}
                  </Stack>
                  <Typography variant="caption" color="text.secondary">
                    {new Date(version.createdAt).toLocaleString()}
                    {version.note ? ` \u00b7 ${version.note}` : ''}
                  </Typography>
                </Box>

                <Button
                  disabled={applyVersion.isPending || isRunning}
                  onClick={() => {
                    // An apply rewrites a running deployment, so it is confirmed against a diff of
                    // what is on it now rather than fired straight from the list.
                    setApplyVersionSeq(String(version.seq));
                  }}
                >
                  {t('promotions:detail.apply', 'Apply')}
                </Button>
              </Stack>
            </Card>
          );
        })}
      </Stack>

      <ApplyDialog
        open={applyVersionSeq !== null}
        gatewayId={gatewayId}
        gatewayName={gateway?.name ?? gatewayId}
        version={applyVersionSeq ?? ''}
        onQueued={setQueuedJobId}
        onClose={() => setApplyVersionSeq(null)}
      />

      {revertTo && gateway && (
        <RevertDialog
          open
          gatewayId={gatewayId}
          gatewayName={gateway.name}
          toVersion={revertTo}
          onClose={() => {
            setRevertTo(undefined);
          }}
        />
      )}
        </>
      )}

      {tab === 'variables' && <GatewayVariablesListPage embedded />}
      {tab === 'secrets' && <GatewaySecretsPage embedded />}

    </PageContent>
  );
}
