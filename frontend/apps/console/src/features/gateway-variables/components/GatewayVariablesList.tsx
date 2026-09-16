// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDataGridLocaleText} from '@thunderid/hooks';
import {Box, DataGrid, IconButton, ListingTable, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX, type MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import GatewayVariableDeleteDialog from './GatewayVariableDeleteDialog';
import useGetGatewayVariables from '../api/useGetGatewayVariables';
import type {GatewayVariable} from '../models/gateway-variable';

/**
 * Table of the gateway variables applied to Data Planes.
 */
export default function GatewayVariablesList(): JSX.Element {
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const dataGridLocaleText = useDataGridLocaleText();

  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [selectedId, setSelectedId] = useState<string>('');

  const params = useMemo(
    () => ({limit: paginationModel.pageSize, offset: paginationModel.page * paginationModel.pageSize}),
    [paginationModel],
  );
  const {data, isLoading, error} = useGetGatewayVariables(gatewayId, params);

  const handleEdit = (id: string): void => {
    void navigate(`/promotions/${gatewayId}/variables/${id}`);
  };

  const columns: DataGrid.GridColDef[] = useMemo(
    () => [
      {field: 'key', headerName: t('gatewayVariables:listing.columns.key', 'Key'), flex: 1, minWidth: 200},
      {field: 'value', headerName: t('gatewayVariables:listing.columns.value', 'Value'), flex: 1, minWidth: 200},
      {
        field: 'description',
        headerName: t('gatewayVariables:listing.columns.description', 'Description'),
        flex: 1,
        minWidth: 160,
      },
      {
        field: 'actions',
        headerName: t('gatewayVariables:listing.columns.actions', 'Actions'),
        sortable: false,
        width: 120,
        renderCell: (params: DataGrid.GridRenderCellParams) => (
          <ListingTable.RowActions>
            <Tooltip title={t('common:actions.edit', 'Edit')}>
              <IconButton
                size="small"
                onClick={(event: MouseEvent<HTMLButtonElement>) => {
                  event.stopPropagation();
                  handleEdit((params.row as GatewayVariable).id);
                }}
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete', 'Delete')}>
              <IconButton
                size="small"
                onClick={(event: MouseEvent<HTMLButtonElement>) => {
                  event.stopPropagation();
                  setSelectedId((params.row as GatewayVariable).id);
                  setDeleteDialogOpen(true);
                }}
              >
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </ListingTable.RowActions>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t],
  );

  if (error) {
    return (
      <Box sx={{py: 8, textAlign: 'center'}}>
        <Typography variant="h6" color="error" gutterBottom>
          {t('gatewayVariables:listing.error', 'Failed to load gateway variables')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message ?? t('common:messages.somethingWentWrong', 'Something went wrong')}
        </Typography>
      </Box>
    );
  }

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.gatewayVariables ?? []}
            columns={columns}
            getRowId={(row) => (row as GatewayVariable).id}
            onRowClick={(params) => {
              handleEdit((params.row as GatewayVariable).id);
            }}
            paginationMode="server"
            rowCount={data?.totalResults ?? 0}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            pageSizeOptions={[5, 10, 25]}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            autoHeight
            sx={{'& .MuiDataGrid-row': {cursor: 'pointer'}}}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <GatewayVariableDeleteDialog
        open={deleteDialogOpen}
        gatewayVariableId={selectedId}
        onClose={() => {
          setDeleteDialogOpen(false);
        }}
      />
    </>
  );
}
