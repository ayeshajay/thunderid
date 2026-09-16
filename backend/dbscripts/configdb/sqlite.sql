-- Table to store Entity Schemas (user/agent categories)
CREATE TABLE "ENTITY_TYPES" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    ID          VARCHAR(36) NOT NULL,
    CATEGORY    VARCHAR(50) NOT NULL,
    NAME        VARCHAR(100) NOT NULL,
    OU_ID       VARCHAR(36) NOT NULL,
    ALLOW_SELF_REGISTRATION INTEGER NOT NULL DEFAULT 0,
    SCHEMA_DEF  TEXT NOT NULL,
    SYSTEM_ATTRIBUTES TEXT,
    CREATED_AT  TEXT DEFAULT (datetime('now')),
    UPDATED_AT  TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (NAME, CATEGORY, DEPLOYMENT_ID)
);

-- Composite index for deployment + category + OU-based entity type lookups
CREATE INDEX idx_entity_schemas_deployment_category_ou ON "ENTITY_TYPES" (DEPLOYMENT_ID, CATEGORY, OU_ID);

-- Table to store Roles
CREATE TABLE "ROLE" (
    DEPLOYMENT_ID           VARCHAR(255) NOT NULL,
    ID                  VARCHAR(36) NOT NULL,
    OU_ID               VARCHAR(36) NOT NULL,
    NAME                VARCHAR(50) NOT NULL,
    DESCRIPTION         VARCHAR(255),
    CREATED_AT          TEXT DEFAULT (datetime('now')),
    UPDATED_AT          TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    CONSTRAINT unique_role_ou_name UNIQUE (OU_ID, NAME, DEPLOYMENT_ID)
);

-- Composite index for deployment + OU lookups (supports UNIQUE constraint checks)
CREATE INDEX idx_role_ou_deployment ON "ROLE" (DEPLOYMENT_ID, OU_ID);

-- Table to store Role permissions
CREATE TABLE "ROLE_PERMISSION" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ROLE_ID             VARCHAR(36) NOT NULL,
    RESOURCE_SERVER_ID  VARCHAR(36) NOT NULL,
    PERMISSION          VARCHAR(1000) NOT NULL,
    CREATED_AT          TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (ROLE_ID, DEPLOYMENT_ID, RESOURCE_SERVER_ID, PERMISSION),
    FOREIGN KEY (DEPLOYMENT_ID, ROLE_ID) REFERENCES "ROLE" (DEPLOYMENT_ID, ID) ON DELETE CASCADE
);

-- Index for resource server queries with deployment isolation on ROLE_PERMISSION
CREATE INDEX idx_role_permission_resource_server ON "ROLE_PERMISSION" (RESOURCE_SERVER_ID, DEPLOYMENT_ID);

-- Table to store Role assignments (to entities and groups)
CREATE TABLE "ROLE_ASSIGNMENT" (
    DEPLOYMENT_ID       VARCHAR(255) NOT NULL,
    ROLE_ID         VARCHAR(36) NOT NULL,
    ASSIGNEE_TYPE   VARCHAR(6)  NOT NULL CHECK (ASSIGNEE_TYPE IN ('entity', 'group')),
    ASSIGNEE_ID     VARCHAR(36) NOT NULL,
    CREATED_AT      TEXT DEFAULT (datetime('now')),
    UPDATED_AT      TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (ROLE_ID, DEPLOYMENT_ID, ASSIGNEE_TYPE, ASSIGNEE_ID)
);

-- Table to store theme configurations.
CREATE TABLE "THEME" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    DISPLAY_NAME VARCHAR(255) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(512),
    THEME TEXT NOT NULL,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (DEPLOYMENT_ID, HANDLE)
);

-- Index for deployment isolation on THEME
CREATE INDEX idx_theme_deployment_id ON "THEME" (DEPLOYMENT_ID);

-- Unique index for theme handle per deployment
CREATE UNIQUE INDEX idx_theme_handle_deployment ON "THEME" (HANDLE, DEPLOYMENT_ID);

-- Table to store layout configurations.
CREATE TABLE "LAYOUT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    DISPLAY_NAME VARCHAR(255) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(512),
    LAYOUT TEXT NOT NULL,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (DEPLOYMENT_ID, HANDLE)
);

