// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useGetPromotionTargets, {type PromotionTarget} from '../api/useGetPromotionTargets';

interface PromoteActionProps {
  gatewayId: string;
  /** False when this gateway has nothing to promote yet. */
  hasVersion: boolean;
}

/**
 * Offers promotion out of one gateway, and only where there is somewhere for it to go.
 *
 * Where a version may go is not this console's to decide. With an environment manager connected it
 * holds the organization's hierarchy, so a gateway at the top of it has no target and the action is
 * not offered at all: showing it there would invite an operator to attempt a move the hierarchy
 * refuses. Without one, every other gateway is a target and the action always shows.
 */
export default function PromoteAction({gatewayId, hasVersion}: PromoteActionProps): JSX.Element | null {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {data: targets, isLoading} = useGetPromotionTargets(gatewayId);

  // While the answer is unknown, offering nothing is better than offering a move that may be refused.
  if (isLoading || (targets ?? []).length === 0) {
    return null;
  }

  return (
    <Button
      variant="contained"
      disabled={!hasVersion}
      onClick={() => {
        void navigate(`/promotions/${gatewayId}/promote`);
      }}
    >
      {t('promotions:listing.promote', 'Promote')}
    </Button>
  );
}

export type {PromotionTarget};
