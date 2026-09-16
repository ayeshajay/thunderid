// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import useDeleteGatewayVariable from '../api/useDeleteGatewayVariable';

interface GatewayVariableDeleteDialogProps {
  open: boolean;
  gatewayVariableId: string;
  onClose: () => void;
}

/**
 * Confirms deleting an gateway variable.
 */
export default function GatewayVariableDeleteDialog({
  open,
  gatewayVariableId,
  onClose,
}: GatewayVariableDeleteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const deleteGatewayVariable = useDeleteGatewayVariable(gatewayId);

  const handleDelete = (): void => {
    deleteGatewayVariable.mutate(gatewayVariableId, {onSuccess: onClose});
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('gatewayVariables:delete.title', 'Delete Gateway Variable')}</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {t(
            'gatewayVariables:delete.message',
            'Are you sure you want to delete this gateway variable? This action cannot be undone.',
          )}
        </DialogContentText>
        <Alert severity="warning" sx={{mt: 2}}>
          {t(
            'gatewayVariables:delete.disclaimer',
            'Configuration that references this variable will no longer resolve when applied to a Data Plane.',
          )}
        </Alert>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={deleteGatewayVariable.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" color="error" onClick={handleDelete} disabled={deleteGatewayVariable.isPending}>
          {deleteGatewayVariable.isPending
            ? t('common:status.deleting', 'Deleting...')
            : t('common:actions.delete', 'Delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