-- Index for deployment isolation on LAYOUT
CREATE INDEX idx_layout_deployment_id ON "LAYOUT" (DEPLOYMENT_ID);

-- Unique index for layout handle per deployment
CREATE UNIQUE INDEX idx_layout_handle_deployment ON "LAYOUT" (HANDLE, DEPLOYMENT_ID);

-- Table to store inbound client configurations for an entity.
CREATE TABLE "INBOUND_CLIENT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ENTITY_ID VARCHAR(36) NOT NULL,
    AUTH_FLOW_ID VARCHAR(100) NOT NULL,
    REGISTRATION_FLOW_ID VARCHAR(100),
    IS_REGISTRATION_FLOW_ENABLED CHAR(1) DEFAULT '1',
    RECOVERY_FLOW_ID VARCHAR(100),
    IS_RECOVERY_FLOW_ENABLED CHAR(1) DEFAULT '0',
    SIGNOUT_FLOW_ID VARCHAR(100),
    THEME_ID VARCHAR(36),
    LAYOUT_ID VARCHAR(36),
    PROPERTIES TEXT,
    PRIMARY KEY (DEPLOYMENT_ID, ENTITY_ID)
);

-- Index for efficient lookups by theme.
CREATE INDEX idx_inbound_client_theme_id ON "INBOUND_CLIENT"(THEME_ID);

-- Index for efficient lookups by layout.
CREATE INDEX idx_inbound_client_layout_id ON "INBOUND_CLIENT"(LAYOUT_ID);

-- Table to store OAuth inbound profile for an entity.
CREATE TABLE "OAUTH_INBOUND_PROFILE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ENTITY_ID VARCHAR(36) NOT NULL,
    OAUTH_CONFIG TEXT,
    PRIMARY KEY (ENTITY_ID, DEPLOYMENT_ID),
    FOREIGN KEY (DEPLOYMENT_ID, ENTITY_ID) REFERENCES "INBOUND_CLIENT" (DEPLOYMENT_ID, ENTITY_ID) ON DELETE CASCADE
);

-- Table to store identity providers.
CREATE TABLE "IDP" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(255) NOT NULL,
    DESCRIPTION VARCHAR(500),
    TYPE VARCHAR(20) NOT NULL,
    PROPERTIES TEXT,
    ATTRIBUTE_CONFIGURATION TEXT,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for name-based IDP lookups
CREATE INDEX idx_idp_name_deployment ON "IDP" (DEPLOYMENT_ID, NAME);

-- Expression index for issuer-based IDP lookups
CREATE INDEX idx_idp_issuer ON "IDP" (DEPLOYMENT_ID, json_extract(PROPERTIES, '$.issuer.value'));

-- Table to store notification senders.
CREATE TABLE "NOTIFICATION_SENDER" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    DESCRIPTION VARCHAR(500),
    TYPE VARCHAR(20) NOT NULL,
    PROVIDER VARCHAR(20) NOT NULL,
    PROPERTIES TEXT,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Composite index for name-based notification sender lookups
CREATE INDEX idx_notification_sender_name_deployment ON "NOTIFICATION_SENDER" (DEPLOYMENT_ID, NAME);

-- Table to store certificates associated with various entities.
CREATE TABLE "CERTIFICATE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    REF_TYPE VARCHAR(20) NOT NULL,
    REF_ID VARCHAR(36) NOT NULL,
    TYPE VARCHAR(20) NOT NULL,
    VALUE TEXT NOT NULL,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (REF_TYPE, REF_ID, DEPLOYMENT_ID)
);

-- Table to store resource servers.
CREATE TABLE "RESOURCE_SERVER" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    OU_ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    IDENTIFIER VARCHAR(2048) NOT NULL,
    TYPE VARCHAR(20) CHECK (TYPE IS NULL OR TYPE IN ('API', 'MCP', 'CUSTOM')),
    PROPERTIES TEXT,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (OU_ID, NAME, DEPLOYMENT_ID)
);

-- Composite index for name-based resource server lookups
CREATE INDEX idx_resource_server_name_deployment ON "RESOURCE_SERVER" (DEPLOYMENT_ID, NAME);

-- Unique constraint: Resource server identifier must be unique per deployment
CREATE UNIQUE INDEX uq_resource_server_identifier
    ON "RESOURCE_SERVER"(IDENTIFIER, DEPLOYMENT_ID);

