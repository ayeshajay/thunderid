// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** How a configuration version came to exist. */
export type VersionOrigin = 'captured' | 'uploaded';

/** How a resource differs between two configuration versions. */
export type ChangeType = 'added' | 'updated' | 'deleted' | 'unchanged';

/** An gateway in the promotion chain. */
export interface Gateway {
  id: string;
  name: string;
  appliedSeq: number;
  /**
   * Resource keys a user chose not to promote into this gateway. The choice is remembered, so a
   * later promotion holds them back by default until they are deliberately selected again.
   */
  excluded?: string[];
  latestSeq: number;
  hasPendingChanges: boolean;
  /**
   * Whether the Control Plane administers this gateway directly rather than only promoting into
   * it. Editing configuration in the organization's workspace is editing this gateway, and a
   * credential created there is issued against it. Exactly one gateway holds this.
   */
  managedByControlPlane?: boolean;
  /**
   * Whether this gateway's Data Plane is currently connected. The Data Plane dials the Control
   * Plane and holds that connection open, so nothing can be applied or promoted to one that is not
   * connected.
   */
  dataPlane: DataPlaneStatus;
  createdAt: string;
  updatedAt: string;
}

/** Whether an gateway's Data Plane is connected, and when it was last heard from. */
export interface DataPlaneStatus {
  connected: boolean;
  lastSeen?: string;
}

export interface GatewayListResponse {
  gateways: Gateway[];
  /**
   * Whether this caller holds the promotion scope. Promotion is a release decision, so it is gated
   * where every other gateway action is not; the console leaves the action out rather than
   * offering it and having the request refused.
   */
  canPromote?: boolean;
}

/** A stored configuration snapshot for an gateway. */
export interface Version {
  seq: number;
  origin: VersionOrigin;
  note?: string;
  createdAt: string;
}

export interface VersionListResponse {
  versions: Version[];
}

/** One entry in a gateway's history: an organization version it ran, and when. */
export interface GatewayApply {
  ordinal: number;
  seq: number;
  appliedAt: string;
}

export interface GatewayHistoryResponse {
  history: GatewayApply[];
}

/** One line of a unified diff. Kind is ' ' (context), '+' (added) or '-' (removed). */
export interface LineOp {
  kind: string;
  text: string;
}

/** How a single resource differs between two versions. */
export interface ResourceChange {
  key: string;
  type: string;
  id?: string;
  name?: string;
  category?: string;
  change: ChangeType;
  lines?: LineOp[];
}

export interface DiffSummary {
  added: number;
  updated: number;
  deleted: number;
  unchanged: number;
}

export interface Diff {
  changes: ResourceChange[];
  summary: DiffSummary;
}

export interface ImportOutcome {
  resourceType: string;
  resourceId: string;
  resourceName: string;
  operation: string;
  status: string;
  message?: string;
}

/** The outcome of writing a bundle through the import API. */
export interface ImportResponse {
  summary?: {totalDocuments: number; imported: number; deleted?: number; failed: number};
  results?: ImportOutcome[];
}

export interface ApplyResult {
  targetSeq: number;
  diff: Diff;
  dryRun: boolean;
  import?: ImportResponse;
  /**
   * Identifies the queued work. An apply is delivered by the Control Plane pod holding the data
   * plane's connection, which is not always the one that took the request.
   */
  jobId: string;
  /**
   * "done" when the data plane has taken the configuration, in which case import is set. "pending"
   * means it is queued for another pod and the outcome is read back with jobId.
   */
  status: DataPlaneJobStatus;
}

/** How far a piece of queued work has got. */
export type DataPlaneJobStatus = 'pending' | 'claimed' | 'done' | 'failed';

/** Work queued for a data plane, and what it answered once delivered. */
export interface DataPlaneJob {
  id: string;
  dataPlaneId: string;
  gatewayId?: string;
  type: string;
  status: DataPlaneJobStatus;
  /** The data plane's answer, as JSON, once the status is "done". */
  result?: string;
  /** Why the delivery failed, when the status is "failed". */
  error?: string;
  attempts: number;
}

export interface PromoteResult {
  preview: Diff;
  /** The organization version the target was moved onto. Promoting creates no version. */
  seq: number;
  applied?: ApplyResult;
}

export interface RevertResult {
  preview: Diff;
  /** The organization version the gateway was moved back onto. Reverting creates no version. */
  seq: number;
  applied?: ApplyResult;
}

/** How an gateway's next apply would resolve its placeholders. */
export interface VariableStatus {
  gatewayId: string;
  seq: number;
  required: string[];
  missing: string[];
  secretBacked: string[];
  /** Secret placeholders the Data Plane's secret service does not hold. */
  missingSecrets: string[];
  /** False when the secret service could not be consulted, so missingSecrets is not a judgement. */
  secretsChecked: boolean;
}

/** One gateway's outcome from applying across every gateway. */
export interface ApplyAllResult {
  gatewayId: string;
  gatewayName: string;
  applied?: ApplyResult;
  error?: string;
}

/**
 * How a credential has to be held on the Data Plane.
 *
 * A credential that is only ever checked against what a caller presents, such as an application's
 * client secret, is stored as a one-way hash and can never be read back. One the Data Plane replays to
 * a third party, such as an SMS gateway key, has to stay readable. This is decided by the configuration
 * that uses the credential, not by whoever sets it.
 */
export type SecretKind = 'hash' | 'value';

/** One secret-backed placeholder of an gateway. */
export interface SecretEntry {
  name: string;
  /** The resource field it fills, e.g. clientSecret. */
  field?: string;
  resourceType?: string;
  resourceName?: string;
  kind: SecretKind;
  /** Whether the Data Plane's secret service holds it. Meaningless when the list's checked is false. */
  held: boolean;
}

/** Every credential an gateway needs, with its status on the Data Plane. */
export interface SecretList {
  gatewayId: string;
  seq: number;
  secrets: SecretEntry[];
  /** False when the secret service could not be reached, so held is not a judgement. */
  checked: boolean;
  /** Why it could not be reached. Usually this gateway's own credentials or endpoint. */
  checkError?: string;
  /**
   * Set when the Control Plane pod serving this request holds no connection to the data plane and
   * queued the question for one that does. Following it and asking again is what turns `checked`
   * true; it means "not yet", not "unavailable".
   */
  pendingJobId?: string;
}

/** The result of regenerating a credential. The value is returned only here. */
export interface RegeneratedSecret {
  secret: SecretEntry;
  value: string;
}
