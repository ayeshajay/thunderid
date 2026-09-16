// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// The Control Plane console's own route tree.
//
// It is separate from the Data Plane console's rather than the same one with entries hidden, because
// the two planes differ in what they can do, not only in what they show. A Control Plane holds
// configuration and serves no runtime: it has no /flow/execute, so a page that runs a flow cannot
// work here and is not routed at all. Adding a user is a form, and the tryout journeys, which run
// flows, are absent.
//
// Everything else is shared. The feature packages and the console's own pages, layouts and route
// config are imported through the @console alias, so an authoring surface is written once.

import {PageLoader} from '@thunderid/components';
import {LayoutBuilderProvider, ThemeBuilderProvider} from '@thunderid/configure-design';
import {GroupCreateProvider} from '@thunderid/configure-groups';
import {OrganizationUnitProvider} from '@thunderid/configure-organization-units';
import {RoleCreateProvider} from '@thunderid/configure-roles';
import {TranslationCreateProvider} from '@thunderid/configure-translations';
import {UserTypeCreateProvider} from '@thunderid/configure-user-types';
import {RoutesProvider, ToastProvider} from '@thunderid/contexts';
import {ProtectedRoute} from '@thunderid/react-router';
import {lazy, Suspense, type JSX} from 'react';
import {BrowserRouter, Navigate, Outlet, Route, Routes} from 'react-router';
import RouteConfig, {ROUTE_SEGMENTS} from '@console/configs/RouteConfig';
import AgentCreateProvider from '@console/features/agents/contexts/AgentCreate/AgentCreateProvider';
import ApplicationCreateProvider from '@console/features/applications/contexts/ApplicationCreate/ApplicationCreateProvider';
import OrganizationUnitDefaultFlowsSettings from '@console/features/organization-units/OrganizationUnitDefaultFlowsSettings';
import GetStartedPage from '@console/features/welcome/pages/GetStartedPage';
import DashboardLayout from '@console/layouts/DashboardLayout';
import FullScreenLayout from '@console/layouts/FullScreenLayout';

const ViewAgentTypePage = lazy(() =>
  import('@thunderid/configure-agent-types').then((m) => ({default: m.ViewAgentTypePage})),
);
const CreateOrganizationUnitPage = lazy(() =>
  import('@thunderid/configure-organization-units').then((m) => ({default: m.CreateOrganizationUnitPage})),
);
const OrganizationUnitEditPage = lazy(() =>
  import('@thunderid/configure-organization-units').then((m) => ({default: m.OrganizationUnitEditPage})),
);
const OrganizationUnitsListPage = lazy(() =>
  import('@thunderid/configure-organization-units').then((m) => ({default: m.OrganizationUnitsListPage})),
);
const TranslationCreatePage = lazy(() =>
  import('@thunderid/configure-translations').then((m) => ({default: m.TranslationCreatePage})),
);
const TranslationsEditPage = lazy(() =>
  import('@console/lib/monaco-setup').then(() =>
    import('@thunderid/configure-translations').then((m) => ({default: m.TranslationsEditPage})),
  ),
);
const TranslationsListPage = lazy(() =>
  import('@thunderid/configure-translations').then((m) => ({default: m.TranslationsListPage})),
);
const UserAddPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserAddFormPage})));
const UserCreatePage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserCreatePage})));
const UserEditPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UserEditPage})));
const UsersListPage = lazy(() => import('@thunderid/configure-users').then((m) => ({default: m.UsersListPage})));
const ResourceServersListPage = lazy(() =>
  import('@thunderid/configure-resource-servers').then((m) => ({default: m.ResourceServersListPage})),
);
const ResourceServerEditPage = lazy(() =>
  import('@thunderid/configure-resource-servers').then((m) => ({default: m.ResourceServerEditPage})),
);
const CreateResourceServerPage = lazy(() =>
  import('@thunderid/configure-resource-servers').then((m) => ({default: m.CreateResourceServerPage})),
);