-- Table to store resources within resource servers.
CREATE TABLE "RESOURCE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    RESOURCE_SERVER_ID VARCHAR(36) NOT NULL,
    PARENT_RESOURCE_ID VARCHAR(36),
    NAME VARCHAR(100) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    PROPERTIES TEXT,
    PERMISSION VARCHAR(1000) NOT NULL,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),

    PRIMARY KEY (DEPLOYMENT_ID, ID),
    FOREIGN KEY (DEPLOYMENT_ID, RESOURCE_SERVER_ID)
        REFERENCES "RESOURCE_SERVER"(DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    FOREIGN KEY (DEPLOYMENT_ID, PARENT_RESOURCE_ID)
        REFERENCES "RESOURCE"(DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
);

-- Composite index for resource server + deployment queries (list, count, and handle checks)
CREATE INDEX idx_resource_server_deployment ON "RESOURCE" (RESOURCE_SERVER_ID, DEPLOYMENT_ID);

-- Unique constraint: Resource handle must be unique under the same parent per deployment
CREATE UNIQUE INDEX uq_resource_handle_with_parent
    ON "RESOURCE"(RESOURCE_SERVER_ID, PARENT_RESOURCE_ID, HANDLE, DEPLOYMENT_ID)
    WHERE PARENT_RESOURCE_ID IS NOT NULL;

-- Unique constraint: Root-level resource handles must be unique per resource server per deployment
CREATE UNIQUE INDEX uq_resource_handle_null_parent
    ON "RESOURCE"(RESOURCE_SERVER_ID, HANDLE, DEPLOYMENT_ID)
    WHERE PARENT_RESOURCE_ID IS NULL;

-- Table to store actions at resource server or resource level.
CREATE TABLE "ACTION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    RESOURCE_SERVER_ID VARCHAR(36) NOT NULL,
    RESOURCE_ID VARCHAR(36),
    NAME VARCHAR(100) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    DESCRIPTION TEXT,
    PERMISSION VARCHAR(1000) NOT NULL,
    PROPERTIES TEXT,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),

    PRIMARY KEY (DEPLOYMENT_ID, ID),
    FOREIGN KEY (DEPLOYMENT_ID, RESOURCE_SERVER_ID)
        REFERENCES "RESOURCE_SERVER"(DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE,
    FOREIGN KEY (DEPLOYMENT_ID, RESOURCE_ID)
        REFERENCES "RESOURCE"(DEPLOYMENT_ID, ID)
        ON DELETE RESTRICT
        ON UPDATE CASCADE
);

-- Composite index for action list/count queries filtered by resource server + deployment + resource
CREATE INDEX idx_action_server_deployment ON "ACTION" (RESOURCE_SERVER_ID, DEPLOYMENT_ID, RESOURCE_ID);

-- Unique constraint: Server-level action handles must be unique per resource server per deployment
CREATE UNIQUE INDEX uq_action_server_handle
    ON "ACTION"(RESOURCE_SERVER_ID, HANDLE, DEPLOYMENT_ID)
    WHERE RESOURCE_ID IS NULL;

-- Unique constraint: Resource-level action handles must be unique per resource per deployment
CREATE UNIQUE INDEX uq_action_resource_handle
    ON "ACTION"(RESOURCE_ID, HANDLE, DEPLOYMENT_ID)
    WHERE RESOURCE_ID IS NOT NULL;

-- Table to store active flow definitions
CREATE TABLE "FLOW" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    HANDLE VARCHAR(100) NOT NULL,
    NAME VARCHAR(100) NOT NULL,
    FLOW_TYPE VARCHAR(50) NOT NULL,
    ACTIVE_VERSION INTEGER NOT NULL,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID),
    UNIQUE (HANDLE, FLOW_TYPE, DEPLOYMENT_ID)
);

-- Composite index for flow type + deployment queries
CREATE INDEX idx_flow_type_deployment ON "FLOW" (DEPLOYMENT_ID, FLOW_TYPE);

-- Table to store flow version history
CREATE TABLE "FLOW_VERSION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    FLOW_ID VARCHAR(36) NOT NULL,
    VERSION INTEGER NOT NULL,
    NODES TEXT NOT NULL,
    INTERCEPTORS TEXT,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (FLOW_ID, VERSION, DEPLOYMENT_ID),
    FOREIGN KEY (DEPLOYMENT_ID, FLOW_ID)
        REFERENCES "FLOW"(DEPLOYMENT_ID, ID)
        ON DELETE CASCADE
);

