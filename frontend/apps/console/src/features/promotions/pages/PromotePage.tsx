// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, Card, PageContent, PageTitle, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import useGetGateways from '../api/useGetGateways';
import useGetPromotionTargets, {type PromotionTarget} from '../api/useGetPromotionTargets';
import DataPlaneStatusChip from '../components/DataPlaneStatusChip';
import PromoteDialog from '../components/PromoteDialog';
import type {Gateway} from '../models/promotion';

/**
 * Chooses which gateway to promote a version into.
 *
 * The target is picked here rather than derived from a chain, because gateways are a flat set. On its
 * own the Control Plane permits any pair, so every other gateway is offered. With an gateway
 * manager connected the hierarchy decides, and it accepts or refuses the move that is asked for.
 */
export default function PromotePage(): JSX.Element {
  const {t} = useTranslation();
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const {data} = useGetGateways();
  const {data: permittedTargets} = useGetPromotionTargets(gatewayId);
  const [target, setTarget] = useState<Gateway | undefined>(undefined);

  const gateways: Gateway[] = data?.gateways ?? [];
  const source: Gateway | undefined = gateways.find((env: Gateway) => env.id === gatewayId);
  // Where this may go is the environment manager's answer when one is connected, and every other
  // gateway when none is. Offering a target the hierarchy refuses would only produce a rejection.
  const permitted: PromotionTarget[] = permittedTargets ?? [];
  const targets: Gateway[] = gateways.filter((env: Gateway) =>
    permitted.some((target: PromotionTarget) => target.gatewayId === env.id),
  );

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>
          {t('promotions:promote.title', 'Promote {{name}}', {name: source?.name ?? gatewayId})}
        </PageTitle.Header>
        <PageTitle.SubHeader>
          {t('promotions:promote.subtitle', 'Choose which gateway to promote this configuration into.')}
        </PageTitle.SubHeader>
      </PageTitle>

      {targets.length === 0 && (
        <Alert severity="info">
          {t(
            'promotions:promote.noTargets',
            'There is nowhere to promote {{name}} to. It is the last step of this organization\u2019s promotion path.',
            {name: source?.name ?? gatewayId},
          )}
        </Alert>
      )}

      <Stack spacing={2}>
        {targets.map((env: Gateway) => (
          <Card key={env.id} sx={{p: 2}}>
            <Stack direction="row" spacing={2} alignItems="center" flexWrap="wrap" useFlexGap>
              <Box sx={{flexGrow: 1, minWidth: 200}}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography variant="subtitle1" sx={{fontWeight: 600}}>
                    {env.name}
                  </Typography>
                  <DataPlaneStatusChip status={env.dataPlane} />
                </Stack>
                <Typography variant="caption" color="text.secondary">
                  {t('promotions:listing.versionState', 'Latest v{{latest}} · Applied v{{applied}}', {
                    applied: env.appliedSeq || '-',
                    latest: env.latestSeq || '-',
                  })}
                </Typography>
              </Box>
              <Button
                variant="contained"
                endIcon={<ArrowRight size={16} />}
                onClick={() => {
                  setTarget(env);
                }}
              >
                {t('promotions:promote.into', 'Promote into {{target}}', {target: env.name})}
              </Button>
            </Stack>
          </Card>
        ))}
      </Stack>

      {target && source && (
        <PromoteDialog
          open
          fromGatewayId={source.id}
          fromGatewayName={source.name}
          toGatewayId={target.id}
          toGatewayName={target.name}
          toDataPlaneConnected={target.dataPlane?.connected ?? false}
          onClose={() => {
            setTarget(undefined);
          }}
        />
      )}
    </PageContent>
  );
}
