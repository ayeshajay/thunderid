// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageContent, PageTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CaptureOrganizationVersion from '../components/CaptureOrganizationVersion';
import GatewayChain from '../components/GatewayChain';
import OrganizationVersions from '../components/OrganizationVersions';

/**
 * Page showing the gateway promotion chain.
 *
 * A gateway is registered when its data plane is provisioned, not from here: registering one issues
 * the token that data plane dials back with, so the two have to be set up together.
 */
export default function PromotionsListPage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('promotions:listing.title', 'Promotions')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('promotions:listing.subtitle', 'Promote configuration through your gateways and review every change')}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          {/* Capturing reads the workspace rather than any one gateway, so it sits here with the set
              of them rather than on a gateway's own page. */}
          <CaptureOrganizationVersion />
        </PageTitle.Actions>
      </PageTitle>
      {/* Versions belong to the organization, so they are listed here rather than under a gateway. */}
      <OrganizationVersions />
      <GatewayChain />
    </PageContent>
  );
}