-- Table to store i18n translations
CREATE TABLE "TRANSLATION" (
    DEPLOYMENT_ID   VARCHAR(255) NOT NULL,
    MESSAGE_KEY     VARCHAR(255) NOT NULL,
    LANGUAGE_CODE   VARCHAR(10) NOT NULL,
    NAMESPACE       VARCHAR(50) NOT NULL DEFAULT 'default',
    VALUE           TEXT NOT NULL,
    CREATED_AT      TEXT DEFAULT (datetime('now')),
    UPDATED_AT      TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, NAMESPACE, MESSAGE_KEY, LANGUAGE_CODE)
);

-- Index for efficient language and namespace combination lookups
CREATE INDEX idx_translation_lang_namespace ON "TRANSLATION" (DEPLOYMENT_ID, LANGUAGE_CODE, NAMESPACE);

-- Table to store OpenID4VP presentation definitions.
CREATE TABLE "PRESENTATION_DEFINITION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    OU_ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(255),
    DESCRIPTION VARCHAR(255),
    VCT VARCHAR(512) NOT NULL,
    FORMAT VARCHAR(64) NOT NULL DEFAULT 'dc+sd-jwt',
    CLAIMS TEXT,
    ENFORCE_TRUSTED_ISSUER INTEGER,
    TRUSTED_AUTHORITIES TEXT,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Each presentation definition handle is unique per deployment.
CREATE UNIQUE INDEX idx_openid4vp_pd_handle ON "PRESENTATION_DEFINITION" (DEPLOYMENT_ID, HANDLE);

-- Table to store OpenID4VCI credential configurations.
CREATE TABLE "CREDENTIAL_CONFIGURATION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID VARCHAR(36) NOT NULL,
    HANDLE VARCHAR(255) NOT NULL,
    OU_ID VARCHAR(36) NOT NULL,
    NAME VARCHAR(255),
    DESCRIPTION VARCHAR(255),
    FORMAT VARCHAR(64) NOT NULL DEFAULT 'dc+sd-jwt',
    VCT VARCHAR(512) NOT NULL,
    CLAIMS TEXT,
    DISPLAY TEXT,
    VALIDITY_SECONDS INTEGER,
    CREATED_AT TEXT DEFAULT (datetime('now')),
    UPDATED_AT TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Each credential configuration handle is unique per deployment.
CREATE UNIQUE INDEX idx_openid4vci_cc_handle ON "CREDENTIAL_CONFIGURATION" (DEPLOYMENT_ID, HANDLE);

-- Table to store server-wide configuration
CREATE TABLE "SERVER_CONFIG" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    VALUE         TEXT         NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, NAME)
);

-- Registry of resources this deployment does not own. A Data Plane records here everything the
-- Control Plane wrote to it through the import API, so its own management APIs can refuse to change
-- them: a resource edited on both planes is silently overwritten by the next promotion. Only the
-- import writes and clears these rows.
CREATE TABLE "MANAGED_RESOURCE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    RESOURCE_TYPE VARCHAR(64)  NOT NULL,
    RESOURCE_ID   VARCHAR(255) NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID)
);
-- Every write is checked against this registry, so the lookup has to be cheap.
CREATE INDEX idx_managed_resource_deployment ON "MANAGED_RESOURCE" (DEPLOYMENT_ID);

-- Configuration as it was authored, before any gateway filled in what it owns.
--
-- A control plane stores the document the caller wrote, with a reference in place of every field
-- that belongs to the gateway the configuration is applied to, such as a client secret or a redirect
-- URI. The payload is therefore never runnable on its own, which is the point: it is a design, and
-- each gateway resolves the references from its own variable and secret stores at import.
--
-- A resource is addressed by type and name rather than by id, because a name is what an author
-- writes and what survives being applied to many gateways, where the ids differ.
CREATE TABLE "PARAMETERISED_RESOURCE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  PRIMARY KEY,
    RESOURCE_TYPE VARCHAR(64)  NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    PAYLOAD       TEXT         NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    CONSTRAINT unique_parameterised_resource UNIQUE (DEPLOYMENT_ID, RESOURCE_TYPE, NAME)
);
-- Listing is always scoped to one deployment and ordered by type then name.
CREATE INDEX idx_parameterised_resource_deployment ON "PARAMETERISED_RESOURCE" (DEPLOYMENT_ID, RESOURCE_TYPE, NAME);

