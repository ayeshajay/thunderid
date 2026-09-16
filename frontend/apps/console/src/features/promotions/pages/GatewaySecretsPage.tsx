// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDataGridLocaleText} from '@thunderid/hooks';
import {Alert, AlertTitle, Box, Button, Chip, CircularProgress, DataGrid, IconButton, ListingTable, PageContent, PageTitle, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {KeyRound, RefreshCw} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useMemo, useState, type JSX, type MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import useDataPlaneJob from '../api/useDataPlaneJob';
import useGetGateways from '../api/useGetGateways';
import useGetGatewaySecrets from '../api/useGetGatewaySecrets';
import GenerateMissingSecretsDialog from '../components/GenerateMissingSecretsDialog';
import RegenerateSecretDialog from '../components/RegenerateSecretDialog';
import SetSecretDialog from '../components/SetSecretDialog';
import type {Gateway, SecretEntry} from '../models/promotion';

/**
 * Manages every credential one gateway's configuration needs, in one place.
 *
 * Nothing here is stored on the Control Plane: a value set on this page goes straight to the
 * gateway's Data Plane secret service, which is the only thing that holds it.
 */
export default function GatewaySecretsPage({embedded = false}: {embedded?: boolean}): JSX.Element {
  const {t} = useTranslation();
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const dataGridLocaleText = useDataGridLocaleText();

  const {data: envData} = useGetGateways();
  const {data, isLoading, error, refetch} = useGetGatewaySecrets(gatewayId);

  // This pod may hold no connection to the data plane, in which case the question was queued for one
  // that does. Follow it and ask again once answered, rather than leaving the page saying the
  // credentials are unknown until someone reloads.
  const {data: pendingJob} = useDataPlaneJob(data?.pendingJobId);
  const pendingSettled: boolean = pendingJob?.status === 'done' || pendingJob?.status === 'failed';
  useEffect((): void => {
    if (pendingSettled) {
      refetch().catch(() => {
        // Ignore; the next poll asks again.
      });
    }
  }, [pendingSettled, refetch]);

  const [toSet, setToSet] = useState<SecretEntry | null>(null);
  const [toRegenerate, setToRegenerate] = useState<SecretEntry | null>(null);
  const [generateOpen, setGenerateOpen] = useState<boolean>(false);

  const gateway: Gateway | undefined = (envData?.gateways ?? []).find((env: Gateway) => env.id === gatewayId);
  const secrets: SecretEntry[] = useMemo(() => data?.secrets ?? [], [data]);
  const missing: SecretEntry[] = useMemo(
    () => (data?.checked ? secrets.filter((secret: SecretEntry) => !secret.held) : []),
    [secrets, data?.checked],
  );
  const generatable: number = missing.filter((secret: SecretEntry) => secret.kind === 'hash').length;

  const columns: DataGrid.GridColDef[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('promotions:secrets.columns.name', 'Placeholder'),
        flex: 1.4,
        minWidth: 260,
      },
      {
        field: 'resourceName',
        headerName: t('promotions:secrets.columns.usedBy', 'Used by'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams) => {
          const secret = params.row as SecretEntry;
          if (!secret.resourceName) {
            return (
              <Typography variant="body2" color="text.secondary">
                {t('promotions:secrets.notInConfiguration', 'Not in the current version')}
              </Typography>
            );
          }
          return (
            <Typography variant="body2">
              {secret.resourceType ? `${secret.resourceType} · ${secret.resourceName}` : secret.resourceName}
              {secret.field ? ` · ${secret.field}` : ''}
            </Typography>
          );
        },
      },
      {
        field: 'kind',
        headerName: t('promotions:secrets.columns.kind', 'Stored as'),
        width: 150,
        renderCell: (params: DataGrid.GridRenderCellParams) =>
          (params.row as SecretEntry).kind === 'hash' ? (
            <Tooltip
              title={t(
                'promotions:secrets.hashedHint',
                'Only ever compared against what a caller presents, so the Data Plane keeps a one-way hash.',
              )}
            >
              <Chip size="small" label={t('promotions:secrets.hashed', 'Hashed')} />
            </Tooltip>
          ) : (
            <Tooltip
              title={t(
                'promotions:secrets.replayedHint',
                'Sent on to an external service, so the Data Plane has to keep the value itself.',
              )}
            >
              <Chip size="small" variant="outlined" label={t('promotions:secrets.replayed', 'Value')} />
            </Tooltip>
          ),
      },
      {
        field: 'held',
        headerName: t('promotions:secrets.columns.status', 'Status'),
        width: 130,
        renderCell: (params: DataGrid.GridRenderCellParams) => {
          if (!data?.checked) {
            return <Chip size="small" label={t('promotions:secrets.unknown', 'Unknown')} />;
          }
          return (params.row as SecretEntry).held ? (
            <Chip size="small" color="success" label={t('promotions:secrets.set', 'Set')} />
          ) : (
            <Chip size="small" color="error" label={t('promotions:secrets.missing', 'Missing')} />
          );
        },
      },
      {
        field: 'actions',
        headerName: t('promotions:secrets.columns.actions', 'Actions'),
        sortable: false,
        width: 120,
        renderCell: (params: DataGrid.GridRenderCellParams) => {
          const secret = params.row as SecretEntry;

          return (
            <ListingTable.RowActions>
              <Tooltip title={t('promotions:secrets.setValue', 'Set value')}>
                <IconButton
                  size="small"
                  onClick={(event: MouseEvent<HTMLButtonElement>) => {
                    event.stopPropagation();
                    setToSet(secret);
                  }}
                >
                  <KeyRound size={16} />
                </IconButton>
              </Tooltip>
              {/* A credential replayed to an external service is issued there, so there is nothing to
                  generate: it can only be set to the value that service gave. */}
              {secret.kind === 'hash' && (
                <Tooltip title={t('promotions:secrets.regenerate', 'Regenerate')}>
                  <IconButton
                    size="small"
                    onClick={(event: MouseEvent<HTMLButtonElement>) => {
                      event.stopPropagation();
                      setToRegenerate(secret);
                    }}
                  >
                    <RefreshCw size={16} />
                  </IconButton>
                </Tooltip>
              )}
            </ListingTable.RowActions>
          );
        },
      },
    ],
    [t, data?.checked],
  );

  const generateAction: JSX.Element = (
    <Button
      variant="contained"
      startIcon={<RefreshCw size={18} />}
      disabled={generatable === 0}
      onClick={() => {
        setGenerateOpen(true);
      }}
    >
      {t('promotions:secrets.generateMissing', 'Generate missing ({{count}})', {count: generatable})}
    </Button>
  );

  if (embedded) {
    return (
      <>
        <Stack direction="row" spacing={1} justifyContent="flex-end" sx={{mb: 2}}>
          {generateAction}
        </Stack>

      {error && (
        <Alert severity="error" sx={{mb: 2}}>
          {error.message || t('promotions:secrets.error', 'Failed to load the secrets of this gateway')}
        </Alert>
      )}

      {/* Without an answer from the secret service, "missing" would be a guess, and generating a
          credential on that guess would replace a working one. */}
      {/* Queued for another pod is "not yet", not "unavailable", so it reads differently. */}
      {data && !data.checked && data.pendingJobId && !pendingSettled && (
        <Alert severity="info" icon={<CircularProgress size={20} />} sx={{mb: 2}}>
          <AlertTitle>
            {t('promotions:secrets.pending', 'Asking the Data Plane which credentials it already holds.')}
          </AlertTitle>
          <Typography variant="body2">
            {t(
              'promotions:secrets.pendingHint',
              'This Control Plane pod holds no connection to it, so the question was passed to one that does.',
            )}
          </Typography>
        </Alert>
      )}

      {data && !data.checked && !(data.pendingJobId && !pendingSettled) && (
        <Alert severity="warning" sx={{mb: 2}}>
          <AlertTitle>
            {t(
              'promotions:secrets.notChecked',
              "The Data Plane's secret service could not be reached, so which credentials are already set is unknown.",
            )}
          </AlertTitle>
          <Typography variant="body2">
            {t(
              'promotions:secrets.notCheckedHint',
              "This is usually this gateway's own target credentials rather than a Data Plane that is down: the same credentials are what an apply authenticates with.",
            )}
          </Typography>
          {data.checkError && (
            <Typography variant="body2" sx={{mt: 1, fontFamily: 'monospace', wordBreak: 'break-all'}}>
              {data.checkError}
            </Typography>
          )}
        </Alert>
      )}

      {data?.checked && missing.length > 0 && (
        <Alert severity="error" sx={{mb: 2}}>
          {t(
            'promotions:secrets.missingNotice',
            '{{count}} credential is not set. Applying now creates resources whose credentials reject every attempt.',
            {count: missing.length},
          )}
        </Alert>
      )}

      {!isLoading && !error && secrets.length === 0 && (
        <Alert severity="info">
          {t(
            'promotions:secrets.empty',
            'This gateway has no captured version yet, so there are no credentials to manage.',
          )}
        </Alert>
      )}

      {secrets.length > 0 && (
        <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
          <ListingTable.Container disablePaper>
            <ListingTable.DataGrid
              rows={secrets}
              columns={columns}
              getRowId={(row) => (row as SecretEntry).name}
              disableRowSelectionOnClick
              localeText={dataGridLocaleText}
              autoHeight
              initialState={{pagination: {paginationModel: {pageSize: 25}}}}
              pageSizeOptions={[10, 25, 50]}
            />
          </ListingTable.Container>
        </ListingTable.Provider>
      )}

      <Box>
        <SetSecretDialog
          open={Boolean(toSet)}
          gatewayId={gatewayId}
          secret={toSet}
          onClose={() => {
            setToSet(null);
          }}
        />
        <RegenerateSecretDialog
          open={Boolean(toRegenerate)}
          gatewayId={gatewayId}
          secret={toRegenerate}
          onClose={() => {
            setToRegenerate(null);
          }}
        />
        <GenerateMissingSecretsDialog
          open={generateOpen}
          gatewayId={gatewayId}
          missing={missing}
          onClose={() => {
            setGenerateOpen(false);
          }}
        />
      </Box>
      </>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>
          {gateway
            ? t('promotions:secrets.titleFor', 'Secrets · {{env}}', {env: gateway.name})
            : t('promotions:secrets.title', 'Secrets')}
        </PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'promotions:secrets.subtitle',
            'Credentials this gateway needs. They are held by the Data Plane, never by the Control Plane.',
          )}
        </PageTitle.SubHeader>
        <PageTitle.Actions>{generateAction}</PageTitle.Actions>
      </PageTitle>

      {error && (
        <Alert severity="error" sx={{mb: 2}}>
          {error.message || t('promotions:secrets.error', 'Failed to load the secrets of this gateway')}
        </Alert>
      )}

      {/* Without an answer from the secret service, "missing" would be a guess, and generating a
          credential on that guess would replace a working one. */}
      {/* Queued for another pod is "not yet", not "unavailable", so it reads differently. */}
      {data && !data.checked && data.pendingJobId && !pendingSettled && (
        <Alert severity="info" icon={<CircularProgress size={20} />} sx={{mb: 2}}>
          <AlertTitle>
            {t('promotions:secrets.pending', 'Asking the Data Plane which credentials it already holds.')}
          </AlertTitle>
          <Typography variant="body2">
            {t(
              'promotions:secrets.pendingHint',
              'This Control Plane pod holds no connection to it, so the question was passed to one that does.',
            )}
          </Typography>
        </Alert>
      )}

      {data && !data.checked && !(data.pendingJobId && !pendingSettled) && (
        <Alert severity="warning" sx={{mb: 2}}>
          <AlertTitle>
            {t(
              'promotions:secrets.notChecked',
              "The Data Plane's secret service could not be reached, so which credentials are already set is unknown.",
            )}
          </AlertTitle>
          <Typography variant="body2">
            {t(
              'promotions:secrets.notCheckedHint',
              "This is usually this gateway's own target credentials rather than a Data Plane that is down: the same credentials are what an apply authenticates with.",
            )}
          </Typography>
          {data.checkError && (
            <Typography variant="body2" sx={{mt: 1, fontFamily: 'monospace', wordBreak: 'break-all'}}>
              {data.checkError}
            </Typography>
          )}
        </Alert>
      )}

      {data?.checked && missing.length > 0 && (
        <Alert severity="error" sx={{mb: 2}}>
          {t(
            'promotions:secrets.missingNotice',
            '{{count}} credential is not set. Applying now creates resources whose credentials reject every attempt.',
            {count: missing.length},
          )}
        </Alert>
      )}

      {!isLoading && !error && secrets.length === 0 && (
        <Alert severity="info">
          {t(
            'promotions:secrets.empty',
            'This gateway has no captured version yet, so there are no credentials to manage.',
          )}
        </Alert>
      )}

      {secrets.length > 0 && (
        <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
          <ListingTable.Container disablePaper>
            <ListingTable.DataGrid
              rows={secrets}
              columns={columns}
              getRowId={(row) => (row as SecretEntry).name}
              disableRowSelectionOnClick
              localeText={dataGridLocaleText}
              autoHeight
              initialState={{pagination: {paginationModel: {pageSize: 25}}}}
              pageSizeOptions={[10, 25, 50]}
            />
          </ListingTable.Container>
        </ListingTable.Provider>
      )}

      <Box>
        <SetSecretDialog
          open={Boolean(toSet)}
          gatewayId={gatewayId}
          secret={toSet}
          onClose={() => {
            setToSet(null);
          }}
        />
        <RegenerateSecretDialog
          open={Boolean(toRegenerate)}
          gatewayId={gatewayId}
          secret={toRegenerate}
          onClose={() => {
            setToRegenerate(null);
          }}
        />
        <GenerateMissingSecretsDialog
          open={generateOpen}
          gatewayId={gatewayId}
          missing={missing}
          onClose={() => {
            setGenerateOpen(false);
          }}
        />
      </Box>
    </PageContent>
  );
}
