// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Tooltip} from '@wso2/oxygen-ui';
import {CloudDownload} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useCaptureVersion from '../api/useCaptureVersion';

/**
 * Captures the organization's configuration as a new version.
 *
 * A version belongs to the organization, not to a gateway. What a capture reads is the
 * organization's configuration as authored here, which is the same whichever gateway it is later
 * applied to, so a gateway cannot capture a state of its own: it only receives a version, by having
 * one applied to it or by going back to one it already ran.
 *
 * That is why this action sits with the organization rather than on a gateway's page.
 */
export default function CaptureOrganizationVersion(): JSX.Element {
  const {t} = useTranslation();
  const captureVersion = useCaptureVersion();

  return (
    <Tooltip title={t('promotions:capture.hint', "Capture this organization's configuration as a new version")}>
      <span>
        <Button
          startIcon={<CloudDownload size={16} />}
          disabled={captureVersion.isPending}
          onClick={() => {
            captureVersion.mutate({});
          }}
        >
          {captureVersion.isPending
            ? t('promotions:capture.inProgress', 'Capturing...')
            : t('promotions:capture.action', 'Capture version')}
        </Button>
      </span>
    </Tooltip>
  );
}