-- Registry of tenants managed by the platform "system" tenant. Owned by the system deployment
-- (DEPLOYMENT_ID = the system/root deployment id); TENANT_ID is the managed tenant's deployment id.
CREATE TABLE "TENANT" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  PRIMARY KEY,
    TENANT_ID     VARCHAR(255) NOT NULL,
    NAME          VARCHAR(255),
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    CONSTRAINT unique_tenant_id UNIQUE (DEPLOYMENT_ID, TENANT_ID)
);
CREATE INDEX idx_tenant_deployment ON "TENANT" (DEPLOYMENT_ID);

-- The credential a data plane presents when it dials this control plane's channel.
--
-- Keyed by DATA_PLANE_ID alone, not by (DEPLOYMENT_ID, DATA_PLANE_ID) like the tenant-scoped tables:
-- the handshake is authenticated before any tenant context exists, so the lookup cannot be scoped by
-- one. A data plane id is already unique across a control plane, because the connection registry
-- keys by it. DEPLOYMENT_ID records which tenant the gateway belongs to.
--
-- TOKEN holds the ciphertext. It is encrypted with the configuration crypto service, the same one
-- that protects connection secrets, so it is unreadable from a database dump alone.
CREATE TABLE "DATA_PLANE_TOKEN" (
    DATA_PLANE_ID VARCHAR(255) PRIMARY KEY,
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    TOKEN         TEXT         NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now'))
);
CREATE INDEX idx_data_plane_token_deployment ON "DATA_PLANE_TOKEN" (DEPLOYMENT_ID);


-- Credentials this deployment holds, backing the "kv:NAME" references configuration carries.
--
-- A credential never travels with configuration: it is promoted as a placeholder and the value is set
-- against the deployment that needs it. This is where it lands. Names are unique per deployment,
-- because a reference names a credential by name alone.
--
-- VALUE holds the ciphertext, encrypted with the configuration crypto service, so a database dump
-- reveals no credential. KIND records whether the plaintext is the credential itself or a one-way
-- hash of it, and ALGORITHM and PARAMETERS carry what verifying a hash needs.
CREATE TABLE "SECRET" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  PRIMARY KEY,
    NAME          VARCHAR(255) NOT NULL,
    KIND          VARCHAR(10)  NOT NULL CHECK (KIND IN ('hash', 'value')),
    VALUE         TEXT         NOT NULL,
    ALGORITHM     VARCHAR(50),
    PARAMETERS    TEXT,
    DESCRIPTION   VARCHAR(255),
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    CONSTRAINT unique_secret_name UNIQUE (DEPLOYMENT_ID, NAME)
);
CREATE INDEX idx_secret_deployment ON "SECRET" (DEPLOYMENT_ID);

-- ----------------------------------------------------------------------------
-- Gateways an organization's configuration is applied to, and the work queued for them.
--
-- A gateway is a resource of the organization, so these live here with the rest of its configuration
-- rather than in a database of their own. DEPLOYMENT_ID is the organization.
-- ----------------------------------------------------------------------------
-- Gateways configuration is promoted through, one row per gateway.
--
-- DATA is the gateway document. Nothing queries inside it: an organization has a handful of
-- gateways, they are read as a set, and ordering and rank are resolved in the server.
CREATE TABLE "GATEWAY" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  NOT NULL,
    DATA          TEXT         NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Configuration versions captured from the organization, which an apply writes onto a gateway.
--
-- A version belongs to the organization, not to a gateway. What a capture reads is the
-- organization's configuration as authored on the control plane, which is the same whichever
-- gateway it is later applied to, so there is one stream per organization and a gateway holds none
-- of its own. SEQ rises by one within the organization and orders that stream.
CREATE TABLE "VERSION" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    SEQ           INTEGER      NOT NULL,
    DATA          TEXT         NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, SEQ)
);

