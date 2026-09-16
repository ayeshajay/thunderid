// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Card, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetVersions from '../api/useGetVersions';
import type {Version} from '../models/promotion';

/**
 * Lists the organization's captured configuration versions, newest first.
 *
 * They are listed here, with the organization, because that is what they belong to. A gateway shows
 * what it has run instead: capturing produces a version of the organization, not a state of any
 * gateway, so a capture never appears in a gateway's history.
 */
export default function OrganizationVersions(): JSX.Element {
  const {t} = useTranslation();
  const {data} = useGetVersions();

  const versions: Version[] = data?.versions ?? [];

  return (
    <Box sx={{mb: 4}}>
      <Typography variant="h6">{t('promotions:versions.title', 'Versions')}</Typography>
      <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
        {t(
          'promotions:versions.subtitle',
          'Captured configuration for this organization. Any gateway can be moved onto any of these.',
        )}
      </Typography>

      {versions.length === 0 ? (
        <Alert severity="info">
          {t('promotions:versions.empty', 'Nothing captured yet. Capture a version to get started.')}
        </Alert>
      ) : (
        <Stack spacing={1}>
          {versions.map((version: Version) => (
            <Card key={version.seq} sx={{p: 2}}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Typography variant="subtitle2" sx={{fontWeight: 600}}>
                  {t('promotions:detail.version', 'Version {{seq}}', {seq: version.seq})}
                </Typography>
                <Chip size="small" label={version.origin} />
                <Box sx={{flexGrow: 1}} />
                <Typography variant="caption" color="text.secondary">
                  {new Date(version.createdAt).toLocaleString()}
                  {version.note ? ` · ${version.note}` : ''}
                </Typography>
              </Stack>
            </Card>
          ))}
        </Stack>
      )}
    </Box>
  );
}
