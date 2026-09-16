// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Control Plane console runtime configuration.
//   - plane: 'cp'    : authoring surface. DashboardLayout hides Data Plane runtime entries.
//   - server         : management API base -> Control Plane backend (:8095).
//   - trusted_issuer : OAuth/OIDC auth base -> external IdP, which issues tokens.
window.__THUNDERID_RUNTIME_CONFIG__ = {
  plane: 'cp',
  brand: {
    product_name: 'ThunderID',
    documentation: {
      baseUrl: 'https://thunderid.dev/docs/next',
      releasesUrl: 'https://thunderid.dev/data/releases.json',
    },
    favicon: {
      light: 'assets/images/favicon.ico',
      dark: 'assets/images/favicon-inverted.ico',
    },
  },
  client: {
    base: '/console',
    client_id: 'FlMp9nOYYkqwRierCHBlJfdwsaYa',
    resource_identifier: 'https://localhost:8095',
    // System scopes carry the tenant_instance prefix, matching security.system_permission_prefix in
    // the Control Plane's deployment.yaml. A scope names the instance it grants against, so a bare
    // "system" from the trusted issuer would not satisfy the permissions this plane checks.
    scopes: [
      'openid',
      'profile',
      'email',
      'ou',
      'tenant_instance:system',
      'tenant_instance:system:user',
      'tenant_instance:system:group',
      'tenant_instance:system:ou:view',
      'tenant_instance:system:usertype:view',
    ],
  },
  // The management API is this same server: the Control Plane serves this console. Leaving the URL
  // unset makes every call same-origin, whichever host it is reached on, so no cross-origin request
  // is ever made and none has to be allowed. Setting it to one fixed host would instead break the
  // console on every other name the server answers to.
  server: {},
  // The environment manager, which provides promotion. Promotion is not part of the product: a
  // gateway is a flat resource of the organization here, and which gateway may be promoted into
  // which comes from the organization's own environment hierarchy, held by this service. Leave the
  // URL empty to serve a console with no promotion views at all.
  env_manager: {
    public_url: '',
  },
  trusted_issuer: {
    type: 'generic',
    public_url: 'https://api.asgardeo.io/t/b1zt/oauth2/token',
    client_id: 'FlMp9nOYYkqwRierCHBlJfdwsaYa',
    scopes: ['openid', 'profile', 'email', 'tenant_instance:system'],
  },
};