-- What each gateway has run, one row per apply.
--
-- A gateway's own history is this: the organization versions that have been applied to it, in the
-- order they were. It is what "go back to what this gateway was running before" reads, and what
-- keeps a version from being pruned while some gateway can still return to it.
--
-- ORDINAL rises by one per gateway, so the newest row is the version the gateway is running.
CREATE TABLE "GATEWAY_APPLY" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    GATEWAY_ID    VARCHAR(36)  NOT NULL,
    ORDINAL       INTEGER      NOT NULL,
    SEQ           INTEGER      NOT NULL,
    APPLIED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, GATEWAY_ID, ORDINAL),
    CONSTRAINT fk_gateway_apply_gateway
        FOREIGN KEY (DEPLOYMENT_ID, GATEWAY_ID) REFERENCES "GATEWAY" (DEPLOYMENT_ID, ID)
        ON DELETE CASCADE
);

CREATE INDEX idx_gateway_apply_gateway ON "GATEWAY_APPLY" (DEPLOYMENT_ID, GATEWAY_ID);

-- Non-secret gateway variables, held per gateway. KEY is the declarative placeholder the
-- value resolves (e.g. MY_APP_REDIRECT_URL); VALUE is stored in plaintext because it carries no
-- confidential material.
--
-- A variable belongs to one gateway of the organization, because its value is a property of the
-- deployment it is applied to: a redirect URL differs between dev and prod even though the
-- configuration referring to it is the same. Deleting a gateway takes its variables with it.
CREATE TABLE "GATEWAY_VARIABLE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(36)  PRIMARY KEY,
    GATEWAY_ID        VARCHAR(36)  NOT NULL,
    KEY           VARCHAR(255) NOT NULL,
    VALUE         TEXT         NOT NULL,
    DESCRIPTION   VARCHAR(255),
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    CONSTRAINT unique_gateway_variable_key UNIQUE (DEPLOYMENT_ID, GATEWAY_ID, KEY),
    CONSTRAINT fk_gateway_variable_gateway
        FOREIGN KEY (DEPLOYMENT_ID, GATEWAY_ID) REFERENCES "GATEWAY" (DEPLOYMENT_ID, ID)
        ON DELETE CASCADE
);

CREATE INDEX idx_gateway_variable_gateway ON "GATEWAY_VARIABLE" (DEPLOYMENT_ID, GATEWAY_ID);

-- Work queued for a Data Plane, and what it answered.
--
-- A Control Plane pod can only speak to the Data Planes that dialled it, so a request arriving at a
-- pod holding no link would otherwise fail. Every request is written here first and then delivered:
-- by this pod when it holds a link, or by whichever pod does. The caller is handed the id and reads
-- the answer back from this table, from any pod.
--
-- Rows are never deleted here. Pruning is a separate concern.
CREATE TABLE "DATA_PLANE_JOB" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    ID            VARCHAR(64)  NOT NULL,
    -- The deployment this is for, as "<org>:<env>". DEPLOYMENT_ID above is the organization, so that
    -- an organization's queue sits in one partition with its gateways.
    DATA_PLANE_ID VARCHAR(255) NOT NULL,
    GATEWAY_ID        VARCHAR(64),
    -- What to do: "import" applies configuration, "secret_put" stores one credential.
    TYPE          VARCHAR(32)  NOT NULL,
    -- The request, as JSON. Encrypted when it carries a credential, which is what ENCRYPTED records:
    -- a secret is held here only until it is delivered, and never in the clear.
    PAYLOAD       TEXT         NOT NULL,
    ENCRYPTED     CHAR(1)      DEFAULT '0' NOT NULL,
    -- pending -> claimed -> done | failed.
    STATUS        VARCHAR(16)  NOT NULL,
    -- Which pod is delivering it, for diagnosing a job stuck in claimed.
    CLAIMED_BY    VARCHAR(255),
    -- What the Data Plane answered, as JSON, or why it could not be delivered.
    RESULT        TEXT,
    ERROR         TEXT,
    ATTEMPTS      INTEGER      DEFAULT 0 NOT NULL,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    COMPLETED_AT  TEXT,
    PRIMARY KEY (DEPLOYMENT_ID, ID)
);

-- Claiming reads the oldest pending row for one Data Plane, and checks whether that Data Plane
-- already has one in flight.
CREATE INDEX idx_data_plane_job_queue ON "DATA_PLANE_JOB" (DATA_PLANE_ID, STATUS, CREATED_AT);