const AgentCreatePage = lazy(() => import('@console/features/agents/pages/AgentCreatePage'));
const AgentEditPage = lazy(() =>
  import('@console/lib/monaco-setup').then(() => import('@console/features/agents/pages/AgentEditPage')),
);
const AgentsListPage = lazy(() => import('@console/features/agents/pages/AgentsListPage'));
const ApplicationCreatePage = lazy(() => import('@console/features/applications/pages/ApplicationCreatePage'));
const ApplicationEditPage = lazy(() =>
  import('@console/lib/monaco-setup').then(() => import('@console/features/applications/pages/ApplicationEditPage')),
);
const ApplicationsListPage = lazy(() => import('@console/features/applications/pages/ApplicationsListPage'));
const ApplicationTemplateSelectPage = lazy(
  () => import('@console/features/applications/pages/ApplicationTemplateSelectPage'),
);
const DesignPage = lazy(() => import('@thunderid/configure-design').then((m) => ({default: m.DesignPage})));
const LayoutBuilderPage = lazy(() =>
  import('@console/lib/monaco-setup').then(() =>
    import('@thunderid/configure-design').then((m) => ({default: m.LayoutBuilderPage})),
  ),
);
const ThemeBuilderPage = lazy(() => import('@thunderid/configure-design').then((m) => ({default: m.ThemeBuilderPage})));
const ThemeCreatePage = lazy(() => import('@thunderid/configure-design').then((m) => ({default: m.ThemeCreatePage})));
const FlowCreatePage = lazy(() => import('@console/features/flows/pages/FlowCreatePage'));
const FlowsListPage = lazy(() => import('@console/features/flows/pages/FlowsListPage'));
const CreateGroupPage = lazy(() => import('@thunderid/configure-groups').then((m) => ({default: m.CreateGroupPage})));
const GroupEditPage = lazy(() => import('@thunderid/configure-groups').then((m) => ({default: m.GroupEditPage})));
const GroupsListPage = lazy(() => import('@thunderid/configure-groups').then((m) => ({default: m.GroupsListPage})));
const HomePage = lazy(() => import('@console/features/home/pages/HomePage'));
const ExportPage = lazy(() =>
  import('@console/lib/monaco-setup').then(() =>
    import('@thunderid/configure-import-export').then((m) => ({default: m.ExportPage})),
  ),
);
const ImportConfigurationSummaryPage = lazy(() =>
  import('@console/lib/monaco-setup').then(() =>
    import('@thunderid/configure-import-export').then((m) => ({default: m.ImportConfigurationSummaryPage})),
  ),
);
const ImportConfigurationUploadPage = lazy(() =>
  import('@thunderid/configure-import-export').then((m) => ({default: m.ImportConfigurationUploadPage})),
);
const ImportConfigurationValidatePage = lazy(() =>
  import('@thunderid/configure-import-export').then((m) => ({default: m.ImportConfigurationValidatePage})),
);
const ImportExportPage = lazy(() =>
  import('@thunderid/configure-import-export').then((m) => ({default: m.ImportExportPage})),
);
const ConnectionsListPage = lazy(() =>
  import('@thunderid/configure-connections').then((m) => ({default: m.ConnectionsListPage})),
);
const ConnectionDetailPage = lazy(() =>
  import('@thunderid/configure-connections').then((m) => ({default: m.ConnectionDetailPage})),
);
const ConnectionConfigureWizardPage = lazy(() =>
  import('@thunderid/configure-connections').then((m) => ({default: m.ConnectionConfigureWizardPage})),
);
const ConnectionCreateWizardPage = lazy(() =>
  import('@thunderid/configure-connections').then((m) => ({default: m.ConnectionCreateWizardPage})),
);
const PromotionsListPage = lazy(() => import('@console/features/promotions/pages/PromotionsListPage'));
const GatewayDetailPage = lazy(() => import('@console/features/promotions/pages/GatewayDetailPage'));
const GatewaySecretsPage = lazy(() => import('@console/features/promotions/pages/GatewaySecretsPage'));
const PromotePage = lazy(() => import('@console/features/promotions/pages/PromotePage'));
const GatewayVariablesListPage = lazy(
  () => import('@console/features/gateway-variables/pages/GatewayVariablesListPage'),
);
const GatewayVariableEditPage = lazy(() => import('@console/features/gateway-variables/pages/GatewayVariableEditPage'));
const CreateGatewayVariablePage = lazy(
  () => import('@console/features/gateway-variables/pages/CreateGatewayVariablePage'),
);
const FlowBuilderPage = lazy(() => import('@console/features/flows/pages/FlowBuilderPage'));
const CreateRolePage = lazy(() => import('@thunderid/configure-roles').then((m) => ({default: m.CreateRolePage})));
const RoleEditPage = lazy(() => import('@thunderid/configure-roles').then((m) => ({default: m.RoleEditPage})));
const RolesListPage = lazy(() => import('@thunderid/configure-roles').then((m) => ({default: m.RolesListPage})));
const SettingsPage = lazy(() => import('@thunderid/configure-settings').then((m) => ({default: m.SettingsPage})));
const TrustedIssuerDetailPage = lazy(() =>
  import('@thunderid/configure-connections').then((m) => ({default: m.TrustedIssuerDetailPage})),
);
const VerifiablePresentationsListPage = lazy(() =>
  import('@thunderid/configure-verifiable-credentials').then((m) => ({default: m.VerifiablePresentationsListPage})),
);
const VerifiablePresentationCreatePage = lazy(() =>
  import('@thunderid/configure-verifiable-credentials').then((m) => ({default: m.VerifiablePresentationCreatePage})),
);
const VerifiablePresentationEditPage = lazy(() =>
  import('@thunderid/configure-verifiable-credentials').then((m) => ({default: m.VerifiablePresentationEditPage})),
);
const VerifiableCredentialsListPage = lazy(() =>
  import('@thunderid/configure-verifiable-credentials').then((m) => ({default: m.VerifiableCredentialsListPage})),
);
const VerifiableCredentialCreatePage = lazy(() =>
  import('@thunderid/configure-verifiable-credentials').then((m) => ({default: m.VerifiableCredentialCreatePage})),
);
const VerifiableCredentialEditPage = lazy(() =>
  import('@thunderid/configure-verifiable-credentials').then((m) => ({default: m.VerifiableCredentialEditPage})),
);
const CreateUserTypePage = lazy(() =>
  import('@thunderid/configure-user-types').then((m) => ({default: m.CreateUserTypePage})),
);
const UserTypesListPage = lazy(() =>
  import('@thunderid/configure-user-types').then((m) => ({default: m.UserTypesListPage})),
);
const ViewUserTypePage = lazy(() =>
  import('@thunderid/configure-user-types').then((m) => ({default: m.ViewUserTypePage})),
);
const CreateProjectPage = lazy(() => import('@console/features/welcome/pages/CreateProjectPage'));
const WelcomePage = lazy(() => import('@console/features/welcome/pages/WelcomePage'));

export default function App(): JSX.Element {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <RoutesProvider paths={RouteConfig}>
        <ToastProvider>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route
                path="/"
                element={
                  <ProtectedRoute>
                    <DashboardLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<HomePage />} />
                <Route path={ROUTE_SEGMENTS.home} element={<HomePage />} />
                <Route path={ROUTE_SEGMENTS.users} element={<UsersListPage />} />
                <Route path={`${ROUTE_SEGMENTS.users}/:userId`} element={<UserEditPage />} />
                <Route path={ROUTE_SEGMENTS.userTypes} element={<UserTypesListPage />} />
                <Route path={`${ROUTE_SEGMENTS.userTypes}/:id`} element={<ViewUserTypePage />} />
                <Route path={`${ROUTE_SEGMENTS.agentTypes}/:id`} element={<ViewAgentTypePage />} />
                <Route path={ROUTE_SEGMENTS.connections} element={<ConnectionsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.connections}/:type`} element={<ConnectionDetailPage />} />
                <Route path={`${ROUTE_SEGMENTS.connections}/:type/:id`} element={<ConnectionDetailPage />} />
                <Route
                  path={ROUTE_SEGMENTS.trustedIssuers}
                  element={<Navigate to={RouteConfig.connections.list()} replace />}
                />
                <Route path={`${ROUTE_SEGMENTS.trustedIssuers}/:id`} element={<TrustedIssuerDetailPage />} />
                <Route path={ROUTE_SEGMENTS.groups} element={<GroupsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.groups}/:groupId`} element={<GroupEditPage />} />
                <Route path={ROUTE_SEGMENTS.roles} element={<RolesListPage />} />
                <Route path={`${ROUTE_SEGMENTS.roles}/:roleId`} element={<RoleEditPage />} />
                <Route path={ROUTE_SEGMENTS.verifiablePresentations} element={<VerifiablePresentationsListPage />} />
                <Route
                  path={`${ROUTE_SEGMENTS.verifiablePresentations}/:vpId`}
                  element={<VerifiablePresentationEditPage />}
                />
                <Route path={ROUTE_SEGMENTS.verifiableCredentials} element={<VerifiableCredentialsListPage />} />
                <Route
                  path={`${ROUTE_SEGMENTS.verifiableCredentials}/:vcId`}
                  element={<VerifiableCredentialEditPage />}
                />
                <Route path={ROUTE_SEGMENTS.applications} element={<ApplicationsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.applications}/:applicationId`} element={<ApplicationEditPage />} />
                <Route path={ROUTE_SEGMENTS.agents} element={<AgentsListPage />} />
                <Route path={`${ROUTE_SEGMENTS.agents}/:agentId`} element={<AgentEditPage />} />
                <Route path={ROUTE_SEGMENTS.flows} element={<FlowsListPage />} />
                <Route path={ROUTE_SEGMENTS.resourceServers} element={<ResourceServersListPage />} />
                <Route
                  path={`${ROUTE_SEGMENTS.resourceServers}/:resourceServerId`}
                  element={<ResourceServerEditPage />}
                />
                <Route path={ROUTE_SEGMENTS.settings} element={<SettingsPage />} />
                <Route path="promotions" element={<PromotionsListPage />} />
                <Route path="promotions/:gatewayId" element={<GatewayDetailPage />} />
                <Route path="promotions/:gatewayId/secrets" element={<GatewaySecretsPage />} />
                <Route path="promotions/:gatewayId/promote" element={<PromotePage />} />
                {/* A variable belongs to a gateway, so it is reached through one. */}
                <Route path="promotions/:gatewayId/variables" element={<GatewayVariablesListPage />} />
                <Route
                  path="promotions/:gatewayId/variables/:gatewayVariableId"
                  element={<GatewayVariableEditPage />}
                />
              </Route>
              {/* Organization Units - wrapped in OrganizationUnitProvider to preserve tree state across navigation */}
              <Route
                path={RouteConfig.organizationUnits.list()}
                element={
                  <ProtectedRoute>
                    <OrganizationUnitProvider />
                  </ProtectedRoute>
                }
              >
                <Route element={<DashboardLayout />}>
                  <Route index element={<OrganizationUnitsListPage />} />
                  <Route
                    path=":id"
                    element={
                      <OrganizationUnitEditPage
                        renderDefaultFlowsSettings={(props) => <OrganizationUnitDefaultFlowsSettings {...props} />}
                      />
                    }
                  />
                </Route>
                <Route path="create" element={<FullScreenLayout />}>
                  <Route index element={<CreateOrganizationUnitPage />} />
                </Route>
              </Route>
              <Route
                path={RouteConfig.groups.create()}
                element={
                  <ProtectedRoute>
                    <GroupCreateProvider>
                      <FullScreenLayout />
                    </GroupCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateGroupPage />} />
              </Route>
              <Route
                path={RouteConfig.roles.create()}
                element={
                  <ProtectedRoute>
                    <RoleCreateProvider>
                      <FullScreenLayout />
                    </RoleCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateRolePage />} />
              </Route>
              <Route
                path={RouteConfig.users.add()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<UserAddPage />} />
              </Route>
              <Route
                path={RouteConfig.users.addCreate()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<UserCreatePage />} />
              </Route>
              <Route
                path={RouteConfig.userTypes.create()}
                element={
                  <ProtectedRoute>
                    <UserTypeCreateProvider>
                      <FullScreenLayout />
                    </UserTypeCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateUserTypePage />} />
              </Route>
              <Route
                path={RouteConfig.verifiablePresentations.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<VerifiablePresentationCreatePage />} />
              </Route>
              <Route
                path={RouteConfig.verifiableCredentials.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<VerifiableCredentialCreatePage />} />
              </Route>
              <Route
                element={
                  <ProtectedRoute>
                    <ApplicationCreateProvider>
                      <Outlet />
                    </ApplicationCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route path={RouteConfig.applications.types()} element={<DashboardLayout />}>
                  <Route index element={<ApplicationTemplateSelectPage />} />
                </Route>
                <Route path={RouteConfig.applications.create()} element={<FullScreenLayout />}>
                  <Route index element={<ApplicationCreatePage />} />
                </Route>
              </Route>
              <Route
                path={RouteConfig.agents.create()}
                element={
                  <ProtectedRoute>
                    <AgentCreateProvider>
                      <FullScreenLayout />
                    </AgentCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<AgentCreatePage />} />
              </Route>
              <Route
                path={RouteConfig.resourceServers.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateResourceServerPage />} />
              </Route>
              <Route
                path={RouteConfig.flows.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<FlowCreatePage />} />
              </Route>
              <Route
                path={RouteConfig.connections.create()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ConnectionCreateWizardPage />} />
              </Route>
              <Route
                path={`/${ROUTE_SEGMENTS.connections}/:type/configure`}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ConnectionConfigureWizardPage />} />
              </Route>
              <Route
                path={RouteConfig.flows.detail(':flowId')}
                element={
                  <ProtectedRoute>
                    <DashboardLayout collapseSidebar />
                  </ProtectedRoute>
                }
              >
                <Route index element={<FlowBuilderPage />} />
              </Route>
              <Route
                path={RouteConfig.importExport.list()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ImportExportPage />} />
              </Route>
              <Route
                path={RouteConfig.export.page()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ExportPage />} />
              </Route>
              <Route
                path={RouteConfig.importConfiguration.upload()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ImportConfigurationUploadPage />} />
                <Route path="validate" element={<ImportConfigurationValidatePage />} />
                <Route path="summary" element={<ImportConfigurationSummaryPage />} />
              </Route>
              <Route
                path={RouteConfig.welcome.root()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<WelcomePage />} />
                <Route path="create-project" element={<CreateProjectPage />} />
                <Route path="import-configuration" element={<ImportConfigurationUploadPage />} />
                <Route path="import-configuration/validate" element={<ImportConfigurationValidatePage />} />
                <Route path="import-configuration/summary" element={<ImportConfigurationSummaryPage />} />
                <Route path="get-started" element={<GetStartedPage />} />
                <Route
                  element={
                    <ApplicationCreateProvider>
                      <Outlet />
                    </ApplicationCreateProvider>
                  }
                >
                  <Route path="get-started/applications/types" element={<ApplicationTemplateSelectPage />} />
                  <Route path="get-started/applications/create" element={<ApplicationCreatePage />} />
                </Route>
                <Route
                  element={
                    <AgentCreateProvider>
                      <Outlet />
                    </AgentCreateProvider>
                  }
                >
                  <Route path="get-started/agents/create" element={<AgentCreatePage />} />
                </Route>
                {/* The tryout journeys run onboarding flows, which this plane cannot execute, so
                    they are not routed here. Get started only links to the creation wizards below,
                    which are plain writes, so it stays. */}
              </Route>
              <Route
                path={RouteConfig.design.list()}
                element={
                  <ProtectedRoute>
                    <DashboardLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<DesignPage />} />
              </Route>
              <Route
                path={RouteConfig.design.themesCreate()}
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<ThemeCreatePage />} />
              </Route>
              <Route
                path={`/${ROUTE_SEGMENTS.design}/themes/:themeId`}
                element={
                  <ProtectedRoute>
                    <ThemeBuilderProvider>
                      <DashboardLayout />
                    </ThemeBuilderProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<ThemeBuilderPage />} />
              </Route>
              <Route
                path={`/${ROUTE_SEGMENTS.design}/layouts/:layoutId`}
                element={
                  <ProtectedRoute>
                    <LayoutBuilderProvider>
                      <DashboardLayout />
                    </LayoutBuilderProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<LayoutBuilderPage />} />
              </Route>
              <Route
                path={RouteConfig.translations.create()}
                element={
                  <ProtectedRoute>
                    <TranslationCreateProvider>
                      <FullScreenLayout />
                    </TranslationCreateProvider>
                  </ProtectedRoute>
                }
              >
                <Route index element={<TranslationCreatePage />} />
              </Route>
              <Route
                path="/promotions/:gatewayId/variables/create"
                element={
                  <ProtectedRoute>
                    <FullScreenLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<CreateGatewayVariablePage />} />
              </Route>
              <Route
                path={RouteConfig.translations.list()}
                element={
                  <ProtectedRoute>
                    <DashboardLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<TranslationsListPage />} />
                <Route path=":language" element={<TranslationsEditPage />} />
              </Route>
            </Routes>
          </Suspense>
        </ToastProvider>
      </RoutesProvider>
    </BrowserRouter>
  );
}
