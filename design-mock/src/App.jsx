import React, { Fragment, createContext, useContext, useEffect, useRef, useState } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";

/*
  HomeSignal product skeleton

  This mock is intentionally data-model aware. UI rows marked with a warning
  badge are not directly backed by the current architecture/data-model docs, or
  are future/conditional surfaces. The backing map is derived from:

  - service-map.md table families
  - account-site-service.md customer/site fields
  - enrollment-claiming-contract.md pairing/claim payloads
  - edge-state-adapter.md homesignal_edge shadow shape
  - telemetry-ingest-architecture.md latest/hot/cold telemetry posture
*/

const schema = {
  backed: "Backed by current architecture/data model",
  partial: "Partially defined; implementation needs field-level contract",
  conditional: "Allowed only through an approved owning flow",
  future: "Future or productized later",
  missing: "Not defined in current architecture/data model",
};

const navGroups = [
  {
    title: "Entry flows",
    items: ["Login", "Sign Up", "Password Reset"],
  },
  {
    title: "Operations",
    items: ["Dashboard", "Devices", "Sites", "Backups", "Updates", "Alerts", "Activity"],
  },
  {
    title: "Management",
    items: ["Users", "Account Settings"],
  },
  {
    title: "Internal",
    items: ["Internal Admin", "Internal Diagnostics", "Internal Audit", "HA App", "Schema Coverage"],
  },
];

const defaultRoute = {
  page: "Dashboard",
  site: "site_123",
  device: "dev_123",
  haApp: "onboarding",
  wiring: "off",
};

const DataWiringContext = createContext(false);

const autoPairingStorageKey = "homesignal.auto_pairing";
const homeAssistantUrlStorageKey = "homesignal.hass_url";
const mockHaAppPairingStateKey = "homesignal.mock_ha_app_pairing_state";
const mockHaAppBootstrapStateKey = "homesignal.mock_ha_app_bootstrap_state";
const autoPairingBridgePath = "/ha_app_pairing?bridge=1";
const defaultHomeAssistantUrl = "http://192.168.1.3";
const haAppRepositoryDeepLink = "https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fhomesignal-io%2Fhomesignal-home-assistant-app";
const installedHaAppPath = "/hassio/addon/fdd5d111_homesignal/info";
const haAppBridgePath = "/homesignal/ha-app-bridge";
const mockHaAppBridgePath = "/ha_app_bridge_mock";
const autoPairingContextTtlMs = 2 * 60 * 60 * 1000;
const haAppBridgeProbeTimeoutMs = 3000;
const haAppBridgeAutoContinueDelayMs = 1400;
const haAppBootstrapLoadingMaxMs = 3000;
const haAppBootstrapHardStopMs = 10000;

function isLocalMockHost() {
  if (typeof window === "undefined") return false;
  return ["127.0.0.1", "localhost"].includes(window.location.hostname);
}

function readAutoPairingContext() {
  if (typeof window === "undefined") return null;

  try {
    const raw = window.localStorage.getItem(autoPairingStorageKey);
    if (!raw) return null;

    const value = JSON.parse(raw);
    if (!value?.pairing_id || !value?.stored_at || !value?.auto_pairing_exp) {
      window.localStorage.removeItem(autoPairingStorageKey);
      return null;
    }

    if (Date.parse(value.auto_pairing_exp) <= Date.now()) {
      window.localStorage.removeItem(autoPairingStorageKey);
      return null;
    }

    return value;
  } catch {
    window.localStorage.removeItem(autoPairingStorageKey);
    return null;
  }
}

function writeAutoPairingContext(pairingId) {
  const now = new Date();
  const value = {
    pairing_id: pairingId,
    stored_at: now.toISOString(),
    auto_pairing_exp: new Date(now.getTime() + autoPairingContextTtlMs).toISOString(),
  };
  window.localStorage.setItem(autoPairingStorageKey, JSON.stringify(value));
  return value;
}

function removeAutoPairingContext() {
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(autoPairingStorageKey);
  }
}

async function copyTextToClipboard(text) {
  if (!text) return false;

  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.setAttribute("readonly", "");
    textArea.style.position = "fixed";
    textArea.style.left = "-9999px";
    document.body.appendChild(textArea);
    textArea.select();
    const copied = document.execCommand("copy");
    document.body.removeChild(textArea);
    return copied;
  }
}

function readStoredHomeAssistantUrl() {
  if (typeof window === "undefined") return "";

  try {
    return window.localStorage.getItem(homeAssistantUrlStorageKey) || "";
  } catch {
    return "";
  }
}

function writeStoredHomeAssistantUrl(url) {
  if (typeof window === "undefined") return;

  const trimmedUrl = url.trim().replace(/\/$/, "");
  if (!trimmedUrl) {
    window.localStorage.removeItem(homeAssistantUrlStorageKey);
    return;
  }

  window.localStorage.setItem(homeAssistantUrlStorageKey, trimmedUrl);
}

function readMockHaAppPairingState() {
  if (typeof window === "undefined") {
    return { has_ever_paired: false, last_pairing_id: null, paired_at: null };
  }

  try {
    const raw = window.localStorage.getItem(mockHaAppPairingStateKey);
    if (!raw) return { has_ever_paired: false, last_pairing_id: null, paired_at: null };
    const value = JSON.parse(raw);
    return {
      has_ever_paired: Boolean(value?.has_ever_paired),
      last_pairing_id: value?.last_pairing_id || null,
      paired_at: value?.paired_at || null,
    };
  } catch {
    return { has_ever_paired: false, last_pairing_id: null, paired_at: null };
  }
}

function writeMockHaAppPairingState(nextState) {
  if (typeof window === "undefined") return nextState;
  window.localStorage.setItem(mockHaAppPairingStateKey, JSON.stringify(nextState));
  return nextState;
}

function readMockHaAppBootstrapState() {
  if (typeof window === "undefined") {
    return { has_run_bootstrap: false, last_checked_at: null };
  }

  try {
    const raw = window.localStorage.getItem(mockHaAppBootstrapStateKey);
    if (!raw) return { has_run_bootstrap: false, last_checked_at: null };
    const value = JSON.parse(raw);
    return {
      has_run_bootstrap: Boolean(value?.has_run_bootstrap),
      last_checked_at: value?.last_checked_at || null,
    };
  } catch {
    return { has_run_bootstrap: false, last_checked_at: null };
  }
}

function writeMockHaAppBootstrapState(nextState) {
  if (typeof window === "undefined") return nextState;
  window.localStorage.setItem(mockHaAppBootstrapStateKey, JSON.stringify(nextState));
  return nextState;
}

function clearMockHaAppLocalState() {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(mockHaAppPairingStateKey);
  window.localStorage.removeItem(mockHaAppBootstrapStateKey);
  window.localStorage.removeItem(autoPairingStorageKey);
}

// Data wiring hints shown by the wiring overlay.
// Numeric IDs are stable conversation handles. Keep hints terse and schema-led:
// name the fields/columns needed, with light napkin math only when computed.
const wiringHints = {
  "1": { status: "backed", text: "Fields: presence, backup_status, ha_app_version, latest_ha_app_version, ha_version, latest_ha_version. Calc: any unhealthy/drift => action_required." },
  "2": { status: "backed", text: "Fields: site_id, device_id. Calc: distinct affected site_id / total site_id." },
  "3": { status: "backed", text: "Fields: dashboard_state, primary_issue, online_count, managed_site_count. Calc: API returns summary; UI renders copy from that shape." },
  "4": { status: "backed", text: "Fields: site_id. Calc: count managed sites." },
  "5": { status: "backed", text: "Fields: presence, device_id. Calc: online devices / total devices." },
  "6": { status: "backed", text: "Fields: issue_projection[]. Calc: count visible v0 issue rows." },
  "7": { status: "backed", text: "Fields: issue_projection[].site_id. Calc: distinct sites with issue_count > 0." },
  "8": { status: "backed", text: "Fields: backup_status. Calc: count failed backups." },
  "9": { status: "backed", text: "Fields: ha_app_version, latest_ha_app_version. Calc: installed != latest." },
  "10": { status: "backed", text: "Fields: alert_recipients, recipient_status, enabled_subscriptions, site_scope. Calc: configured/verified recipient coverage." },
  "11": { status: "backed", text: "Fields: site_name." },
  "12": { status: "backed", text: "Fields: customer_display_name, service_address_city, service_address_region. Display may choose city/state, but address data exists." },
  "13": { status: "backed", text: "Fields: issue_projection[].label, severity, sort_priority. Calc: first issue by sort_priority." },
  "14": { status: "backed", text: "Fields: issue_projection[].detail. API formats detail from backing facts." },
  "15": { status: "backed", text: "Fields: issue_projection[].issue_code. Calc: count issues for site/device." },
  "16": { status: "backed", text: "Fields: issue_projection[].label." },
  "17": { status: "backed", text: "Fields: issue_projection[].detail." },
  "18": { status: "backed", text: "Fields: site_name." },
  "19": { status: "backed", text: "Fields: customer_display_name, service_address_city, service_address_region. Display may choose city/state, but address data exists." },
  "20": { status: "backed", text: "Fields: presence. Calc: online => Connected, else Disconnected." },
  "21": { status: "backed", text: "Fields: ha_version, backup_status." },
  "22": { status: "backed", text: "Fields: ha_version, latest_ha_version from daily cloud catalog/cache. Hide advisory if catalog is unavailable." },
  "23": { status: "backed", text: "Fields: issue_projection[].primary_action. Calc: any actionable issue => Review; otherwise View." },
  "24": { status: "backed", text: "Fields: device_id, site_id. Calc: count devices across count sites." },
  "25": { status: "backed", text: "Fields: activity_feed[].occurred_at." },
  "26": { status: "backed", text: "Fields: activity_feed[].action." },
  "27": { status: "backed", text: "Fields: activity_feed[].subject_type, subject_id, subject_label." },
  "28": { status: "backed", text: "Fields: activity_feed[].detail." },
  "29": { status: "backed", text: "Fields: activity_feed[].category: alert, backup, device, update, enrollment, account." },
  "30": { status: "backed", text: "Fields: site_category optional. UI defaults to standard Home Assistant/site icon when absent; presentation only." },
  "31": { status: "backed", text: "Fields: issue_projection[].severity: critical, warning, info." },
  "32": { status: "backed", text: "Fields: ha_app_version, latest_ha_app_version. Calc: installed app version differs from latest desired/reported release version." },
};

const pages = new Set([
  "Login",
  "Sign Up",
  "Password Reset",
  "Dashboard",
  "Devices",
  "Device Detail",
  "Sites",
  "Enrollment",
  "Backups",
  "Updates",
  "Alerts",
  "Activity",
  "Users",
  "Account Settings",
  "Internal Admin",
  "Internal Diagnostics",
  "Internal Audit",
  "HA App",
  "Schema Coverage",
]);

const authPages = new Set(["Login", "Sign Up", "Password Reset"]);

function readRouteHash() {
  if (typeof window === "undefined") {
    return defaultRoute;
  }

  const routeDefault = window.location.pathname === installedHaAppPath
    ? { ...defaultRoute, page: "HA App", haApp: "onboarding" }
    : defaultRoute;
  const hash = window.location.hash.replace(/^#/, "");
  const params = new URLSearchParams(hash);
  const page = params.get("page");
  const site = params.get("site");
  const device = params.get("device");
  const haApp = params.get("ha_app");
  const wiring = params.get("wiring");

  return {
    page: pages.has(page) ? page : routeDefault.page,
    site: sites.some((item) => item.id === site) ? site : routeDefault.site,
    device: devices.some((item) => item.id === device) ? device : routeDefault.device,
    haApp: Object.prototype.hasOwnProperty.call(haAppScreens, haApp) || Object.prototype.hasOwnProperty.call(haAppStatusStates, haApp) ? haApp : routeDefault.haApp,
    wiring: wiring === "on" ? "on" : "off",
  };
}

function routeToHash(route) {
  const params = new URLSearchParams();
  params.set("page", route.page);
  params.set("site", route.site);
  params.set("device", route.device);
  params.set("ha_app", route.haApp);
  if (route.wiring === "on") {
    params.set("wiring", "on");
  }
  return `#${params.toString()}`;
}

const modelCoverage = {
  accounts: "backed",
  users: "backed",
  roles: "backed",
  customers: "backed",
  sites: "backed",
  buildings: "backed",
  zones: "backed",
  devices: "backed",
  device_credentials: "backed",
  device_claim_invites: "backed",
  device_claim_verifications: "backed",
  device_claim_invite_email_deliveries: "backed",
  device_presence: "backed",
  device_latest_state: "backed",
  device_lifecycle_events: "backed",
  device_telemetry_events: "backed",
  device_desired_state: "backed",
  device_edge_state_projection: "backed",
  commands: "backed",
  command_results: "backed",
  backups: "backed",
  diagnostic_bundles: "conditional",
  remote_access_links: "backed",
  alerts: "future",
  alert_events: "future",
  platform_health_findings: "future",
  release_channels: "backed",
  release_artifacts: "backed",
  rollouts: "backed",
  device_update_assignments: "backed",
  audit_events: "backed",
  topology_snapshots: "future",
  billing_subscriptions: "missing",
  customer_login_accounts: "missing",
  live_event_streams: "future",
};

const account = {
  id: "acct_123",
  name: "Northstar Smart Homes",
  type: "integrator",
  status: "active",
};

const customers = [
  {
    id: "cust_101",
    account_id: "acct_123",
    display_name: "John Smith",
    email: "john.smith@example.com",
    phone: "(555) 010-2214",
    notes: "Primary contact for Smith Residence.",
    status: "active",
    created_at: "2026-05-01T15:22:00Z",
    updated_at: "2026-05-13T18:10:00Z",
    archived_at: null,
  },
  {
    id: "cust_102",
    account_id: "acct_123",
    display_name: "Maya Lee",
    email: "maya.lee@example.com",
    phone: null,
    notes: "Primary contact for Lee Residence.",
    status: "active",
    created_at: "2026-05-02T12:04:00Z",
    updated_at: "2026-05-12T20:30:00Z",
    archived_at: null,
  },
];

const sites = [
  {
    id: "site_123",
    name: "Smith Residence",
    site_category: "residential",
    account_id: "acct_123",
    customer_record_id: "cust_101",
    status: "active",
    service_address: "14 Maple Lane, Raleigh, NC",
    location: "Raleigh, NC",
    created_at: "2026-05-01T15:24:00Z",
    updated_at: "2026-05-13T18:20:00Z",
    archived_at: null,
    buildings: [{ id: "bldg_1", name: "Main House" }],
    zones: [{ id: "zone_1", building_id: "bldg_1", name: "Whole Home" }],
  },
  {
    id: "site_124",
    name: "Lee Residence",
    site_category: "residential",
    account_id: "acct_123",
    customer_record_id: "cust_102",
    status: "active",
    service_address: "99 Lake Road, Cary, NC",
    location: "Cary, NC",
    created_at: "2026-05-02T12:07:00Z",
    updated_at: "2026-05-12T20:30:00Z",
    archived_at: null,
    buildings: [{ id: "bldg_2", name: "Main House" }],
    zones: [{ id: "zone_2", building_id: "bldg_2", name: "Whole Home" }],
  },
];

const devices = [
  {
    id: "dev_123",
    thing_name: "dev_123",
    site_id: "site_123",
    zone_id: "zone_1",
    status: "claimed",
    lifecycle_status: "active",
    presence: "online",
    last_seen_at: "2026-05-14T09:21:04Z",
    ha_instance_uuid: "ha_8f1db7b1d4fb4c12a2c4b0fcb4df8e5a",
    machine_id: "2f8f8c8e2c7f4f89a2a1f5a9e6d3b111",
    hostname: "homeassistant",
    os_type: "Home Assistant OS",
    home_assistant_version: "2026.5.0",
    latest_home_assistant_version: "2026.5.1",
    supervisor_version: "2026.05.0",
    latest_supervisor_version: "2026.05.0",
    ha_app_version: "0.1.4",
    latest_ha_app_version: "0.1.4",
    storage_status: "healthy",
    cloud_connection: "connected",
    credential_status: "active",
  },
  {
    id: "dev_124",
    thing_name: "dev_124",
    site_id: "site_124",
    zone_id: "zone_2",
    status: "claimed",
    lifecycle_status: "active",
    presence: "degraded",
    last_seen_at: "2026-05-14T07:15:11Z",
    ha_instance_uuid: "ha_d7ef01b3679c49b6bb7a1d0a6fb98b04",
    machine_id: "71b0bf90476a4a2e8576b5d412cb5618",
    hostname: "homeassistant",
    os_type: "Home Assistant OS",
    home_assistant_version: "2026.5.0",
    latest_home_assistant_version: "2026.5.1",
    supervisor_version: "2026.05.0",
    latest_supervisor_version: "2026.05.0",
    ha_app_version: "0.1.3",
    latest_ha_app_version: "0.1.4",
    storage_status: "warning",
    cloud_connection: "connected",
    credential_status: "active",
  },
];

const edgeState = {
  desired: {
    publish_policy: {
      version: "ppv_123",
      ref: "publish-policies/ppv_123",
      refresh_after: "2026-05-14T12:00:00Z",
      expires_at: "2026-05-20T12:00:00Z",
      telemetry_cadence_seconds: 3600,
      enabled_event_families: ["app_alarm"],
    },
    update: {
      desired_version: "0.1.4",
      channel: "stable",
      rollout_id: "rollout_456",
    },
  },
  reported: {
    publish_policy: {
      active_version: "ppv_123",
      status: "applied",
      applied_at: "2026-05-13T12:05:00Z",
    },
    update: {
      current_version: "0.1.4",
      status: "applied",
      reported_at: "2026-05-13T12:10:00Z",
    },
  },
};

const backups = [
  {
    id: "backup_1001",
    site_id: "site_123",
    device_id: "dev_123",
    status: "succeeded",
    last_success_at: "2026-05-13T05:00:00Z",
    last_failure_at: null,
    artifact_status: "stored",
    size_mb: 184,
  },
  {
    id: "backup_1002",
    site_id: "site_124",
    device_id: "dev_124",
    status: "failed",
    last_success_at: "2026-05-10T05:00:00Z",
    last_failure_at: "2026-05-14T05:02:00Z",
    artifact_status: "none",
    size_mb: null,
  },
];

const commands = [
  {
    id: "cmd_9001",
    device_id: "dev_123",
    type: "trigger_backup",
    status: "succeeded",
    issued_at: "2026-05-13T04:59:00Z",
    result_at: "2026-05-13T05:00:00Z",
  },
  {
    id: "cmd_9002",
    device_id: "dev_124",
    type: "refresh_publish_policy",
    status: "ack_accepted",
    issued_at: "2026-05-14T08:10:00Z",
    result_at: null,
  },
];

const releases = [
  {
    id: "rel_014",
    channel: "stable",
    version: "0.1.4",
    rollout_id: "rollout_456",
    status: "promoted",
    published_via: "GitHub / HA app repository",
  },
  {
    id: "rel_015",
    channel: "candidate",
    version: "0.1.5",
    rollout_id: "rollout_500",
    status: "published_not_promoted",
    published_via: "GitHub / HA app repository",
  },
];

const auditEvents = [
  "Provisioning session created for Smith Residence",
  "Device dev_123 claim finalized",
  "Backup trigger issued for dev_123",
  "Update rollout intent changed for rollout_456",
  "Credential rotation completed for dev_123",
];

const activityEvents = [
  {
    action: "Backup completed",
    subject: "Smith Residence",
    category: "Backup",
    time: "Today, 5:00 AM",
    group: "Today",
    detail: "Nightly backup finished and the latest archive is available.",
  },
  {
    action: "Reported disconnected",
    subject: "Lee Residence",
    category: "Alert",
    time: "2 hours ago",
    group: "Today",
    detail: "HomeSignal has not heard from this Home Assistant instance recently.",
  },
  {
    action: "Backup failed",
    subject: "Lee Residence",
    category: "Backup",
    time: "Today, 5:02 AM",
    group: "Today",
    detail: "The scheduled backup did not complete; last success was May 10.",
  },
  {
    action: "Claim finalized",
    subject: "Smith Residence",
    category: "Enrollment",
    time: "Yesterday",
    group: "Yesterday",
    detail: "Pairing completed and the HomeSignal app began reporting.",
  },
  {
    action: "Stable app release changed",
    subject: "0.1.4",
    category: "Update",
    time: "Yesterday",
    group: "Yesterday",
    detail: "The stable HomeSignal app version was advanced for managed sites.",
  },
  {
    action: "Credential rotation completed",
    subject: "Smith Residence",
    category: "Device",
    time: "May 12",
    group: "Earlier",
    detail: "The device certificate overlap window closed successfully.",
  },
  {
    action: "Pairing code created",
    subject: "Lee Residence",
    category: "Enrollment",
    time: "May 12",
    group: "Earlier",
    detail: "A new site-bound pairing code was created in the portal.",
  },
  {
    action: "Alert recipient updated",
    subject: "Northstar Smart Homes",
    category: "Alert",
    time: "May 11",
    group: "Earlier",
    detail: "Email alert delivery settings were changed for the account.",
  },
];

const currentAlerts = [
  {
    condition: "Backup failed",
    site: "Lee Residence",
    detail: "Last attempt today; last success 4 days ago",
    status: "backed",
    actionLabel: "View backups",
    page: "Backups",
    siteId: "site_124",
    deviceId: "dev_124",
  },
  {
    condition: "Disconnected",
    site: "Lee Residence",
    detail: "Last seen 2 hours ago",
    status: "backed",
    actionLabel: "View device",
    page: "Device Detail",
    siteId: "site_124",
    deviceId: "dev_124",
  },
];

const haAppState = {
  local_state: "UNCLAIMED",
  claim_invite_code: "4f8b0e7a-0f7d-45f8-8b8b-1e25f4d68a10",
  claim_invite_expires_at: "Valid for 72 hours from creation",
  claim_verification_expires_at: "15 minutes after verification",
  device_id: "dev_123",
  thing_name: "dev_123",
  config_path: "/config/device.json",
  cert_path: "/config/iot/device.pem",
  private_key_path: "/config/iot/private.key",
  cloud_connection: "cloud visible, not paired",
  last_policy_version: "ppv_123",
  ha_app_version: "0.1.3",
  desired_ha_app_version: "0.1.4",
  latest_ha_app_version: "0.1.4",
  update_available_for: "2 days",
  stale_update_threshold_days: 5,
  last_error_excerpt: "No recent errors.",
  site: "Smith Residence",
  organization: "Northstar Smart Homes",
  claim_creator_name: "Maya Patel",
  claim_creator_email: "maya.patel@northstar.example",
  customer_name: "Alex Smith",
  customer_email: "alex@example.com",
  service_address: "12 Oak Street, Columbus, OH",
  last_heartbeat: "Not paired yet",
  last_command_received: "None",
  last_command_completed: "None",
};

const haAppLogStorageSummary = {
  capture_level: "info",
  budget: "32 MiB",
  used: "7.8 MiB",
  retained_from: "2026-05-14T10:42:12Z",
  retained_to: "2026-05-14T12:00:03Z",
  dropped_count: 0,
  suppressed_count: 12,
  active_debug_session: "None",
  last_cloud_request: "None",
};

const haAppLogLevels = ["debug", "info", "warning", "error"];

const haAppLogLevelRank = {
  debug: 0,
  info: 1,
  warning: 2,
  error: 3,
};

const mockHaAppLogEntries = [
  {
    id: "log_001",
    ts: "2026-05-14T11:58:42Z",
    level: "info",
    component: "startup",
    reason: "startup_complete",
    message: "HomeSignal Manager runtime started; local state and options loaded.",
  },
  {
    id: "log_002",
    ts: "2026-05-14T11:58:44Z",
    level: "info",
    component: "enrollment",
    reason: "credential_loaded",
    message: "Loaded durable device credential from /config/device.json.",
  },
  {
    id: "log_003",
    ts: "2026-05-14T11:58:46Z",
    level: "debug",
    component: "config",
    reason: "runtime_options_loaded",
    message: "Loaded app runtime options: capture_level=info, publish_channel=stable.",
  },
  {
    id: "log_004",
    ts: "2026-05-14T11:58:59Z",
    level: "info",
    component: "permissions",
    reason: "policy_loaded",
    message: "Local management policy revision 1 loaded with full permissions.",
  },
  {
    id: "log_005",
    ts: "2026-05-14T11:59:00Z",
    level: "info",
    component: "cloud_connection",
    reason: "connected",
    message: "Connected to HomeSignal cloud broker.",
  },
  {
    id: "log_006",
    ts: "2026-05-14T11:59:01Z",
    level: "info",
    component: "telemetry",
    reason: "health_snapshot_sent",
    message: "Published HomeSignal Manager health snapshot schema_version=1 message_id=01J00000000000000000000000.",
  },
  {
    id: "log_007",
    ts: "2026-05-14T11:59:05Z",
    level: "warning",
    component: "logging",
    reason: "over_budget",
    message: "Suppressed 12 debug entries in the current local logging window.",
  },
  {
    id: "log_008",
    ts: "2026-05-14T11:59:19Z",
    level: "info",
    component: "commands",
    reason: "poll_complete",
    message: "No pending remote management commands.",
  },
  {
    id: "log_009",
    ts: "2026-05-14T11:59:30Z",
    level: "info",
    component: "local_probe",
    reason: "readiness_probe_completed",
    message: "App local readiness probe completed successfully.",
  },
  {
    id: "log_010",
    ts: "2026-05-14T11:59:31Z",
    level: "warning",
    component: "commands",
    reason: "command_rejected",
    message: "Rejected command cmd_01J00000000000000000000001 because local permission diagnostics_error_log_bundle is disabled.",
  },
  {
    id: "log_011",
    ts: "2026-05-14T11:59:42Z",
    level: "debug",
    component: "logging",
    reason: "segment_rollover_check",
    message: "Diagnostic log segment budget check complete: used=7.8MiB budget=32MiB.",
  },
  {
    id: "log_012",
    ts: "2026-05-14T12:00:03Z",
    level: "info",
    component: "updates",
    reason: "update_posture_checked",
    message: "HomeSignal Manager update posture checked: installed=0.1.3 latest=0.1.4.",
  },
];

const haAppScreens = {
  status: { label: "Status" },
  pairing: { label: "Pairing" },
  permissions: { label: "Permissions" },
  advanced: { label: "Advanced" },
};

const haAppStatusStates = {
  onboarding: { label: "Fresh install" },
  healthy: { label: "Healthy" },
  disconnected: { label: "Disconnected" },
  outdated: { label: "Update out of date" },
};

const haAppStatusHighlights = [
  ["HomeSignal cloud", "Visible", "Ready", "Cloud can be reached"],
  ["Pairing", "Not paired", "Needs attention", "No site association yet"],
  ["Home Assistant Core", "Connected", "Ready", "Core API responds"],
  ["Supervisor API", "Connected", "Ready", "Supervisor API responds"],
];

const haAppStatusSignals = [
  ["Overall health", "Ready to pair", "Ready", "Derived from local checks"],
  ["Attention reasons", "Not paired", "Needs attention", "Pairing required before cloud reporting"],
  ["HomeSignal app", "Running", "Ready", "Local app process"],
  ["HomeSignal app version", haAppState.ha_app_version, "Needs attention", `Latest ${haAppState.latest_ha_app_version}`],
  ["Home Assistant Core", "Connected", "Ready", "Core API"],
  ["Home Assistant version", "2026.5.1", "Ready", "Reported locally"],
  ["Supervisor API", "Connected", "Ready", "Supervisor API"],
  ["Supervisor version", "2026.05.0", "Ready", "Reported locally"],
  ["Storage", "Healthy", "Ready", "Local app storage"],
  ["Backups", "Not reporting yet", "Unavailable", "Available after pairing"],
  ["Updates", "New version available", "Needs attention", "Advisory after 48 hours"],
];

const haAppHealthSnapshot = [
  ["HomeSignal agent", "OK", "0.1.3 · uptime 24h", "ready"],
  ["Cloud access", "Ready", "HomeSignal portal reachable", "ready"],
  ["Account status", "Not paired yet", "First-time setup has not been completed", "neutral"],
  ["HomeSignal App Update", "Needs attention", "0.1.3 installed · 0.1.4 available", "warning"],
  ["Home Assistant Core", "Connected", "2026.5.1 · last checked 11:59", "ready"],
  ["Home Assistant Supervisor", "Connected", "2026.05.0 · last checked 11:59", "ready"],
  ["Storage", "OK", "61.4% used · 12 GB free", "ready"],
];

const haAppHealthySnapshot = [
  ["HomeSignal agent", "OK", "0.1.4 · uptime 24h", "ready"],
  ["Cloud paths", "OK", "HTTPS reachable · IoT connected", "ready"],
  ["Telemetry", "Reported", "Last reported May 14, 2026, 11:59 AM", "ready"],
  ["Account status", "Linked", "Northstar Smart Homes · Smith Residence", "ready"],
  ["HomeSignal App Update", "Current", "0.1.4 installed · stable channel", "ready"],
  ["Home Assistant Core", "Connected", "2026.5.1 · last checked 11:59", "ready"],
  ["Home Assistant Supervisor", "Connected", "2026.05.0 · last checked 11:59", "ready"],
  ["Backup", "OK", "Last success 03:00 · none running", "ready"],
  ["Storage", "OK", "61.4% used · 12 GB free", "ready"],
  ["Managed apps", "OK", "Alarm Bridge current · 42 events/hour", "ready"],
  ["Runtime logs", "OK", "No recent warnings", "ready"],
];

const haAppDisconnectedSnapshot = [
  ["HomeSignal agent", "OK", "0.1.4 · uptime 24h", "ready"],
  ["Cloud paths", "Disconnected", "Last successful connection May 14, 2026, 11:12 AM", "warning"],
  ["Telemetry", "Not reporting", "Last reported May 14, 2026, 11:12 AM", "warning"],
  ["Account status", "Linked", "Northstar Smart Homes · Smith Residence", "ready"],
  ["HomeSignal App Update", "Current", "0.1.4 installed · stable channel", "ready"],
  ["Home Assistant Core", "Connected", "2026.5.1 · last checked 11:59", "ready"],
  ["Home Assistant Supervisor", "Connected", "2026.05.0 · last checked 11:59", "ready"],
  ["Backup", "OK", "Last success 03:00 · none running", "ready"],
  ["Storage", "OK", "61.4% used · 12 GB free", "ready"],
  ["Managed apps", "OK", "Alarm Bridge current · 42 events/hour", "ready"],
  ["Runtime logs", "Review", "Cloud reconnect backoff active", "warning"],
];

const haAppOutdatedSnapshot = [
  ["HomeSignal agent", "OK", "0.1.3 · uptime 24h", "ready"],
  ["Cloud paths", "OK", "HTTPS reachable · IoT connected", "ready"],
  ["Telemetry", "Reported", "Last reported May 14, 2026, 11:59 AM", "ready"],
  ["Account status", "Linked", "Northstar Smart Homes · Smith Residence", "ready"],
  ["HomeSignal App Update", "Action required", "0.1.3 installed · 0.1.4 available for 6 days", "warning"],
  ["Home Assistant Core", "Connected", "2026.5.1 · last checked 11:59", "ready"],
  ["Home Assistant Supervisor", "Connected", "2026.05.0 · last checked 11:59", "ready"],
  ["Backup", "OK", "Last success 03:00 · none running", "ready"],
  ["Storage", "OK", "61.4% used · 12 GB free", "ready"],
  ["Managed apps", "OK", "Alarm Bridge current · 42 events/hour", "ready"],
  ["Runtime logs", "OK", "No recent warnings", "ready"],
];

const haAppUpdatePosture = [
  ["Installed version", haAppState.ha_app_version, "Needs attention", "Running app version"],
  ["Latest available version", haAppState.latest_ha_app_version, "Ready", `Available for ${haAppState.update_available_for}`],
  ["Update status", "New version available", "Needs attention", "Grace period has passed"],
  ["Auto-update setting", "Check in Home Assistant", "Needs attention", "HomeSignal cannot read this directly"],
];

const fullPermissionGroupOrder = ["Local status", "Apps", "Approved maintenance", "Reports"];

const haAppControlPolicy = [
  {
    key: "ha_status_read",
    label: "Read Home Assistant status",
    fullPermissionGroup: "Local status",
    fullPermissionLabel: "View Home Assistant health",
    action: "read Home Assistant status",
    description: "Read Core and Supervisor reachability and version status.",
    enabled: true,
    boundary: "Default managed install",
    audit: "normal",
    actor: "HomeSignal cloud and authorized site users",
    why: "Used to show whether the local Home Assistant environment is reachable and healthy before running any management workflow.",
  },
  {
    key: "ha_backup_status_read",
    label: "Read backup status",
    fullPermissionGroup: "Local status",
    fullPermissionLabel: "View backup status",
    action: "read backup status",
    description: "Read bounded Home Assistant backup summary and status.",
    enabled: true,
    boundary: "Default managed install",
    audit: "normal",
    actor: "HomeSignal cloud and authorized site users",
    why: "Used to surface whether the local Home Assistant installation appears protected by recent backups.",
  },
  {
    key: "ha_backup_trigger",
    label: "Trigger approved backups",
    fullPermissionGroup: "Approved maintenance",
    fullPermissionLabel: "Run approved backup actions",
    action: "trigger approved backup",
    description: "Allow approved cloud backup trigger commands.",
    enabled: true,
    boundary: "Default managed install",
    audit: "sensitive",
    actor: "Authorized site admins",
    why: "Used to start a bounded backup workflow without allowing arbitrary Home Assistant service calls.",
  },
  {
    key: "ha_storage_status_read",
    label: "Read storage status",
    fullPermissionGroup: "Local status",
    fullPermissionLabel: "View storage status",
    action: "read storage status",
    description: "Read bounded local storage status.",
    enabled: true,
    boundary: "Default managed install",
    audit: "normal",
    actor: "HomeSignal cloud and authorized site users",
    why: "Used to warn before the Home Assistant host runs low on storage.",
  },
  {
    key: "ha_app_inventory_read",
    label: "View installed apps",
    fullPermissionGroup: "Apps",
    fullPermissionLabel: "View installed apps",
    action: "view installed apps",
    description: "Read installed app names, versions, update status, and health.",
    enabled: true,
    boundary: "Default managed install",
    audit: "normal",
    actor: "HomeSignal cloud and authorized site users",
    why: "Used to show update posture and managed app health without changing the Home Assistant installation.",
  },
  {
    key: "homesignal_app_update_status_read",
    label: "Read HomeSignal app update status",
    fullPermissionGroup: "Apps",
    fullPermissionLabel: "View HomeSignal update status",
    action: "read HomeSignal app update status",
    description: "Read the installed, latest, desired, and update-readiness state for this app.",
    enabled: true,
    boundary: "Default managed install",
    audit: "normal",
    actor: "HomeSignal cloud and authorized site users",
    why: "Used to explain whether the HomeSignal app is current or needs local update attention.",
  },
  {
    key: "homesignal_app_update_intent",
    label: "Apply HomeSignal app update intent",
    fullPermissionGroup: "Apps",
    fullPermissionLabel: "Manage approved HomeSignal app updates",
    action: "apply HomeSignal app update intent",
    description: "Allow HomeSignal to request the desired HomeSignal app version or channel.",
    enabled: true,
    boundary: "Default managed install",
    audit: "normal",
    actor: "HomeSignal cloud and authorized site admins",
    why: "Used to keep the HomeSignal app aligned with the version policy chosen for the site.",
  },
  {
    key: "diagnostics_basic",
    label: "Run bounded diagnostics",
    fullPermissionGroup: "Approved maintenance",
    fullPermissionLabel: "Run bounded diagnostics",
    action: "run bounded diagnostics",
    description: "Collect bounded app, connectivity, and update-readiness diagnostics.",
    enabled: true,
    boundary: "Default managed install",
    audit: "sensitive",
    actor: "HomeSignal support or authorized site admins",
    why: "Used when an installer or support user needs enough local context to troubleshoot a failed command or unhealthy app.",
  },
  {
    key: "diagnostics_error_log_bundle",
    label: "Share redacted error-log bundles",
    action: "share redacted error-log bundle",
    description: "Allow approved brokered upload of redacted error-log bundles.",
    enabled: false,
    boundary: "Local opt-in",
    audit: "sensitive",
    actor: "Authorized site admins after local opt-in",
    why: "Logs can contain more sensitive operational detail, so this starts disabled and can only be enabled from the local app.",
  },
  {
    key: "runtime_log_summary",
    label: "Send runtime warning summaries",
    fullPermissionGroup: "Reports",
    fullPermissionLabel: "Send runtime warning summaries",
    action: "send runtime warning summaries",
    description: "Send bounded collapsed runtime warning summaries.",
    enabled: true,
    boundary: "Default managed install",
    audit: "normal",
    actor: "HomeSignal app runtime",
    why: "Used to show recurring local runtime issues without uploading raw logs.",
  },
];

const unsupportedHaAppControls = [
  ["Install Home Assistant Apps", "Future command contract required"],
  ["Rollback Home Assistant Apps", "Future command contract required"],
  ["Update Home Assistant Core", "Future command contract required"],
  ["Broad Home Assistant diagnostics", "Not executable in v0"],
  ["Raw log export", "Not allowed"],
  ["Arbitrary Home Assistant service calls", "Never allowed"],
];

const initialHaAppPermissionPolicy = {
  accessMode: "full",
  granularControls: Object.fromEntries(haAppControlPolicy.map((control) => [control.key, control.enabled])),
};

const haAppFullPermissionControls = haAppControlPolicy.filter(
  (control) => control.enabled && control.boundary === "Default managed install" && control.fullPermissionGroup
);

const fullControlPermissionGroups = fullPermissionGroupOrder
  .map((group) => [
    group,
    haAppFullPermissionControls
      .filter((control) => control.fullPermissionGroup === group)
      .map((control) => control.fullPermissionLabel || control.label),
  ])
  .filter(([, permissions]) => permissions.length > 0);

const fullControlPermissionLabels = haAppFullPermissionControls.map((control) => control.fullPermissionLabel || control.label);

const haAppAuditEvents = [
  ["Update posture checked", "Supervisor API returned auto-update off", "2 min ago"],
  ["Control policy reviewed", "Diagnostics enabled; log export disabled", "2 min ago"],
  ["Pairing code created", "Pairing code expires in 71 hours", "3 min ago"],
  ["Supervisor capability check completed", "Required local APIs available", "3 min ago"],
];

function HaAppPairingBridgePage() {
  const [, setStoredContext] = useState(() => readAutoPairingContext());
  const [storedHomeAssistantUrl, setStoredHomeAssistantUrl] = useState(() => readStoredHomeAssistantUrl());
  const [draftHomeAssistantUrl, setDraftHomeAssistantUrl] = useState(() => readStoredHomeAssistantUrl());
  const [isEditingHomeAssistantUrl, setIsEditingHomeAssistantUrl] = useState(false);
  const [allowHomeAssistantUrlParam, setAllowHomeAssistantUrlParam] = useState(true);
  const [initialLookupDone, setInitialLookupDone] = useState(false);
  const didApplyHomeAssistantUrlParamRef = useRef(false);
  const params = typeof window === "undefined" ? new URLSearchParams() : new URLSearchParams(window.location.search);
  const pairingId = params.get("pairing_id") || "";
  const isBridgeOnly = params.get("bridge") === "1";
  const mockHomeAssistantUrlParam = params.get("mock_ha_url") || "";
  const homeAssistantUrlParam = params.get("ha_url") || "";
  const preferredHomeAssistantUrlParam = (mockHomeAssistantUrlParam || homeAssistantUrlParam).replace(/\/$/, "");
  const visibleHomeAssistantUrl = (allowHomeAssistantUrlParam && preferredHomeAssistantUrlParam) || storedHomeAssistantUrl;
  const realHomeAssistantUrl = visibleHomeAssistantUrl || defaultHomeAssistantUrl;
  const realInstalledAppUrl = `${realHomeAssistantUrl.replace(/\/$/, "")}${installedHaAppPath}`;
  const realHaAppBridgeUrl = `${realHomeAssistantUrl.replace(/\/$/, "")}${haAppBridgePath}`;
  const mockHaAppBridgeUrl = mockHomeAssistantUrlParam ? `${mockHomeAssistantUrlParam.replace(/\/$/, "")}${mockHaAppBridgePath}` : "";
  const haAppBridgeUrl = params.get("bridge_url") || params.get("mock_ha_app_bridge_url") || mockHaAppBridgeUrl || realHaAppBridgeUrl;
  const installHaAppUrl = haAppRepositoryDeepLink;
  const installedHaAppUrl = realInstalledAppUrl;
  const hasHomeAssistantUrl = Boolean(visibleHomeAssistantUrl);

  useEffect(() => {
    if (!pairingId) return;
    setStoredContext(writeAutoPairingContext(pairingId));
  }, [pairingId]);

  useEffect(() => {
    if (!allowHomeAssistantUrlParam || !preferredHomeAssistantUrlParam || didApplyHomeAssistantUrlParamRef.current) return;
    didApplyHomeAssistantUrlParamRef.current = true;
    writeStoredHomeAssistantUrl(preferredHomeAssistantUrlParam);
    setStoredHomeAssistantUrl(preferredHomeAssistantUrlParam);
    setDraftHomeAssistantUrl(preferredHomeAssistantUrlParam);
  }, [allowHomeAssistantUrlParam, preferredHomeAssistantUrlParam]);

  useEffect(() => {
    if (hasHomeAssistantUrl) {
      setInitialLookupDone(true);
      return undefined;
    }

    setInitialLookupDone(false);
    const timeout = window.setTimeout(() => setInitialLookupDone(true), haAppBridgeProbeTimeoutMs);
    return () => window.clearTimeout(timeout);
  }, [hasHomeAssistantUrl]);

  useEffect(() => {
    const onMessage = (event) => {
      if (!event.source || typeof event.data !== "object" || event.data === null) return;

      const { type, request_id: requestId } = event.data;
      if (type === "homesignal.auto_pairing.get") {
        const value = readAutoPairingContext();
        event.source.postMessage(
          value
            ? { type: "homesignal.auto_pairing.value", request_id: requestId, ok: true, value }
            : { type: "homesignal.auto_pairing.value", request_id: requestId, ok: false, reason: "missing_or_expired" },
          event.origin
        );
      }

      if (type === "homesignal.auto_pairing.remove") {
        removeAutoPairingContext();
        setStoredContext(null);
        event.source.postMessage(
          { type: "homesignal.auto_pairing.removed", request_id: requestId, ok: true },
          event.origin
        );
      }
    };

    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  const saveHomeAssistantUrl = () => {
    const nextUrl = draftHomeAssistantUrl.trim().replace(/\/$/, "");
    writeStoredHomeAssistantUrl(nextUrl);
    setAllowHomeAssistantUrlParam(false);
    setStoredHomeAssistantUrl(nextUrl);
    setDraftHomeAssistantUrl(nextUrl);
    setIsEditingHomeAssistantUrl(false);
  };

  const openDraftHomeAssistantUrl = () => {
    const nextUrl = draftHomeAssistantUrl.trim().replace(/\/$/, "");
    if (!nextUrl) return;

    writeStoredHomeAssistantUrl(nextUrl);
    setAllowHomeAssistantUrlParam(false);
    setStoredHomeAssistantUrl(nextUrl);
    setDraftHomeAssistantUrl(nextUrl);

    window.location.href = `${nextUrl}${installedHaAppPath}`;
  };

  const showNoSavedHomeAssistantUrlMock = () => {
    didApplyHomeAssistantUrlParamRef.current = true;
    setAllowHomeAssistantUrlParam(false);
    writeStoredHomeAssistantUrl("");
    setStoredHomeAssistantUrl("");
    setDraftHomeAssistantUrl("");
    setIsEditingHomeAssistantUrl(false);
  };

  const showSavedHomeAssistantUrlMock = () => {
    setAllowHomeAssistantUrlParam(true);
    const nextUrl = preferredHomeAssistantUrlParam || defaultHomeAssistantUrl;
    writeStoredHomeAssistantUrl(nextUrl);
    setStoredHomeAssistantUrl(nextUrl);
    setDraftHomeAssistantUrl(nextUrl);
    setIsEditingHomeAssistantUrl(false);
  };

  if (isBridgeOnly) {
    return <div aria-hidden="true" className="sr-only">HomeSignal auto-pairing bridge</div>;
  }

  return (
    <main className="min-h-screen bg-[#f5f5f5] px-4 py-10 font-['Roboto',Arial,sans-serif] text-[#212121]">
      <div className="mx-auto max-w-3xl">
        {isLocalMockHost() && (
          <HaAppPairingMockControls
            activeView={hasHomeAssistantUrl ? "saved" : "empty"}
            onShowNoSavedAddress={showNoSavedHomeAssistantUrlMock}
            onShowSavedAddress={showSavedHomeAssistantUrlMock}
          />
        )}

        {hasHomeAssistantUrl ? (
          <HaAppPairingKnownHomeAssistant
            homeAssistantUrl={visibleHomeAssistantUrl}
            draftHomeAssistantUrl={draftHomeAssistantUrl}
            isEditing={isEditingHomeAssistantUrl}
            installedHaAppUrl={installedHaAppUrl}
            installHaAppUrl={installHaAppUrl}
            haAppBridgeUrl={haAppBridgeUrl}
            pairingId={pairingId}
            onDraftChange={setDraftHomeAssistantUrl}
            onEdit={() => setIsEditingHomeAssistantUrl(true)}
            onCancelEdit={() => {
              setDraftHomeAssistantUrl(storedHomeAssistantUrl);
              setIsEditingHomeAssistantUrl(false);
            }}
            onSave={saveHomeAssistantUrl}
          />
        ) : !initialLookupDone ? (
          <HaAppPairingDetector
            bridgeStatus={{ state: "checking", label: "Checking HomeSignal app" }}
            homeAssistantUrl="No Home Assistant address saved"
            installedHaAppUrl=""
            pairingId={pairingId}
          />
        ) : (
          <HaAppPairingUnknownHomeAssistant
            draftHomeAssistantUrl={draftHomeAssistantUrl}
            installHaAppUrl={installHaAppUrl}
            installedHaAppUrl={installedHaAppUrl}
            onDraftChange={setDraftHomeAssistantUrl}
            onOpenDraft={openDraftHomeAssistantUrl}
          />
        )}
      </div>
    </main>
  );
}

function HaAppPairingMockControls({ activeView, onShowNoSavedAddress, onShowSavedAddress }) {
  return (
    <section className="mb-7 rounded-lg border border-dashed border-[#bdbdbd] bg-white px-4 py-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-xs font-medium uppercase tracking-wide text-[#616161]">Mock view controls</div>
          <div className="mt-1 text-sm text-[#616161]">Toggle the browser Home Assistant URL state for testing.</div>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={onShowNoSavedAddress}
            aria-pressed={activeView === "empty"}
            className={`rounded-full px-4 py-2 text-sm font-medium ${
              activeView === "empty"
                ? "bg-[#039dcc] text-white"
                : "border border-[#d6d6d6] bg-white text-[#039dcc] hover:bg-[#e1f5fe]"
            }`}
          >
            No saved address
          </button>
          <button
            type="button"
            onClick={onShowSavedAddress}
            aria-pressed={activeView === "saved"}
            className={`rounded-full px-4 py-2 text-sm font-medium ${
              activeView === "saved"
                ? "bg-[#039dcc] text-white"
                : "border border-[#d6d6d6] bg-white text-[#039dcc] hover:bg-[#e1f5fe]"
            }`}
          >
            Saved address
          </button>
        </div>
      </div>
    </section>
  );
}

function HaAppPairingKnownHomeAssistant({
  homeAssistantUrl,
  draftHomeAssistantUrl,
  isEditing,
  installedHaAppUrl,
  installHaAppUrl,
  haAppBridgeUrl,
  pairingId,
  onDraftChange,
  onEdit,
  onCancelEdit,
  onSave,
}) {
  const [bridgeStatus, setBridgeStatus] = useState({ state: "checking", label: "Checking HomeSignal app" });
  const bridgeIframeRef = useRef(null);
  const bridgeRequestIdRef = useRef(null);
  const autoContinueTimerRef = useRef(null);
  const haAppsDashboardUrl = `${homeAssistantUrl.replace(/\/$/, "")}/hassio/dashboard`;

  useEffect(() => {
    if (isEditing) return undefined;

    let cancelled = false;
    const requestId = `ha_app_bridge_${Date.now()}_${Math.random().toString(36).slice(2)}`;
    bridgeRequestIdRef.current = requestId;

    let timeout = null;
    let pingTimers = [];

    const sendPing = () => {
      const targetWindow = bridgeIframeRef.current?.contentWindow;
      if (!targetWindow || cancelled) return;

      targetWindow.postMessage(
        {
          type: "homesignal.ha_app_bridge.ping",
          request_id: requestId,
          expected_origin: window.location.origin,
        },
        "*"
      );
    };

    const onMessage = (event) => {
      if (cancelled || event.source !== bridgeIframeRef.current?.contentWindow) return;
      if (typeof event.data !== "object" || event.data === null) return;
      if (event.data.type !== "homesignal.ha_app_bridge.pong") return;
      if (event.data.request_id !== requestId) return;

      if (timeout) window.clearTimeout(timeout);
      pingTimers.forEach((timer) => window.clearTimeout(timer));

      if (event.data.ok) {
        setBridgeStatus({
          state: "found",
          label: "HomeSignal app found",
          detail: event.data.app?.version ? `App version ${event.data.app.version}` : "",
        });
      } else {
        setBridgeStatus({ state: "not_found", label: "Could not find HomeSignal at this address" });
      }
    };

    window.addEventListener("message", onMessage);
    setBridgeStatus({ state: "checking", label: "Checking HomeSignal app" });
    pingTimers = [100, 450, 1000].map((delay) => window.setTimeout(sendPing, delay));
    timeout = window.setTimeout(() => {
      if (!cancelled) {
        setBridgeStatus({ state: "not_found", label: "Could not find HomeSignal at this address" });
      }
    }, haAppBridgeProbeTimeoutMs);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
      pingTimers.forEach((timer) => window.clearTimeout(timer));
      window.removeEventListener("message", onMessage);
    };
  }, [haAppBridgeUrl, isEditing]);

  useEffect(() => {
    if (!pairingId || bridgeStatus.state !== "found" || isEditing) return undefined;

    autoContinueTimerRef.current = window.setTimeout(() => {
      window.location.href = installedHaAppUrl;
    }, haAppBridgeAutoContinueDelayMs);

    return () => window.clearTimeout(autoContinueTimerRef.current);
  }, [bridgeStatus.state, installedHaAppUrl, isEditing, pairingId]);

  const showDetector = !isEditing && bridgeStatus.state !== "not_found";

  return (
    <>
      {!isEditing && (
        <iframe
          ref={bridgeIframeRef}
          src={haAppBridgeUrl}
          title="HomeSignal app bridge probe"
          className="hidden"
          onLoad={() => {
            const targetWindow = bridgeIframeRef.current?.contentWindow;
            if (!targetWindow) return;
            targetWindow.postMessage(
              {
                type: "homesignal.ha_app_bridge.ping",
                request_id: bridgeRequestIdRef.current,
                expected_origin: window.location.origin,
              },
              "*"
            );
          }}
        />
      )}

      {showDetector ? (
        <HaAppPairingDetector
          bridgeStatus={bridgeStatus}
          installedHaAppUrl={installedHaAppUrl}
          homeAssistantUrl={homeAssistantUrl}
          pairingId={pairingId}
        />
      ) : (
        <HaAppPairingFallbackOptions
          title="We could not find HomeSignal in Home Assistant"
          description="The app may not be installed yet, Home Assistant may be stopped, or the address may need to be changed."
          homeAssistantUrl={homeAssistantUrl}
          draftHomeAssistantUrl={draftHomeAssistantUrl}
          installedHaAppUrl={installedHaAppUrl}
          haAppsDashboardUrl={haAppsDashboardUrl}
          installHaAppUrl={installHaAppUrl}
          isEditing={isEditing}
          onDraftChange={onDraftChange}
          onEdit={onEdit}
          onCancelEdit={onCancelEdit}
          onSave={onSave}
        />
      )}
    </>
  );
}

function HaAppPairingUnknownHomeAssistant({
  draftHomeAssistantUrl,
  installHaAppUrl,
  onDraftChange,
  onOpenDraft,
}) {
  const manualHomeAssistantUrl = draftHomeAssistantUrl.trim().replace(/\/$/, "");
  const manualInstalledAppUrl = manualHomeAssistantUrl ? `${manualHomeAssistantUrl}${installedHaAppPath}` : "";
  const manualAppsDashboardUrl = manualHomeAssistantUrl ? `${manualHomeAssistantUrl}/hassio/dashboard` : "";

  return (
    <HaAppPairingFallbackOptions
      title="Pair Home Assistant with HomeSignal"
      description="Install the HomeSignal app to complete pairing."
      homeAssistantUrl={manualHomeAssistantUrl}
      draftHomeAssistantUrl={draftHomeAssistantUrl}
      installedHaAppUrl={manualInstalledAppUrl}
      haAppsDashboardUrl={manualAppsDashboardUrl}
      installHaAppUrl={installHaAppUrl}
      isEditing
      onDraftChange={onDraftChange}
      onSave={onOpenDraft}
      saveLabel="Open HomeSignal app"
    />
  );
}

function HaAppPairingDetector({ bridgeStatus, homeAssistantUrl, pairingId }) {
  const isFound = bridgeStatus.state === "found";

  return (
    <section className="flex min-h-[68vh] items-center justify-center text-center">
      <div className="max-w-xl">
        <div className="mx-auto flex h-14 w-14 items-center justify-center">
          {isFound ? <HaAppPairingCheckIcon /> : <HaAppPairingSpinner />}
        </div>
        <h1 className="mt-6 text-4xl font-normal tracking-normal">
          {isFound ? "HomeSignal found" : "Looking for HomeSignal"}
        </h1>
        <p className="mx-auto mt-3 max-w-md text-base leading-6 text-[#616161]">
          {isFound
            ? "We found the HomeSignal app in Home Assistant."
            : "Checking your local Home Assistant for the HomeSignal app."}
        </p>
        <p className="mt-5 break-all font-mono text-sm text-[#616161]">{homeAssistantUrl}</p>

        {bridgeStatus.detail && <p className="mt-4 text-sm text-emerald-700">{bridgeStatus.detail}</p>}

        {isFound && pairingId ? (
          <p className="mt-4 text-sm text-emerald-700">Continuing in Home Assistant...</p>
        ) : null}
      </div>
    </section>
  );
}

function HaAppPairingFallbackOptions({
  title,
  description,
  homeAssistantUrl,
  draftHomeAssistantUrl,
  installedHaAppUrl,
  haAppsDashboardUrl,
  installHaAppUrl,
  isEditing,
  onDraftChange,
  onEdit,
  onCancelEdit,
  onSave,
  saveLabel = "Save address",
}) {
  const [copiedField, setCopiedField] = useState("");

  const copyField = async (field, value) => {
    const didCopy = await copyTextToClipboard(value);
    if (!didCopy && !value) return;
    setCopiedField(field);
    window.setTimeout(() => setCopiedField(""), 1600);
  };

  return (
    <>
      <div className="text-center">
        <h1 className="text-4xl font-normal tracking-normal">{title}</h1>
        <p className="mx-auto mt-3 max-w-xl text-base leading-6 text-[#616161]">{description}</p>
      </div>

      <section className="mx-auto mt-8 max-w-2xl rounded-xl border border-[#e0e0e0] bg-white p-7 text-center">
        <h2 className="text-2xl font-normal text-[#212121]">Install HomeSignal app</h2>
        <p className="mx-auto mt-3 max-w-md text-base leading-6 text-[#616161]">
          Add HomeSignal to Home Assistant, then return here to continue pairing.
        </p>
        <a
          href={installHaAppUrl}
          className="mt-6 inline-flex rounded-full bg-[#039dcc] px-8 py-3 text-base font-medium text-white hover:bg-[#0288d1]"
        >
          Install in Home Assistant
        </a>
      </section>

      <section className="mx-auto mt-6 max-w-2xl rounded-xl border border-[#e0e0e0] bg-transparent p-5">
        <h2 className="text-lg font-medium text-[#212121]">Open manually</h2>
        <p className="mt-2 text-sm leading-6 text-[#616161]">
          Use this if the app is already installed or the Home Assistant address needs adjustment.
        </p>

        {isEditing ? (
          <div className="mt-4">
            <label className="text-sm font-medium text-[#616161]" htmlFor="app-pairing-ha-url">
              Home Assistant address
            </label>
            <div className="mt-2 grid gap-3 md:grid-cols-[1fr_auto]">
              <input
                id="app-pairing-ha-url"
                type="url"
                value={draftHomeAssistantUrl}
                onChange={(event) => onDraftChange(event.target.value)}
                placeholder="http://192.168.1.3"
                className="min-w-0 rounded-lg border border-[#d6d6d6] bg-white px-4 py-3 text-base text-[#212121] outline-none focus:border-[#039dcc]"
                aria-label="Home Assistant address"
              />
              <button
                type="button"
                onClick={onSave}
                disabled={!draftHomeAssistantUrl.trim()}
                className="rounded-full bg-[#039dcc] px-5 py-2.5 text-sm font-medium text-white hover:bg-[#0288d1] disabled:cursor-not-allowed disabled:bg-[#bdbdbd] disabled:hover:bg-[#bdbdbd]"
              >
                {saveLabel}
              </button>
            </div>
            {onCancelEdit && (
              <button
                type="button"
                onClick={onCancelEdit}
                className="mt-3 rounded-full px-4 py-2 text-sm font-medium text-[#616161] hover:bg-[#eeeeee]"
              >
                Cancel
              </button>
            )}
          </div>
        ) : (
          <div className="mt-4 flex items-start justify-between gap-4">
            <div>
              <h3 className="text-sm font-medium text-[#616161]">Home Assistant address</h3>
              <p className="mt-2 break-all text-base font-medium text-[#212121]">{homeAssistantUrl}</p>
            </div>
            {onEdit && (
              <button
                type="button"
                onClick={onEdit}
                className="rounded-full px-3 py-1.5 text-sm font-medium text-[#039dcc] hover:bg-[#e1f5fe]"
              >
                Edit
              </button>
            )}
          </div>
        )}

        {installedHaAppUrl && (
          <HaAppPairingUrlRow
            label="Full app URL"
            value={installedHaAppUrl}
            copied={copiedField === "app"}
            onCopy={() => copyField("app", installedHaAppUrl)}
          />
        )}
        {haAppsDashboardUrl && (
          <HaAppPairingUrlRow
            label="Home Assistant apps"
            value={haAppsDashboardUrl}
            copied={copiedField === "ha_apps"}
            onCopy={() => copyField("ha_apps", haAppsDashboardUrl)}
          />
        )}

        <div className="mt-5 flex flex-wrap gap-3">
          {installedHaAppUrl && (
            <a
              href={installedHaAppUrl}
              className="inline-flex rounded-full bg-[#039dcc] px-5 py-2.5 text-sm font-medium text-white hover:bg-[#0288d1]"
            >
              Open HomeSignal app
            </a>
          )}
          {haAppsDashboardUrl && (
            <a
              href={haAppsDashboardUrl}
              className="inline-flex rounded-full border border-[#d6d6d6] px-5 py-2.5 text-sm font-medium text-[#039dcc] hover:bg-[#e1f5fe]"
            >
              Open Home Assistant apps
            </a>
          )}
        </div>
      </section>
    </>
  );
}

function HaAppPairingUrlRow({ label, value, copied, onCopy }) {
  return (
    <div className="mt-5">
      <h3 className="text-sm font-medium text-[#616161]">{label}</h3>
      <div className="mt-2 flex items-start gap-3">
        <p className="min-w-0 flex-1 break-all font-mono text-sm text-[#424242]">{value}</p>
        <button
          type="button"
          onClick={onCopy}
          className="rounded-full px-3 py-1.5 text-sm font-medium text-[#039dcc] hover:bg-[#e1f5fe]"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
    </div>
  );
}

function HaAppPairingSpinner() {
  return (
    <div
      aria-hidden="true"
      className="h-12 w-12 animate-spin rounded-full border-4 border-[#b3e5fc] border-t-[#039dcc]"
    />
  );
}

function HaAppPairingCheckIcon() {
  return (
    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-emerald-600 text-white">
      <svg aria-hidden="true" viewBox="0 0 24 24" className="h-7 w-7" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
        <path d="M20 6 9 17l-5-5" />
      </svg>
    </div>
  );
}

function HaAppBridgeMockPage() {
  const params = typeof window === "undefined" ? new URLSearchParams() : new URLSearchParams(window.location.search);
  const mode = params.get("mode") || "found";
  const responseDelayMs = Number(params.get("delay") || 0);

  useEffect(() => {
    const responseTimers = new Set();

    const onMessage = (event) => {
      if (!event.source || typeof event.data !== "object" || event.data === null) return;
      if (event.data.type !== "homesignal.ha_app_bridge.ping") return;
      if (mode === "silent") return;

      const respond = () => {
        event.source.postMessage(
          {
            type: "homesignal.ha_app_bridge.pong",
            request_id: event.data.request_id,
            ok: mode !== "missing",
            app: {
              name: "HomeSignal",
              version: "0.1.3",
              bridge_version: 1,
            },
            home_assistant: {
              base_url: window.location.origin,
            },
          },
          event.origin || "*"
        );
      };

      if (responseDelayMs > 0) {
        const timer = window.setTimeout(() => {
          responseTimers.delete(timer);
          respond();
        }, responseDelayMs);
        responseTimers.add(timer);
        return;
      }

      respond();
    };

    window.addEventListener("message", onMessage);
    return () => {
      responseTimers.forEach((timer) => window.clearTimeout(timer));
      window.removeEventListener("message", onMessage);
    };
  }, [mode, responseDelayMs]);

  return (
    <main className="min-h-screen bg-white px-6 py-8 font-['Roboto',Arial,sans-serif] text-[#212121]">
      <h1 className="text-2xl font-normal">HomeSignal app bridge mock</h1>
      <p className="mt-3 text-sm text-[#616161]">
        This mock page answers the narrow HomeSignal app bridge postMessage ping.
      </p>
      <p className="mt-3 font-mono text-sm text-[#424242]">mode={mode}; delay={responseDelayMs}ms</p>
    </main>
  );
}

export default function HomeSignalProductSkeleton() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/ha_app_pairing" element={<HaAppPairingBridgePage />} />
        <Route path={mockHaAppBridgePath} element={<HaAppBridgeMockPage />} />
        <Route path="/*" element={<HomeSignalProductApp />} />
      </Routes>
    </BrowserRouter>
  );
}

function HomeSignalProductApp() {
  const initialRoute = readRouteHash();
  const [page, setPage] = useState(initialRoute.page);
  const [selectedSiteId, setSelectedSiteId] = useState(initialRoute.site);
  const [selectedDeviceId, setSelectedDeviceId] = useState(initialRoute.device);
  const [haAppScreen, setHaAppScreen] = useState(initialRoute.haApp);
  const [showWiring, setShowWiring] = useState(initialRoute.wiring === "on");
  const [activeWiring, setActiveWiring] = useState(null);
  const lastRouteKeyRef = useRef(routeToHash(initialRoute));

  useEffect(() => {
    const syncFromHash = () => {
      const route = readRouteHash();
      setPage(route.page);
      setSelectedSiteId(route.site);
      setSelectedDeviceId(route.device);
      setHaAppScreen(route.haApp);
      setShowWiring(route.wiring === "on");
    };

    window.addEventListener("hashchange", syncFromHash);
    return () => window.removeEventListener("hashchange", syncFromHash);
  }, []);

  useEffect(() => {
    const nextHash = routeToHash({
      page,
      site: selectedSiteId,
      device: selectedDeviceId,
      haApp: haAppScreen,
      wiring: showWiring ? "on" : "off",
    });

    if (lastRouteKeyRef.current !== nextHash) {
      window.scrollTo({ top: 0, left: 0, behavior: "auto" });
      lastRouteKeyRef.current = nextHash;
    }

    if (window.location.hash !== nextHash) {
      window.history.pushState(null, "", nextHash);
    }
  }, [page, selectedSiteId, selectedDeviceId, haAppScreen, showWiring]);

  useEffect(() => {
    if (!showWiring) {
      setActiveWiring(null);
      return undefined;
    }

    const readTarget = (target) => {
      const element = target?.closest?.("[data-wiring-id]");
      if (!element) return null;
      return {
        id: element.getAttribute("data-wiring-id"),
        status: element.getAttribute("data-wiring-status") || "missing",
        source: element.getAttribute("data-wiring-source") || "No source hint.",
      };
    };

    const onOver = (event) => {
      const next = readTarget(event.target);
      if (next) setActiveWiring(next);
    };

    const onOut = (event) => {
      if (!readTarget(event.relatedTarget)) {
        setActiveWiring(null);
      }
    };

    const onFocus = (event) => {
      const next = readTarget(event.target);
      if (next) setActiveWiring(next);
    };

    const onClick = (event) => {
      const next = readTarget(event.target);
      if (next) setActiveWiring(next);
    };

    const onBlur = () => setActiveWiring(null);

    document.addEventListener("mouseover", onOver);
    document.addEventListener("mouseout", onOut);
    document.addEventListener("click", onClick);
    document.addEventListener("focusin", onFocus);
    document.addEventListener("focusout", onBlur);

    return () => {
      document.removeEventListener("mouseover", onOver);
      document.removeEventListener("mouseout", onOut);
      document.removeEventListener("click", onClick);
      document.removeEventListener("focusin", onFocus);
      document.removeEventListener("focusout", onBlur);
    };
  }, [showWiring]);

  const selectedSite = sites.find((site) => site.id === selectedSiteId) || sites[0];
  const selectedDevice =
    devices.find((device) => device.id === selectedDeviceId) || devices[0];

  if (authPages.has(page)) {
    return <AuthExperience page={page} setPage={setPage} />;
  }

  if (page === "HA App") {
    return (
      <DataWiringContext.Provider value={showWiring}>
        <div className="min-h-screen bg-[#f5f5f5] px-3 py-4 pb-24 text-[#212121] sm:px-6 sm:py-6 sm:pb-6">
          <HaApp haAppScreen={haAppScreen} setHaAppScreen={setHaAppScreen} />
        </div>
      </DataWiringContext.Provider>
    );
  }

  return (
    <DataWiringContext.Provider value={showWiring}>
    <div className="min-h-screen bg-slate-100 text-slate-950">
      <div className="flex min-h-screen">
        <aside className="w-72 shrink-0 border-r border-slate-300 bg-white p-4">
          <div className="mb-5">
            <div className="text-lg font-semibold">HomeSignal</div>
            <div className="text-xs text-slate-500">Home Assistant management</div>
          </div>

          <button
            type="button"
            onClick={() => setPage("Enrollment")}
            className={`mb-5 w-full rounded-md px-3 py-2 text-left text-sm font-medium ${
              page === "Enrollment"
                ? "bg-slate-900 text-white"
                : "bg-[#03a9f4] text-white hover:bg-[#0288d1]"
            }`}
          >
            Pair Home Assistant
          </button>

          <button
            type="button"
            onClick={() => setShowWiring((value) => !value)}
            className={`mb-2 w-full rounded-md border px-3 py-2 text-left text-sm font-medium ${
              showWiring
                ? "border-rose-200 bg-rose-50 text-rose-900"
                : "border-slate-200 bg-white text-slate-600 hover:bg-slate-50"
            }`}
          >
            Data wiring {showWiring ? "on" : "off"}
          </button>
          {showWiring && (
            <div className="mb-5 rounded-md border border-rose-200 bg-rose-50/70 px-3 py-2 text-xs leading-5 text-rose-900">
              Dynamic values are highlighted. Hover to see schema fields.
              <div className="mt-2 flex flex-wrap gap-2 text-[11px]">
                <span className="rounded border border-emerald-300 bg-white px-1.5 py-0.5 text-emerald-800">green backed</span>
                <span className="rounded border border-amber-400 bg-white px-1.5 py-0.5 text-amber-900">yellow partial</span>
                <span className="rounded border border-rose-300 bg-white px-1.5 py-0.5 text-rose-800">red missing</span>
              </div>
            </div>
          )}

          <nav className="space-y-5">
            {navGroups.map((group) => (
              <div key={group.title}>
                <div className="mb-1 px-3 text-xs font-semibold uppercase tracking-normal text-slate-400">
                  {group.title}
                </div>
                <div className="space-y-1">
                  {group.items.map((item) => (
                    <button
                      key={item}
                      onClick={() => setPage(item)}
                      className={`w-full rounded-md px-3 py-2 text-left text-sm ${
                        page === item || (page === "Device Detail" && item === "Devices")
                          ? "bg-slate-900 text-white"
                          : "text-slate-700 hover:bg-slate-100"
                      }`}
                    >
                      {item}
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </nav>

        </aside>

        <main className="flex-1 p-6">
          {page === "Dashboard" && (
            <Dashboard
              setPage={setPage}
              setSelectedSiteId={setSelectedSiteId}
              setSelectedDeviceId={setSelectedDeviceId}
            />
          )}
          {page === "Account Settings" && <Accounts />}
          {page === "Users" && <Users />}
          {page === "Sites" && (
            <Sites
              selectedSiteId={selectedSiteId}
              setSelectedSiteId={setSelectedSiteId}
              setSelectedDeviceId={setSelectedDeviceId}
              setPage={setPage}
            />
          )}
          {page === "Enrollment" && <Enrollment selectedSite={selectedSite} />}
          {page === "Devices" && (
            <DeviceFleet
              setPage={setPage}
              setSelectedDeviceId={setSelectedDeviceId}
              setSelectedSiteId={setSelectedSiteId}
            />
          )}
          {page === "Device Detail" && (
            <DeviceDetail
              device={selectedDevice}
              site={sites.find((s) => s.id === selectedDevice.site_id)}
              setPage={setPage}
            />
          )}
          {page === "Backups" && (
            <Backups
              selectedDeviceId={selectedDeviceId}
              setPage={setPage}
              setSelectedDeviceId={setSelectedDeviceId}
              setSelectedSiteId={setSelectedSiteId}
            />
          )}
          {page === "Updates" && <Updates />}
          {page === "Internal Diagnostics" && <Diagnostics />}
          {page === "Alerts" && (
            <Alerts
              setPage={setPage}
              setSelectedSiteId={setSelectedSiteId}
              setSelectedDeviceId={setSelectedDeviceId}
            />
          )}
          {page === "Activity" && <Activity />}
          {page === "Internal Audit" && <Audit />}
          {page === "Internal Admin" && <Admin />}
          {page === "Schema Coverage" && <SchemaCoverage />}
        </main>
      </div>
      {showWiring && <WiringInspector active={activeWiring} />}
    </div>
    </DataWiringContext.Provider>
  );
}

function AuthExperience({ page, setPage }) {
  if (page === "Sign Up") {
    return <SignUpPage setPage={setPage} />;
  }

  if (page === "Password Reset") {
    return <PasswordResetPage setPage={setPage} />;
  }

  return <LoginPage setPage={setPage} />;
}

function AuthShell({ eyebrow, title, subtitle, children, asideTitle, asideItems, currentPage, setPage }) {
  const authNavItems = ["Login", "Sign Up", "Password Reset"];

  return (
    <div className="min-h-screen bg-slate-100 text-slate-950">
      <div className="grid min-h-screen lg:grid-cols-[minmax(0,0.95fr)_minmax(480px,1.05fr)]">
        <section className="hidden border-r border-slate-300 bg-slate-950 px-10 py-10 text-white lg:flex lg:flex-col lg:justify-between">
          <div>
            <div className="text-xl font-semibold">HomeSignal</div>
            <div className="mt-1 text-sm text-slate-300">Home Assistant management</div>
            <div className="mt-6 flex flex-wrap gap-2">
              {authNavItems.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setPage(item)}
                  className={`rounded-md border px-3 py-1.5 text-xs font-semibold ${
                    currentPage === item
                      ? "border-sky-300 bg-sky-300 text-slate-950"
                      : "border-white/15 bg-white/5 text-slate-200 hover:bg-white/10"
                  }`}
                >
                  {item}
                </button>
              ))}
              <button
                type="button"
                onClick={() => setPage("Dashboard")}
                className="rounded-md border border-white/15 bg-white/5 px-3 py-1.5 text-xs font-semibold text-slate-200 hover:bg-white/10"
              >
                Portal
              </button>
            </div>
          </div>

          <div className="max-w-lg">
            <div className="text-sm font-semibold uppercase tracking-normal text-sky-300">{asideTitle}</div>
            <div className="mt-4 space-y-4">
              {asideItems.map((item) => (
                <div key={item.title} className="rounded-md border border-white/10 bg-white/5 p-4">
                  <div className="font-semibold">{item.title}</div>
                  <div className="mt-1 text-sm leading-6 text-slate-300">{item.detail}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        <main className="flex items-center justify-center px-5 py-10">
          <section className="w-full max-w-md">
            <div className="mb-8 lg:hidden">
              <div className="text-xl font-semibold">HomeSignal</div>
              <div className="mt-1 text-sm text-slate-500">Home Assistant management</div>
              <div className="mt-4 flex flex-wrap gap-2">
                {authNavItems.map((item) => (
                  <button
                    key={item}
                    type="button"
                    onClick={() => setPage(item)}
                    className={`rounded-md border px-3 py-1.5 text-xs font-semibold ${
                      currentPage === item
                        ? "border-slate-950 bg-slate-950 text-white"
                        : "border-slate-300 bg-white text-slate-700"
                    }`}
                  >
                    {item}
                  </button>
                ))}
                <button
                  type="button"
                  onClick={() => setPage("Dashboard")}
                  className="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700"
                >
                  Portal
                </button>
              </div>
            </div>

            <div className="rounded-md border border-slate-300 bg-white p-6 shadow-sm">
              <div className="text-xs font-semibold uppercase tracking-normal text-slate-500">{eyebrow}</div>
              <h1 className="mt-2 text-2xl font-semibold text-slate-950">{title}</h1>
              <p className="mt-2 text-sm leading-6 text-slate-600">{subtitle}</p>
              {children}
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

function LoginPage({ setPage }) {
  return (
    <AuthShell
      currentPage="Login"
      setPage={setPage}
      eyebrow="Sign in"
      title="Manage Home Assistant sites"
      subtitle="Use your HomeSignal account to review site health, backups, updates, alerts, and pairing."
      asideTitle="Built for managed homes"
      asideItems={[
        { title: "Fleet-first view", detail: "Start with the sites and Home Assistant instances that need attention." },
        { title: "Operational signal", detail: "Presence, backups, updates, and alerts are projected into one workbench." },
        { title: "Customer-safe controls", detail: "Sensitive device identity and internal support findings stay out of the customer UI." },
      ]}
    >
      <form
        className="mt-6 space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          setPage("Dashboard");
        }}
      >
        <AuthInput label="Email" type="email" placeholder="you@example.com" autoComplete="email" />
        <AuthInput label="Password" type="password" placeholder="Password" autoComplete="current-password" />

        <div className="flex items-center justify-between gap-3 text-sm">
          <label className="flex items-center gap-2 text-slate-600">
            <input type="checkbox" className="h-4 w-4 rounded border-slate-300" />
            Keep me signed in
          </label>
          <button type="button" onClick={() => setPage("Password Reset")} className="font-medium text-sky-700 hover:text-sky-900">
            Forgot password?
          </button>
        </div>

        <button type="submit" className="w-full rounded-md bg-slate-950 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-800">
          Sign in
        </button>
      </form>

      <div className="mt-5 border-t border-slate-200 pt-5 text-center text-sm text-slate-600">
        New to HomeSignal?{" "}
        <button type="button" onClick={() => setPage("Sign Up")} className="font-semibold text-sky-700 hover:text-sky-900">
          Create an account
        </button>
      </div>
    </AuthShell>
  );
}

function SignUpPage({ setPage }) {
  return (
    <AuthShell
      currentPage="Sign Up"
      setPage={setPage}
      eyebrow="Create account"
      title="Start managing Home Assistant sites"
      subtitle="Create the integrator account that will own users, customers, sites, alert recipients, and pairing flows."
      asideTitle="What this creates"
      asideItems={[
        { title: "Integrator account", detail: "The account is the authority container for team members, customer records, and sites." },
        { title: "Seeded roles", detail: "Default roles are created structurally; custom role editing is not exposed in v0." },
        { title: "First site later", detail: "After signup, pair a Home Assistant instance into a customer/site record." },
      ]}
    >
      <form
        className="mt-6 space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          setPage("Dashboard");
        }}
      >
        <AuthInput label="Name" type="text" placeholder="Jamie Smith" autoComplete="name" />
        <AuthInput label="Work email" type="email" placeholder="you@company.com" autoComplete="email" />
        <AuthInput label="Company / account name" type="text" placeholder="Northstar Smart Homes" autoComplete="organization" />
        <AuthInput label="Password" type="password" placeholder="Create a password" autoComplete="new-password" />

        <button type="submit" className="w-full rounded-md bg-slate-950 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-800">
          Create account
        </button>
      </form>

      <div className="mt-5 border-t border-slate-200 pt-5 text-center text-sm text-slate-600">
        Already have an account?{" "}
        <button type="button" onClick={() => setPage("Login")} className="font-semibold text-sky-700 hover:text-sky-900">
          Sign in
        </button>
      </div>
    </AuthShell>
  );
}

function PasswordResetPage({ setPage }) {
  const [sent, setSent] = useState(false);

  return (
    <AuthShell
      currentPage="Password Reset"
      setPage={setPage}
      eyebrow="Password reset"
      title={sent ? "Check your email" : "Reset your password"}
      subtitle={
        sent
          ? "If the address is associated with a HomeSignal account, a reset link has been sent."
          : "Enter your account email and HomeSignal will send a password reset link."
      }
      asideTitle="Account recovery"
      asideItems={[
        { title: "Email-first recovery", detail: "The reset flow is tied to the verified user email on the account." },
        { title: "No account leakage", detail: "The confirmation message is the same whether or not the email exists." },
        { title: "Back to operations", detail: "After reset, users return to the same portal views and account-scoped access." },
      ]}
    >
      {!sent ? (
        <form
          className="mt-6 space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            setSent(true);
          }}
        >
          <AuthInput label="Email" type="email" placeholder="you@example.com" autoComplete="email" />
          <button type="submit" className="w-full rounded-md bg-slate-950 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-800">
            Send reset link
          </button>
        </form>
      ) : (
        <div className="mt-6 rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm leading-6 text-emerald-950">
          For the mock, this confirms the sent state. The real service should
          expire reset links quickly and never reveal whether an email exists.
        </div>
      )}

      <div className="mt-5 border-t border-slate-200 pt-5 text-center text-sm text-slate-600">
        <button type="button" onClick={() => setPage("Login")} className="font-semibold text-sky-700 hover:text-sky-900">
          Back to sign in
        </button>
      </div>
    </AuthShell>
  );
}

function AuthInput({ label, type, placeholder, autoComplete }) {
  return (
    <label className="block">
      <span className="text-sm font-medium text-slate-700">{label}</span>
      <input
        type={type}
        placeholder={placeholder}
        autoComplete={autoComplete}
        className="mt-1 w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-950 placeholder:text-slate-400 focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-100"
      />
    </label>
  );
}

function Dashboard({ setPage, setSelectedSiteId, setSelectedDeviceId }) {
  const [managedFilter, setManagedFilter] = useState("attention");
  const [managedSort, setManagedSort] = useState("priority");
  const [expandedAttentionSiteId, setExpandedAttentionSiteId] = useState("top");
  const connectedDevices = devices.filter((device) => device.presence === "online").length;
  const needsAttentionDevices = devices.filter((device) => {
    const backup = backups.find((item) => item.device_id === device.id);
    const haAppBehind = device.latest_ha_app_version && device.ha_app_version !== device.latest_ha_app_version;
    const haBehind = device.latest_home_assistant_version && device.home_assistant_version !== device.latest_home_assistant_version;
    return device.presence !== "online" || backup?.status === "failed" || haAppBehind || haBehind;
  });
  const failedBackups = backups.filter((backup) => backup.status === "failed").length;
  const haAppDrift = devices.filter((device) => device.latest_ha_app_version && device.ha_app_version !== device.latest_ha_app_version).length;
  const attentionSiteCount = new Set(needsAttentionDevices.map((device) => device.site_id)).size;
  const attentionVerb = attentionSiteCount === 1 ? "needs" : "need";
  const attentionSites = sites
    .map((site) => {
      const device = devices.find((item) => item.site_id === site.id);
      const backup = backups.find((item) => item.site_id === site.id);
      const customer = customers.find((item) => item.id === site.customer_record_id);
      const issues = [];

      if (device?.presence !== "online") {
        issues.push({
          label: "Disconnected",
          detail: `Last seen ${formatRelativeTime(device?.last_seen_at)}`,
          severity: "critical",
        });
      }

      if (backup?.status === "failed") {
        issues.push({
          label: "Backup failed",
          detail: `Last success ${formatDay(backup.last_success_at)}`,
          severity: "critical",
        });
      }

      if (device?.latest_ha_app_version && device.ha_app_version !== device.latest_ha_app_version) {
        issues.push({
          label: "HomeSignal App Update available",
          detail: `${device.ha_app_version} installed; ${device.latest_ha_app_version} available`,
          severity: "warning",
        });
      }

      if (device?.latest_home_assistant_version && device.home_assistant_version !== device.latest_home_assistant_version) {
        issues.push({
          label: "HA update available",
          detail: `${device.home_assistant_version} installed; ${device.latest_home_assistant_version} available`,
          severity: "info",
        });
      }

      return { site, customer, device, issues };
    })
    .filter((item) => item.issues.length > 0)
    .sort((a, b) => {
      const highA = a.issues.some((issue) => issue.severity === "critical") ? 1 : 0;
      const highB = b.issues.some((issue) => issue.severity === "critical") ? 1 : 0;
      return highB - highA || b.issues.length - a.issues.length;
    });
  const issueCount = attentionSites.reduce((sum, item) => sum + item.issues.length, 0);
  const visibleAttentionSites = [...attentionSites].sort((a, b) => {
    if (managedSort === "name") return a.site.name.localeCompare(b.site.name);
    if (managedSort === "recent") {
      return new Date(b.device?.last_seen_at || 0).getTime() - new Date(a.device?.last_seen_at || 0).getTime();
    }
    return 0;
  });
  const managedDevices = [...devices].sort((a, b) => {
    if (managedSort === "name") {
      const siteA = sites.find((site) => site.id === a.site_id)?.name || "";
      const siteB = sites.find((site) => site.id === b.site_id)?.name || "";
      return siteA.localeCompare(siteB);
    }

    if (managedSort === "recent") {
      return new Date(b.last_seen_at).getTime() - new Date(a.last_seen_at).getTime();
    }

    const attentionA = needsAttentionDevices.some((device) => device.id === a.id) ? 1 : 0;
    const attentionB = needsAttentionDevices.some((device) => device.id === b.id) ? 1 : 0;
    return attentionB - attentionA || new Date(b.last_seen_at).getTime() - new Date(a.last_seen_at).getTime();
  });
  const visibleManagedDevices = managedDevices.filter((device) => {
    const backup = backups.find((item) => item.device_id === device.id);
    const haAppBehind = device.latest_ha_app_version && device.ha_app_version !== device.latest_ha_app_version;
    const haBehind = device.latest_home_assistant_version && device.home_assistant_version !== device.latest_home_assistant_version;

    if (managedFilter === "online") return device.presence === "online";
    if (managedFilter === "backup") return backup?.status === "failed";
    if (managedFilter === "updates") return haAppBehind || haBehind;
    return true;
  });
  const topAttention = attentionSites[0];
  const expandedAttentionSite = expandedAttentionSiteId === "top" ? topAttention?.site.id : expandedAttentionSiteId;
  const heroCopy =
    topAttention
      ? `${topAttention.site.name} has ${topAttention.issues.length} current ${topAttention.issues.length === 1 ? "condition" : "conditions"}. ${connectedDevices} of ${devices.length} Home Assistant instances are online.`
      : "All managed Home Assistant sites are connected, backed up, and current.";
  const topReviewLabel = topAttention ? `Review ${topAttention.site.name}` : "Review devices";

  return (
    <Screen title="Dashboard" subtitle="A working view for the Home Assistant sites you manage.">
      <section className="rounded-md border border-slate-300 bg-white p-5">
        <div className="grid gap-5 lg:grid-cols-[1fr_auto] lg:items-start">
          <div>
            <div className="mb-2 flex items-center gap-2">
              <PresenceDot state={needsAttentionDevices.length > 0 ? "degraded" : "online"} wiringId="1" />
              <WiringValue id="1" className="text-sm font-medium text-slate-600">
                {needsAttentionDevices.length > 0 ? "Action required" : "All clear"}
              </WiringValue>
            </div>
            <h2 className="max-w-3xl text-3xl font-semibold tracking-normal text-slate-950">
              <WiringValue id="2">
                {attentionSiteCount} of {sites.length} managed sites {attentionVerb} attention
              </WiringValue>
            </h2>
            <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">
              <WiringValue id="3">
                {heroCopy}
              </WiringValue>
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => {
                  if (topAttention) {
                    setSelectedSiteId(topAttention.site.id);
                    if (topAttention.device) setSelectedDeviceId(topAttention.device.id);
                    setPage("Device Detail");
                  } else {
                    setPage("Devices");
                  }
                }}
                className="rounded-md bg-slate-950 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
              >
                {topReviewLabel}
              </button>
              <button
                type="button"
                onClick={() => setPage("Enrollment")}
                className="rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Pair Home Assistant
              </button>
            </div>
          </div>

          <div className="min-w-[210px] rounded-md border border-slate-200 bg-slate-50 p-4">
            <div className="text-xs font-semibold uppercase tracking-normal text-slate-500">Today</div>
            <div className="mt-3 space-y-2 text-sm text-slate-700">
              <DashboardSummaryLine label="Sites" value={`${sites.length} managed`} wiringId="4" />
              <DashboardSummaryLine label="Online" value={`${connectedDevices}/${devices.length}`} wiringId="5" />
              <DashboardSummaryLine label="Open issues" value={issueCount} wiringId="6" />
            </div>
          </div>
        </div>
      </section>

      <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-5">
        <DashboardSignal label="Online" value={`${connectedDevices}/${devices.length}`} wiringId="5" tone={connectedDevices === devices.length ? "success" : "warning"} onClick={() => setPage("Devices")} />
        <DashboardSignal label="Sites with issues" value={attentionSiteCount} wiringId="7" tone={attentionSiteCount > 0 ? "warning" : "success"} onClick={() => setPage("Alerts")} />
        <DashboardSignal label="Backup issues" value={failedBackups} wiringId="8" tone={failedBackups > 0 ? "warning" : "success"} onClick={() => setPage("Backups")} />
        <DashboardSignal label="App drift" value={haAppDrift} wiringId="9" tone={haAppDrift > 0 ? "warning" : "success"} onClick={() => setPage("Updates")} />
        <DashboardSignal label="Email alerts" value="Soon" wiringId="10" tone="neutral" onClick={() => setPage("Alerts")} />
      </div>

      <Section title="Managed Home Assistants">
        <div className="mb-3">
          <p className="max-w-2xl text-sm text-slate-600">
            Filter the fleet by the work you are doing now. Needs attention is the default dashboard view.
          </p>
        </div>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap gap-2">
            {[
              ["all", "All"],
              ["attention", "Needs attention"],
              ["online", "Online"],
              ["backup", "Backup issues"],
              ["updates", "Updates"],
            ].map(([key, label]) => (
              <button
                key={key}
                type="button"
                onClick={() => setManagedFilter(key)}
                className={`rounded-md border px-2.5 py-1 text-sm transition ${
                  managedFilter === key
                    ? "border-slate-400 bg-slate-100 font-medium text-slate-950"
                    : "border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:bg-slate-50"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-3 text-sm">
            <span className="text-xs font-semibold uppercase tracking-normal text-slate-400">Sort by</span>
            {[
              ["priority", "Priority", "down"],
              ["recent", "Last seen", "down"],
              ["name", "Name", "up"],
            ].map(([key, label, direction]) => (
              <button
                key={key}
                type="button"
                onClick={() => setManagedSort(key)}
                className={`inline-flex items-center gap-1 text-sm hover:text-slate-950 ${
                  managedSort === key ? "font-semibold text-slate-950" : "font-medium text-slate-500"
                }`}
              >
                {label}
                <span aria-hidden="true" className="text-xs text-slate-400">
                  {direction === "up" ? "↑" : "↓"}
                </span>
              </button>
            ))}
          </div>
        </div>
        {managedFilter === "attention" ? (
          <div className="overflow-hidden rounded-md border border-slate-300 bg-white">
            <div className="divide-y divide-slate-200">
              {visibleAttentionSites.map((item) => (
                <DashboardAttentionSite
                  key={item.site.id}
                  item={item}
                  primary={item.site.id === expandedAttentionSite}
                  expanded={item.site.id === expandedAttentionSite}
                  framed
                  onToggle={() => setExpandedAttentionSiteId(item.site.id === expandedAttentionSite ? null : item.site.id)}
                  onOpen={() => {
                    setSelectedSiteId(item.site.id);
                    if (item.device) setSelectedDeviceId(item.device.id);
                    setPage("Device Detail");
                  }}
                />
              ))}
            </div>
          </div>
        ) : (
          <ManagedHomeAssistantList
            deviceItems={visibleManagedDevices}
            onOpen={(device) => {
              setSelectedSiteId(device.site_id);
              setSelectedDeviceId(device.id);
              setPage("Device Detail");
            }}
          />
        )}
      </Section>

      <Section title="Latest activity">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <p className="max-w-2xl text-sm text-slate-600">
            Recent alerts, backups, updates, and pairing activity across managed sites.
          </p>
          <TextButton onClick={() => setPage("Activity")}>View all activity</TextButton>
        </div>
        <div className="rounded-md border border-slate-200">
          <ActivityTimeline events={activityEvents.slice(0, 4)} compact />
        </div>
      </Section>
    </Screen>
  );
}

function DashboardSummaryLine({ label, value, wiringId }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span>{label}</span>
      <WiringValue id={wiringId} source={`Dashboard summary value for "${label}".`} className="font-semibold text-slate-950">
        {value}
      </WiringValue>
    </div>
  );
}

function DashboardSignal({ label, value, wiringId, tone, onClick }) {
  const styles =
    tone === "success"
      ? "border-emerald-200 bg-white text-emerald-800"
      : tone === "warning"
        ? "border-amber-200 bg-white text-amber-900"
        : "border-slate-300 bg-white text-slate-700";
  const dot = tone === "success" ? "online" : tone === "warning" ? "degraded" : "offline";

  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-left text-sm transition hover:border-slate-500 focus:outline-none focus:ring-2 focus:ring-slate-900 ${styles}`}
    >
      <span className="flex items-center gap-2 font-medium">
        <PresenceDot state={dot} wiringId={wiringId} />
        {label}
      </span>
      <WiringValue id={wiringId} source={`Dashboard signal value for "${label}" is derived from current mock arrays: devices, sites, backups, and update/version fields.`} className="font-semibold text-slate-950">
        {value}
      </WiringValue>
    </button>
  );
}

function wiringMeta(id, fallback) {
  const hint = wiringHints[id];
  if (!hint) {
    return {
      status: "missing",
      text: fallback || "Source hint not yet mapped.",
    };
  }

  if (typeof hint === "string") {
    return { status: "partial", text: hint };
  }

  return hint;
}

function wiringSource(id, fallback) {
  return wiringMeta(id, fallback).text;
}

function wiringStatus(id) {
  return wiringMeta(id).status;
}

function wiringClasses(id) {
  const status = wiringStatus(id);

  if (status === "backed") {
    return {
      wrap: "bg-emerald-50 ring-emerald-300",
      badge: "border-emerald-400 bg-white text-emerald-800",
      popover: "border-emerald-200",
      title: "text-emerald-900",
    };
  }

  if (status === "missing") {
    return {
      wrap: "bg-rose-50 ring-rose-300",
      badge: "border-rose-300 bg-white text-rose-800",
      popover: "border-rose-200",
      title: "text-rose-900",
    };
  }

  return {
    wrap: "bg-amber-50 ring-amber-300",
    badge: "border-amber-400 bg-white text-amber-900",
    popover: "border-amber-200",
    title: "text-amber-900",
  };
}

function WiringInspector({ active }) {
  const status = active?.status || "idle";
  const statusClass =
    status === "backed"
      ? "border-emerald-300 bg-emerald-50 text-emerald-950"
      : status === "partial"
        ? "border-amber-300 bg-amber-50 text-amber-950"
        : status === "missing"
          ? "border-rose-300 bg-rose-50 text-rose-950"
          : "border-slate-300 bg-white text-slate-700";

  return (
    <aside className={`fixed bottom-4 right-4 z-[100] w-[360px] rounded-md border p-3 text-sm shadow-lg ${statusClass}`}>
      {active ? (
        <>
          <div className="mb-1 flex items-center justify-between gap-3">
            <span className="font-semibold">ID {active.id}</span>
            <span className="rounded border border-current px-1.5 py-0.5 text-[11px] font-semibold uppercase">{active.status}</span>
          </div>
          <div className="text-xs leading-5">{active.source}</div>
        </>
      ) : (
        <div className="text-xs leading-5">Hover a numbered value to inspect schema support.</div>
      )}
    </aside>
  );
}

function WiringValue({ children, id, source, className = "" }) {
  const showWiring = useContext(DataWiringContext);
  const label =
    typeof children === "string" || typeof children === "number"
      ? children
      : "Dynamic value";
  const displayId = id || "0";
  const hint = wiringSource(displayId, source);
  const classes = wiringClasses(displayId);

  if (!showWiring) {
    return <span className={className}>{children}</span>;
  }

  return (
    <span
      className={`group relative rounded-sm px-1 ring-1 ring-inset cursor-help ${classes.wrap} ${className}`}
      title={`${displayId}: ${hint}`}
      aria-label={`${label}. ${displayId}: ${hint}`}
      data-wiring-id={displayId}
      data-wiring-source={hint}
      data-wiring-status={wiringStatus(displayId)}
    >
      <WiringIdBadge id={displayId} />
      {children}
      <span className={`pointer-events-none absolute left-0 top-full z-50 mt-1 hidden w-80 rounded-md border bg-white p-3 text-left text-xs leading-5 text-slate-700 shadow-lg group-hover:block ${classes.popover}`}>
        <span className={`mb-1 block font-semibold ${classes.title}`}>{displayId}</span>
        <span className="block">{hint}</span>
      </span>
    </span>
  );
}

function WiringFrame({ children, id, source, className = "" }) {
  const showWiring = useContext(DataWiringContext);
  const displayId = id || "0";
  const hint = wiringSource(displayId, source);
  const classes = wiringClasses(displayId);

  if (!showWiring) {
    return children;
  }

  return (
    <span
      className={`group relative inline-flex rounded-sm p-0.5 ring-1 ring-inset cursor-help ${classes.wrap} ${className}`}
      title={`${displayId}: ${hint}`}
      aria-label={`${displayId}: ${hint}`}
      data-wiring-id={displayId}
      data-wiring-source={hint}
      data-wiring-status={wiringStatus(displayId)}
    >
      <span className="absolute -left-1.5 -top-1.5 z-10">
        <WiringIdBadge id={displayId} />
      </span>
      {children}
      <span className={`pointer-events-none absolute left-0 top-full z-50 mt-1 hidden w-80 rounded-md border bg-white p-3 text-left text-xs leading-5 text-slate-700 shadow-lg group-hover:block ${classes.popover}`}>
        <span className={`mb-1 block font-semibold ${classes.title}`}>{displayId}</span>
        <span className="block">{hint}</span>
      </span>
    </span>
  );
}

function WiringIdBadge({ id }) {
  const classes = wiringClasses(id).badge;

  return (
    <span className={`mr-1 inline-flex min-w-[1.15rem] items-center justify-center rounded border px-1 align-middle text-[10px] font-semibold leading-4 ${classes}`}>
      {id}
    </span>
  );
}

function DashboardAttentionSite({ item, onOpen, onToggle, primary = false, expanded = false, framed = false }) {
  const primaryIssue = item.issues[0];
  const extraIssueCount = Math.max(0, item.issues.length - 1);
  const issueLabel = `${item.issues.length} ${item.issues.length === 1 ? "issue" : "issues"}`;
  const shellClass = framed
    ? `bg-white ${primary ? "ring-1 ring-inset ring-slate-400" : ""}`
    : `overflow-hidden rounded-md border bg-white ${primary ? "border-slate-400 shadow-sm" : "border-slate-200"}`;

  return (
    <div className={shellClass}>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={expanded}
        className={`w-full px-4 py-3 text-left hover:bg-slate-50 focus:outline-none focus-visible:bg-slate-50 ${expanded ? "border-b border-slate-200" : ""}`}
      >
        <div className="grid gap-3 md:grid-cols-[minmax(170px,1fr)_minmax(190px,1.2fr)_78px] md:items-center">
          <div className="flex items-center gap-2">
            <HomeIcon category={item.site.site_category} wiringId="30" />
            <div>
              <div className="font-semibold text-slate-950">
                <WiringValue id="11">
                  {item.site.name}
                </WiringValue>
              </div>
              <div className="text-sm text-slate-500">
                <WiringValue id="12">
                  {item.customer?.display_name || "No customer"} · {item.site.location}
                </WiringValue>
              </div>
            </div>
          </div>

          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <PresenceDot state={primaryIssue.severity === "info" ? "offline" : "degraded"} wiringId="31" />
              <WiringValue id="13" className="truncate text-sm font-medium text-slate-900">
                {primaryIssue.label}
              </WiringValue>
            </div>
            <div className="mt-1 truncate text-xs text-slate-500">
              <WiringValue id="14">
                {primaryIssue.detail}
                {extraIssueCount > 0 && ` +${extraIssueCount}`}
              </WiringValue>
            </div>
          </div>

          <div className="flex items-center gap-3 text-xs font-medium text-slate-500 md:justify-end">
            <WiringValue id="15" className="whitespace-nowrap">
              {issueLabel}
            </WiringValue>
            <span
              aria-hidden="true"
              className="inline-block h-2 w-2 border-b border-r border-slate-400 transition-transform"
              style={{ transform: expanded ? "rotate(225deg)" : "rotate(45deg)" }}
            />
          </div>
        </div>
      </button>

      <div
        aria-hidden={!expanded}
        className={`grid transition-[grid-template-rows] duration-200 ease-out ${expanded ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
      >
        <div className="overflow-hidden">
          <div className={`px-4 transition-opacity duration-200 ${expanded ? "py-4 opacity-100" : "py-0 opacity-0"}`}>
            <div className="grid gap-2 sm:grid-cols-2">
              {item.issues.map((issue) => (
                <div key={issue.label} className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2">
                  <div className="flex items-center gap-2">
                    <PresenceDot state={issue.severity === "info" ? "offline" : "degraded"} wiringId="31" />
                    <WiringValue id="16" className="text-sm font-medium text-slate-900">
                      {issue.label}
                    </WiringValue>
                  </div>
                  <div className="mt-1 text-xs text-slate-500">
                    <WiringValue id="17">
                      {issue.detail}
                    </WiringValue>
                  </div>
                </div>
              ))}
            </div>

            <button
              type="button"
              onClick={onOpen}
              tabIndex={expanded ? 0 : -1}
              className="mt-3 w-full rounded-md bg-slate-950 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
            >
              Review site
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function ManagedHomeAssistantList({ deviceItems, onOpen }) {
  return (
    <div className="overflow-hidden rounded-md border border-slate-300 bg-white">
      <div className="hidden grid-cols-[minmax(170px,1fr)_minmax(190px,1.2fr)_78px] gap-3 border-b border-slate-200 bg-slate-50 px-4 py-3 text-xs font-semibold uppercase tracking-normal text-slate-500 md:grid">
        <div>Site</div>
        <div>Status</div>
        <div className="text-right">Action</div>
      </div>
      <div className="divide-y divide-slate-200">
        {deviceItems.map((device) => {
          const site = sites.find((item) => item.id === device.site_id);
          const customer = customers.find((item) => item.id === site?.customer_record_id);
          const backup = backups.find((item) => item.device_id === device.id);

          return (
            <ManagedHomeAssistantRow
              key={device.id}
              device={device}
              site={site}
              customer={customer}
              backup={backup}
              onOpen={() => onOpen(device, site)}
            />
          );
        })}
      </div>
    </div>
  );
}

function ManagedHomeAssistantRow({ device, site, customer, backup, onOpen }) {
  const showWiring = useContext(DataWiringContext);
  const connected = device.presence === "online";
  const backupOk = backup?.status === "succeeded";
  const haAppBehind = device.latest_ha_app_version && device.ha_app_version !== device.latest_ha_app_version;
  const haBehind = device.latest_home_assistant_version && device.home_assistant_version !== device.latest_home_assistant_version;
  const needsReview = !connected || !backupOk || haAppBehind || haBehind;
  const primaryStatus = connected ? "Connected" : "Disconnected";
  const statusDetail = `HA ${device.home_assistant_version} · ${backupOk ? "Backup current" : "Backup failed"}`;
  const secondaryDetails = [
    haBehind ? { id: "22", label: `Update ${device.latest_home_assistant_version}` } : null,
    haAppBehind ? { id: "32", label: "HomeSignal App Update" } : null,
  ].filter(Boolean);

  return (
    <div className="w-full px-4 py-3 text-left">
      <div className="grid gap-3 md:grid-cols-[minmax(170px,1fr)_minmax(190px,1.2fr)_78px] md:items-center">
        <div className="flex min-w-0 items-center gap-2">
          <HomeIcon category={site?.site_category} wiringId="30" />
          <div className="min-w-0">
            <div className="font-semibold text-slate-950">
              <WiringValue id="18">
                {site?.name || "Unknown site"}
              </WiringValue>
            </div>
            <div className="text-sm text-slate-500">
              <WiringValue id="19">
                {customer?.display_name || "No customer"} · {site?.location || "No location"}
              </WiringValue>
            </div>
          </div>
        </div>

        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <PresenceDot state={connected ? "online" : "degraded"} wiringId="20" />
            <WiringValue id="20" className="truncate text-sm font-medium text-slate-900">
              {primaryStatus}
            </WiringValue>
          </div>
          <div className="mt-1 truncate text-xs text-slate-500">
            <WiringValue id="21">
              {statusDetail}
            </WiringValue>
          </div>
          {secondaryDetails.length > 0 && (
            <div className="mt-0.5 flex min-w-0 flex-wrap gap-x-1 gap-y-0.5 text-xs text-amber-700">
              {secondaryDetails.map((detail, index) => (
                <React.Fragment key={detail.id}>
                  {index > 0 && <span>·</span>}
                  <WiringValue id={detail.id}>{detail.label}</WiringValue>
                </React.Fragment>
              ))}
            </div>
          )}
        </div>

        <div className="md:text-right">
          <button
            type="button"
            onClick={onOpen}
            title={showWiring ? `23: ${wiringSource("23")}` : undefined}
            data-wiring-id={showWiring ? "23" : undefined}
            data-wiring-source={showWiring ? wiringSource("23") : undefined}
            className={`rounded-md border px-3 py-1.5 text-sm font-medium ${
              needsReview
                ? "border-slate-950 bg-slate-950 text-white hover:bg-slate-800"
                : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
            } ${showWiring ? "ring-1 ring-inset ring-rose-200" : ""}`}
          >
            {showWiring && <WiringIdBadge id="23" />}
            {needsReview ? "Review" : "View"}
          </button>
        </div>
      </div>
    </div>
  );
}

function DashboardDeviceRow({ device, site, customer, backup, onOpen }) {
  return (
    <button type="button" onClick={onOpen} className="w-full px-5 py-4 text-left hover:bg-slate-50">
      <div className="grid gap-5 lg:grid-cols-[minmax(220px,2fr)_minmax(145px,1fr)_minmax(150px,1fr)_minmax(150px,1fr)] lg:items-start">
        <div className="flex items-center gap-2">
          <HomeIcon category={site?.site_category} />
          <div>
            <div className="font-semibold text-slate-950">{site?.name || "Unknown site"}</div>
            <div className="text-sm text-slate-500">{customer?.display_name || "No customer"} · {site?.location || "No location"}</div>
          </div>
        </div>
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Home Assistant</div>
          <DeviceVersionSummary device={device} />
        </div>
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Backup</div>
          <BackupSummary backup={backup} />
        </div>
        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Connection</div>
          <ConnectionSummary device={device} />
        </div>
      </div>
    </button>
  );
}

function Accounts() {
  return (
    <Screen title="Account" subtitle="Integrator account and team authority container.">
      <TwoColumn>
        <Section title="Account profile">
          <Field label="Account ID" value={account.id} status="backed" source="accounts" />
          <Field label="Display name" value={account.name} status="backed" source="accounts" />
          <Field label="Account type" value={account.type} status="partial" source="account-site-service.md" />
          <Field label="Status" value={account.status} status="backed" source="accounts.status" />
          <Field label="Billing plan" value="Free / internal seed" status="missing" source="billing_subscriptions" />
        </Section>

        <Section title="Team and roles">
          <Field label="Users" value="Jamie, Alex" status="backed" source="users" />
          <Field label="Seeded roles" value="Owner, Admin, Technician, Read-only" status="backed" source="roles" />
          <Field label="Customer-defined role editor" value="Not in v0 UI" status="future" source="roles backend supports shape" />
          <Field label="Impersonation" value="Not supported" status="missing" source="explicitly out of scope" />
        </Section>
      </TwoColumn>
    </Screen>
  );
}

function Users() {
  return (
    <Screen title="Users" subtitle="Integrator team members, roles, invitations, and account access.">
      <Section title="Team members">
        <Table
          columns={["Name", "Email", "Role", "Access"]}
          rows={[
            ["Jamie Smith", "jamie@example.com", "Owner", <StatusPill state="online" label="Active" />],
            ["Alex Lee", "alex@example.com", "Technician", <StatusPill state="online" label="Active" />],
            ["Morgan Patel", "morgan@example.com", "Read-only", <StatusPill state="warning" label="Invited" />],
          ]}
        />
      </Section>

      <InternalNoteSection title="Role model">
        <Field label="Seeded defaults" value="Owner, Admin, Technician, Read-only" status="backed" source="roles" />
        <Field label="Customer-defined roles" value="Backend shape later; not exposed in v0 UI" status="future" source="roles backend supports shape" />
        <Field label="Impersonation" value="Not supported" status="missing" source="explicitly out of scope" />
      </InternalNoteSection>
    </Screen>
  );
}

function Sites({ selectedSiteId, setSelectedSiteId, setSelectedDeviceId, setPage }) {
  return (
    <Screen title="Sites" subtitle="Customer locations and account context for managed homes.">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-sm font-medium text-slate-700">
            Sites are customer/place records. Day-to-day operations live in Devices.
          </div>
          <div className="text-xs text-slate-500">
            Open a site to view the attached Home Assistant instance for now.
          </div>
        </div>
        <TextButton onClick={() => setPage("Enrollment")}>Pair Home Assistant</TextButton>
      </div>

      <section className="overflow-hidden rounded-md border border-slate-300 bg-white">
        <div className="hidden grid-cols-[minmax(230px,2fr)_minmax(145px,1fr)_minmax(150px,1fr)_minmax(150px,1fr)] gap-6 border-b border-slate-200 px-5 py-3 text-xs font-semibold uppercase tracking-normal text-slate-500 lg:grid">
          <div>Site</div>
          <div>Home Assistant</div>
          <div>Backup</div>
          <div>Connection</div>
        </div>

        <div className="divide-y divide-slate-200">
          {sites.map((site) => {
            const customer = customers.find((item) => item.id === site.customer_record_id);
            const device = devices.find((item) => item.site_id === site.id);
            const backup = backups.find((item) => item.site_id === site.id);

            return (
              <button
                key={site.id}
                type="button"
                onClick={() => {
                  setSelectedSiteId(site.id);
                  if (device) setSelectedDeviceId(device.id);
                  setPage("Device Detail");
                }}
                className="w-full px-5 py-5 text-left hover:bg-slate-50"
              >
                <div className="grid gap-5 lg:grid-cols-[minmax(230px,2fr)_minmax(145px,1fr)_minmax(150px,1fr)_minmax(150px,1fr)] lg:items-start">
                  <div className="flex items-center gap-2">
                    <HomeIcon category={site.site_category} />
                    <div>
                      <div className="font-semibold text-slate-950">{site.name}</div>
                      <div className="text-sm text-slate-500">{customer?.display_name || "No customer"} · {site.location}</div>
                    </div>
                  </div>

                  <div>
                    <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Home Assistant</div>
                    {device ? <DeviceVersionSummary device={device} /> : <EmptySummary title="Not connected" detail="No app paired" />}
                  </div>

                  <div>
                    <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Backup</div>
                    <BackupSummary backup={backup} />
                  </div>

                  <div>
                    <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Connection</div>
                    {device ? <ConnectionSummary device={device} /> : <EmptySummary title="Not connected" detail="Pair Home Assistant" />}
                  </div>
                </div>
              </button>
            );
          })}
        </div>
      </section>

    </Screen>
  );
}

function Enrollment() {
  const [enrollmentState, setEnrollmentState] = useState("choose");
  const [targetSiteId, setTargetSiteId] = useState("");
  const [reviewSiteId, setReviewSiteId] = useState("");
  const targetSite = sites.find((site) => site.id === targetSiteId);
  const lockedSite = sites.find((site) => site.id === reviewSiteId);
  const activeSite = lockedSite || targetSite;
  const activeCustomer = customers.find((customer) => customer.id === activeSite?.customer_record_id);
  const canEnterCode = Boolean(targetSite);
  const isReviewing = enrollmentState === "review" || enrollmentState === "connected";

  if (enrollmentState === "review" && activeSite) {
    return (
      <Screen title="Review pairing code" subtitle="Confirm the site and customer before sharing a pairing code.">
        <section className="rounded-md border border-slate-300 bg-white p-5">
          <div className="grid gap-5 lg:grid-cols-[1.2fr_1fr]">
            <div>
              <div className="mb-2 text-xs font-semibold uppercase tracking-normal text-slate-500">Connect to</div>
              <div className="flex items-center gap-3 rounded-md border border-slate-200 bg-slate-50 p-4">
                <HomeIcon category={activeSite.site_category} />
                <div>
                  <div className="text-lg font-semibold">{activeSite.name}</div>
                  <div className="text-sm text-slate-500">
                    {activeCustomer?.display_name || "No customer"} · {activeSite.location}
                  </div>
                </div>
              </div>

              <div className="mt-5 grid gap-3 md:grid-cols-2">
                <ReviewFact label="Pairing code" value="Secure pairing code" />
                <ReviewFact label="Created by" value="Maya Patel" />
                <ReviewFact label="Invite expiry" value="72 hours" />
                <ReviewFact label="History transfer" value="No history transfer" warning />
              </div>
            </div>

            <div className="rounded-md border border-[#b3e5fc] bg-[#e1f5fe] p-4">
              <p className="text-sm leading-6 text-slate-700">You are creating an invite for</p>
              <div className="mt-3 rounded-md border border-[#81d4fa] bg-white p-4">
                <div className="text-xl font-semibold text-slate-950">{activeSite.name}</div>
                <div className="mt-1 text-sm text-slate-500">
                  {activeCustomer?.display_name || "No customer"} · {activeSite.location}
                </div>
              </div>
              <p className="mt-3 text-xs leading-5 text-slate-600">
                Creating the invite does not claim a device yet. The Home Assistant app must verify the invite details locally before it can confirm pairing.
              </p>
              <div className="mt-5 flex flex-col gap-2 sm:flex-row">
                <button
                  type="button"
                  onClick={() => setEnrollmentState("connected")}
                  className="rounded-md bg-[#03a9f4] px-4 py-2 text-sm font-medium text-white hover:bg-[#0288d1]"
                >
                  Create pairing code
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setReviewSiteId("");
                    setEnrollmentState("code");
                  }}
                  className="rounded-md border border-[#81d4fa] bg-white px-4 py-2 text-sm font-medium text-[#0277bd] hover:bg-[#f5fcff]"
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        </section>
      </Screen>
    );
  }

  if (enrollmentState === "connected" && activeSite) {
    return (
      <Screen title="Pairing code created" subtitle="Share this code with the Home Assistant administrator.">
        <section className="rounded-md border border-slate-300 bg-white p-5">
          <div className="flex items-center gap-3">
            <PresenceDot state="online" />
            <div>
              <div className="text-lg font-semibold">{activeSite.name}</div>
              <div className="text-sm text-slate-500">
                {activeCustomer?.display_name || "No customer"} · {activeSite.location}
              </div>
            </div>
          </div>
          <p className="mt-4 max-w-2xl text-sm leading-6 text-slate-600">
            The app user can enter this pairing code locally, review the integrator/site/customer details, and then confirm pairing.
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            <TextButton>Open device</TextButton>
            <TextButton onClick={() => {
              setTargetSiteId("");
              setReviewSiteId("");
              setEnrollmentState("choose");
            }}>
              Create another
            </TextButton>
          </div>
        </section>
      </Screen>
    );
  }

  return (
      <Screen title="Create Home Assistant pairing code" subtitle="Choose a customer site, then create a pairing code for the local app user.">
      <div className="mb-4 flex flex-wrap gap-2">
        {[
          ["choose", "Choose site"],
          ["code", "Create invite"],
          ["review", "Review"],
          ["connected", "Connected"],
        ].map(([key, label]) => (
          <button
            key={key}
            type="button"
            onClick={() => setEnrollmentState(key)}
            className={`rounded-md border px-3 py-1.5 text-sm ${
              enrollmentState === key
                ? "border-[#03a9f4] bg-[#e3f2fd] text-[#0277bd]"
                : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      <section className="rounded-md border border-slate-300 bg-white p-5">
        <div className="grid gap-5 lg:grid-cols-[1.2fr_1fr]">
          <div>
            <div className="mb-2 text-xs font-semibold uppercase tracking-normal text-slate-500">
              Customer site
            </div>
            <select
              value={targetSiteId}
              onChange={(event) => {
                setTargetSiteId(event.target.value);
                setReviewSiteId("");
                if (event.target.value && enrollmentState === "choose") setEnrollmentState("code");
              }}
              disabled={isReviewing}
              className="w-full rounded-md border border-slate-300 bg-white px-3 py-3 text-sm text-slate-900"
            >
              <option value="">Search or choose a customer site...</option>
              {sites.map((site) => {
                const customer = customers.find((item) => item.id === site.customer_record_id);
                return (
                  <option key={site.id} value={site.id}>
                    {customer?.display_name || "No customer"} - {site.name} - {site.location}
                  </option>
                );
              })}
            </select>

            {activeSite ? (
              <div className="mt-3 flex items-center gap-3 rounded-md border border-slate-200 bg-slate-50 p-4">
                <HomeIcon category={activeSite.site_category} />
                <div>
                  <div className="flex items-center gap-2">
                    <div className="text-lg font-semibold">{activeSite.name}</div>
                    {isReviewing && <span className="rounded bg-slate-200 px-2 py-0.5 text-xs text-slate-600">Locked for review</span>}
                  </div>
                  <div className="text-sm text-slate-500">
                    {activeCustomer?.display_name || "No customer"} · {activeSite.location}
                  </div>
                </div>
              </div>
            ) : (
              <div className="mt-3 rounded-md border border-dashed border-slate-300 bg-slate-50 p-4 text-sm text-slate-600">
                Pick the customer site that this Home Assistant installation should report into.
              </div>
            )}

            <p className="mt-4 max-w-xl text-sm leading-6 text-slate-600">
              Create a pairing code for this site, then share it through HomeSignal email or your normal customer handoff. The app will verify the pairing details locally before pairing.
            </p>
          </div>

          <div className="rounded-md border border-[#b3e5fc] bg-[#e1f5fe] p-4">
            <label className="text-sm font-semibold text-[#01579b]" htmlFor="claim-invite-code">
              Pairing code
            </label>
            <input
              id="claim-invite-code"
              value="4f8b0e7a-0f7d-45f8-8b8b-1e25f4d68a10"
              readOnly
              className={`mt-2 w-full rounded-md border px-4 py-3 font-mono text-sm font-semibold tracking-normal ${
                canEnterCode
                  ? "border-[#81d4fa] bg-white text-slate-950"
                  : "border-slate-300 bg-slate-100 text-slate-400"
              }`}
            />
            <div className="mt-3 flex flex-wrap gap-2">
              <button
                type="button"
                disabled={!canEnterCode}
                onClick={() => {
                  setReviewSiteId(targetSiteId);
                  setEnrollmentState("review");
                }}
                className={`rounded-md px-4 py-2 text-sm font-medium text-white ${
                  canEnterCode ? "bg-[#03a9f4] hover:bg-[#0288d1]" : "bg-slate-300"
                }`}
              >
                Review invite
              </button>
              <button
                type="button"
                onClick={() => {
                  setReviewSiteId("");
                  setEnrollmentState("choose");
                }}
                className="rounded-md border border-[#81d4fa] bg-white px-4 py-2 text-sm font-medium text-[#0277bd] hover:bg-[#f5fcff]"
              >
                Change site
              </button>
            </div>
          </div>
        </div>
      </section>

      <Section title="What happens next">
        <div className="grid gap-3 md:grid-cols-3">
          <Action label="Create site-bound pairing code" status="backed" source="device_claim_invites" />
          <Action label="Local app verifies details" status="backed" source="device_claim_verifications" />
          <Action label="Confirm pairing locally" status="backed" source="devices + device_credentials" />
        </div>
      </Section>
    </Screen>
  );
}

function DeviceFleet({ setPage, setSelectedDeviceId, setSelectedSiteId }) {
  return (
    <Screen title="Devices" subtitle="Fleet workbench for managed Home Assistant instances.">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-sm font-medium text-slate-700">
            <WiringValue id="24">
              {devices.length} Home Assistant instances across {sites.length} active sites
            </WiringValue>
          </div>
          <div className="text-xs text-slate-500">
            Use the row action to review backups, versions, connection state, and actions for that Home Assistant instance.
          </div>
        </div>
        <TextButton onClick={() => setPage("Enrollment")}>Pair Home Assistant</TextButton>
      </div>

      <ManagedHomeAssistantList
        deviceItems={devices}
        onOpen={(device, site) => {
          setSelectedDeviceId(device.id);
          if (site) setSelectedSiteId(site.id);
          setPage("Device Detail");
        }}
      />

    </Screen>
  );
}

function DeviceDetail({ device, site, setPage }) {
  const backup = backups.find((item) => item.device_id === device.id);
  const offline = device.presence !== "online";

  return (
    <Screen title="Home Assistant" subtitle={`${site?.name || "Unknown site"} - ${site?.location || "Unknown location"}`}>
      <div className="mb-3">
        <TextButton onClick={() => setPage("Devices")}>Back to devices</TextButton>
      </div>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-md border border-slate-300 bg-white p-4">
        <div>
          <div className="flex items-center gap-2">
            <PresenceDot state={device.presence} />
            <span className="text-lg font-semibold capitalize">{device.presence}</span>
            {offline && <span className="text-sm text-slate-500">Last seen {device.last_seen_at}</span>}
          </div>
          <div className="mt-1 text-sm text-slate-600">
            Home Assistant managed by HomeSignal app
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <TextButton>Trigger backup</TextButton>
          <TextButton>View backups</TextButton>
          <TextButton>Update status</TextButton>
          <TextButton>Advanced</TextButton>
        </div>
      </div>

      <TwoColumn>
        <Section title="Home Assistant">
          <Field label="Site" value={site?.name} status="backed" source="sites" />
          <Field label="Location" value={site?.location} status="backed" source="sites.location" />
          <VersionField label="Home Assistant version" current={device.home_assistant_version} latest={device.latest_home_assistant_version} source="device_latest_state + Home Assistant version catalog cache" />
          <VersionField label="Supervisor version" current={device.supervisor_version} latest={device.latest_supervisor_version} source="device_latest_state; no customer advisory unless catalog is defined" />
          <VersionField label="HomeSignal app version" current={device.ha_app_version} latest={device.latest_ha_app_version} source="device_latest_state + homesignal_edge.update" />
          <Field label="Storage health" value={device.storage_status} status="partial" source="reported latest-state field; exact derivation needs schema" />
          <Field label="Topology browser" value="Not exposed v0" status="future" source="topology_snapshots" />
        </Section>

        <Section title="Backups for this Home Assistant instance">
          <Field label="Latest backup" value={backup?.status || "None"} status="backed" source="backups.status" />
          <Field label="Last success" value={backup?.last_success_at || "None"} status="backed" source="backups.last_success_at" />
          <Field label="Offsite artifact" value={backup?.artifact_status || "None"} status="backed" source="Backup Service + Artifact Upload Broker" />
          <Field label="Restore backup" value="Future" status="future" source="not v0" />
        </Section>
      </TwoColumn>

      <InternalNoteSection title="Advanced">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <Action label="Request diagnostics" status="backed" source="Installed app consent + diagnostics guardrails" />
          <Action label="Restart app" status="partial" source="Product decision: likely allowed, command spec not pinned" />
          <Action label="Release device" status="backed" source="device lifecycle + audit_events" />
          <Action label="Delete / archive site" status="backed" source="Account / Site archive semantics" />
        </div>
        <div className="mt-4 rounded-md border border-slate-200 bg-slate-50 p-3 text-sm text-slate-600">
          Internal-only identifiers, credential details, raw edge projection, and
          publish-policy repair controls belong under Internal Admin.
        </div>
      </InternalNoteSection>
    </Screen>
  );
}

function Backups({ selectedDeviceId, setPage, setSelectedDeviceId, setSelectedSiteId }) {
  const backupRows = backups.map((backup) => {
    const site = sites.find((item) => item.id === backup.site_id);
    const device = devices.find((item) => item.id === backup.device_id);
    const customer = customers.find((item) => item.id === site?.customer_record_id);

    return { backup, site, device, customer };
  });
  const attentionRows = backupRows.filter(({ backup }) => backup.status !== "succeeded");
  const storedCopies = backupRows.filter(({ backup }) => backup.artifact_status === "stored").length;
  const protectedCount = backupRows.filter(({ backup }) => backup.status === "succeeded" && backup.artifact_status === "stored").length;
  const selectedRow =
    backupRows.find(({ device }) => device?.id === selectedDeviceId) ||
    attentionRows[0] ||
    backupRows[0];
  const selectedNeedsAttention = selectedRow?.backup.status !== "succeeded";

  const openDevice = (row) => {
    if (row.site) setSelectedSiteId(row.site.id);
    if (row.device) setSelectedDeviceId(row.device.id);
    setPage("Device Detail");
  };

  return (
    <Screen title="Backups" subtitle="Offsite backup status across managed Home Assistant instances.">
      <section className="grid items-start gap-3 lg:grid-cols-[minmax(0,1.5fr)_repeat(3,minmax(0,0.65fr))]">
        <div className={`rounded-md border bg-white p-4 ${selectedNeedsAttention ? "border-amber-300" : "border-emerald-300"}`}>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0">
              <StatusPill state={selectedNeedsAttention ? "warning" : "online"} label={selectedNeedsAttention ? "Needs attention" : "Protected"} />
              <h2 className="mt-3 text-lg font-semibold text-slate-950">
                {selectedRow?.site?.name || "Selected Home Assistant"}
              </h2>
              <p className="mt-1 max-w-2xl text-sm leading-6 text-slate-600">
                {selectedNeedsAttention
                  ? `Backup failed ${formatShortDate(selectedRow?.backup.last_failure_at)}. Last successful offsite candidate was ${formatShortDate(selectedRow?.backup.last_success_at)}.`
                  : `Latest backup completed ${formatShortDate(selectedRow?.backup.last_success_at)} and the offsite copy is available.`}
              </p>
            </div>
            {selectedRow && (
              <button
                type="button"
                onClick={() => openDevice(selectedRow)}
                className="rounded-md border border-slate-300 px-3 py-2 text-sm font-medium text-slate-800 hover:bg-slate-50"
              >
                Open device
              </button>
            )}
          </div>

          <div className="mt-4 grid gap-3 sm:grid-cols-3">
            <BackupFact label="Last success" value={formatShortDate(selectedRow?.backup.last_success_at)} />
            <BackupFact label="Offsite copy" value={selectedRow?.backup.artifact_status === "stored" ? `${selectedRow.backup.size_mb} MB stored` : "Not stored"} warning={selectedRow?.backup.artifact_status !== "stored"} />
            <BackupFact label="Connection" value={selectedRow?.device ? (selectedRow.device.presence === "online" ? "Connected" : `Last seen ${formatRelativeTime(selectedRow.device.last_seen_at)}`) : "Unknown"} warning={selectedRow?.device?.presence !== "online"} />
          </div>
        </div>

        <BackupMetric label="Protected" value={`${protectedCount}/${backupRows.length}`} detail="Current with offsite copy" tone={protectedCount === backupRows.length ? "success" : "neutral"} />
        <BackupMetric label="Needs attention" value={attentionRows.length} detail="Failed latest run" tone={attentionRows.length > 0 ? "warning" : "success"} />
        <BackupMetric label="Offsite copies" value={storedCopies} detail="Artifacts available" tone={storedCopies === backupRows.length ? "success" : "neutral"} />
      </section>

      <section className="overflow-hidden rounded-md border border-slate-300 bg-white">
        <div className="flex flex-wrap items-start justify-between gap-3 border-b border-slate-200 px-5 py-4">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-normal text-slate-700">Backup jobs</h2>
            <p className="mt-1 text-sm text-slate-500">Latest run, offsite copy, and connection state for each managed instance.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <StatusPill state={attentionRows.length > 0 ? "warning" : "online"} label={`${attentionRows.length} attention`} />
            <StatusPill state="neutral" label={`${storedCopies} stored`} withDot={false} />
          </div>
        </div>

        <div className="hidden grid-cols-[minmax(230px,1.6fr)_minmax(150px,0.9fr)_minmax(150px,0.9fr)_minmax(150px,0.9fr)_minmax(120px,auto)] gap-6 border-b border-slate-200 px-5 py-3 text-xs font-semibold uppercase tracking-normal text-slate-500 lg:grid">
          <div>Home Assistant</div>
          <div>Backup</div>
          <div>Offsite copy</div>
          <div>Connection</div>
          <div className="text-right">Action</div>
        </div>

        <div className="divide-y divide-slate-200">
          {backupRows.map((row) => (
            <BackupFleetRow
              key={row.backup.id}
              row={row}
              selected={row.device?.id === selectedRow?.device?.id}
              onOpen={() => openDevice(row)}
            />
          ))}
        </div>
      </section>

      <InternalNoteSection title="Backup product boundaries">
        <div className="grid gap-3 md:grid-cols-3">
          <Action label="Trigger backup" status="backed" source="commands + backups" />
          <Action label="Offsite backup bytes" status="backed" source="artifact-upload-broker.md" />
          <Action label="Restore backup" status="future" source="device-broker.md later list" />
        </div>
      </InternalNoteSection>
    </Screen>
  );
}

function BackupMetric({ label, value, detail, tone = "neutral" }) {
  const toneClasses =
    tone === "success"
      ? "border-emerald-300 bg-emerald-50/60 text-emerald-900"
      : tone === "warning"
        ? "border-amber-300 bg-amber-50/70 text-amber-950"
        : "border-slate-300 bg-white text-slate-950";

  return (
    <div className={`rounded-md border p-4 ${toneClasses}`}>
      <div className="text-xs font-semibold uppercase tracking-normal opacity-75">{label}</div>
      <div className="mt-2 text-2xl font-semibold">{value}</div>
      <div className="mt-1 text-sm opacity-75">{detail}</div>
    </div>
  );
}

function BackupFact({ label, value, warning = false }) {
  return (
    <div className={`rounded-md border px-3 py-2 ${warning ? "border-amber-200 bg-amber-50 text-amber-950" : "border-slate-200 bg-slate-50 text-slate-900"}`}>
      <div className="text-xs font-semibold uppercase tracking-normal text-slate-500">{label}</div>
      <div className="mt-1 text-sm font-medium">{value}</div>
    </div>
  );
}

function BackupFleetRow({ row, selected, onOpen }) {
  const { backup, site, device, customer } = row;
  const needsAttention = backup.status !== "succeeded";

  return (
    <article className={`px-5 py-5 ${selected ? "bg-sky-50/60" : "hover:bg-slate-50"}`}>
      <div className="grid gap-5 lg:grid-cols-[minmax(230px,1.6fr)_minmax(150px,0.9fr)_minmax(150px,0.9fr)_minmax(150px,0.9fr)_minmax(120px,auto)] lg:items-start">
        <div className="flex min-w-0 items-start gap-3">
          <HomeIcon category={site?.site_category} />
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <div className="font-semibold text-slate-950">{site?.name || backup.site_id}</div>
              {selected && <span className="rounded-md border border-sky-300 bg-white px-2 py-0.5 text-xs font-medium text-sky-700">Selected</span>}
              {needsAttention && <span className="rounded-md border border-amber-300 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-900">Retry needed</span>}
            </div>
            <div className="text-sm text-slate-500">{customer?.display_name || "No customer"} · {site?.location || "No location"}</div>
          </div>
        </div>

        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Backup</div>
          <BackupSummary backup={backup} />
        </div>

        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Offsite copy</div>
          <ArtifactSummary status={backup.artifact_status} size={backup.size_mb} />
        </div>

        <div>
          <div className="mb-1 text-xs font-semibold uppercase tracking-normal text-slate-400 lg:hidden">Connection</div>
          {device ? <ConnectionSummary device={device} /> : <EmptySummary title="Unknown" detail="No device record" />}
        </div>

        <div className="flex flex-wrap gap-2 lg:justify-end">
          {needsAttention && (
            <button type="button" className="rounded-md bg-slate-900 px-3 py-2 text-xs font-medium text-white hover:bg-slate-800">
              Trigger backup
            </button>
          )}
          <button
            type="button"
            onClick={onOpen}
            className="rounded-md border border-slate-300 px-3 py-2 text-xs font-medium text-slate-800 hover:bg-white"
          >
            Open
          </button>
        </div>
      </div>
    </article>
  );
}

function Updates() {
  return (
    <Screen title="Updates" subtitle="Review Home Assistant and HomeSignal app versions across managed sites.">
      <section className="rounded-md border border-slate-300 bg-white">
        <div className="grid grid-cols-[minmax(230px,2fr)_minmax(160px,1fr)_minmax(160px,1fr)_minmax(130px,auto)] gap-6 border-b border-slate-200 px-5 py-3 text-xs font-semibold uppercase tracking-normal text-slate-500">
          <div>Home Assistant</div>
          <div>Home Assistant OS</div>
          <div>HomeSignal app</div>
          <div>Action</div>
        </div>

        <div className="divide-y divide-slate-200">
          {devices.map((device) => {
            const site = sites.find((item) => item.id === device.site_id);
            const customer = customers.find((item) => item.id === site?.customer_record_id);
            return (
              <div key={device.id} className="grid grid-cols-[minmax(230px,2fr)_minmax(160px,1fr)_minmax(160px,1fr)_minmax(130px,auto)] gap-6 px-5 py-5">
                <div className="flex items-center gap-2">
                  <HomeIcon category={site?.site_category} />
                  <div>
                    <div className="font-semibold">{site?.name || "Unassigned Home Assistant"}</div>
                    <div className="text-sm text-slate-500">{customer?.display_name || "No customer"} · {site?.location || "No location"}</div>
                  </div>
                </div>

                <UpdateCell current={device.home_assistant_version} latest={device.latest_home_assistant_version} />
                <UpdateCell current={device.ha_app_version} latest={device.latest_ha_app_version} />

                <div className="flex items-start justify-end">
                  {device.home_assistant_version !== device.latest_home_assistant_version ||
                  device.ha_app_version !== device.latest_ha_app_version ? (
                    <TextButton>Review</TextButton>
                  ) : (
                    <span className="text-sm text-slate-500">Current</span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </section>

      <TwoColumn>
        <InternalNoteSection title="Update policy">
          <Field label="Home Assistant updates" value="Shown for visibility; applied by local Home Assistant controls" status="partial" source="Supervisor/local update boundary" />
          <Field label="HomeSignal app updates" value="Published through app repository" status="backed" source="release_channels + release_artifacts" />
          <Field label="Unsupported versions" value="Visible to user before enforcement" status="partial" source="migration-strategy.md" />
        </InternalNoteSection>

        <InternalNoteSection title="Internal release state">
          <Field label="Stable app release" value={edgeState.desired.update.desired_version} status="backed" source="homesignal_edge.update.desired_version" />
          <Field label="Release channel" value={edgeState.desired.update.channel} status="backed" source="homesignal_edge.update.channel" />
          <Field label="Rollout ID" value={edgeState.desired.update.rollout_id} status="backed" source="homesignal_edge.update.rollout_id" />
          <Field label="Binary install over IoT" value="Not allowed" status="missing" source="update-architecture.md" />
        </InternalNoteSection>
      </TwoColumn>
    </Screen>
  );
}

function Activity() {
  const groups = ["Today", "Yesterday", "Earlier"];

  return (
    <Screen title="Activity" subtitle="Full operational activity across managed Home Assistant sites.">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-md border border-slate-300 bg-white p-4">
        <div>
          <div className="text-sm font-medium text-slate-900">Operational timeline</div>
          <div className="text-xs text-slate-500">Filter by service area as the event stream grows.</div>
        </div>
        <div className="flex flex-wrap gap-2">
          {["All", "Alert", "Backup", "Device", "Enrollment", "Update"].map((item) => (
            <button
              key={item}
              type="button"
              className={`rounded-md border px-3 py-1.5 text-sm ${
                item === "All"
                  ? "border-slate-950 bg-slate-950 text-white"
                  : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
              }`}
            >
              {item}
            </button>
          ))}
        </div>
      </div>

      <div className="space-y-6">
        {groups.map((group) => {
          const events = activityEvents.filter((event) => event.group === group);
          if (events.length === 0) return null;

          return (
            <section key={group} className="grid gap-3 lg:grid-cols-[96px_minmax(0,1fr)]">
              <div className="pt-1 text-xs font-semibold uppercase tracking-normal text-slate-500">
                {group}
              </div>
              <div className="rounded-md border border-slate-300 bg-white">
                <ActivityTimeline events={events} />
              </div>
            </section>
          );
        })}
      </div>
    </Screen>
  );
}

function ActivityTimeline({ events, compact = false }) {
  return (
    <div className="divide-y divide-slate-200">
      {events.map((event, index) => {
        const tone = activityTone(event.category);
        const timeLabel = compact ? event.time : activityTimeLabel(event);

        return (
          <div
            key={`${event.action}-${event.subject}-${event.time}`}
            className={`grid items-start gap-3 px-4 text-sm ${
              compact ? "grid-cols-[18px_minmax(0,1fr)_auto] py-3" : "grid-cols-[88px_20px_minmax(0,1fr)_auto] py-4"
            }`}
          >
            {!compact && (
              <div className="pt-0.5 text-xs font-medium text-slate-500">
                <WiringValue id="25">
                  {timeLabel}
                </WiringValue>
              </div>
            )}

            <div className="relative flex justify-center self-stretch">
              {!compact && index < events.length - 1 && (
                <span className="absolute left-1/2 top-4 -bottom-6 w-px -translate-x-1/2 bg-slate-200" />
              )}
              <span className={`relative z-10 mt-1.5 h-2.5 w-2.5 rounded-full ${tone.dot}`} />
            </div>

            <div className="min-w-0">
              <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <WiringValue id="26" className="font-medium text-slate-950">
                  {event.action}
                </WiringValue>
                <WiringValue id="27" className="text-slate-500">
                  {event.subject}
                </WiringValue>
              </div>
              {!compact && (
                <p className="mt-1 max-w-2xl text-sm text-slate-600">
                  <WiringValue id="28">
                    {event.detail}
                  </WiringValue>
                </p>
              )}
              {compact && (
                <div className="mt-0.5 text-xs text-slate-500">
                  <WiringValue id="25">
                    {timeLabel}
                  </WiringValue>
                </div>
              )}
            </div>

            <div className={`whitespace-nowrap pt-0.5 text-xs font-semibold ${tone.text}`}>
              <WiringValue id="29">
                {event.category}
              </WiringValue>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function activityTimeLabel(event) {
  if (event.group === "Today") {
    return event.time.replace("Today, ", "");
  }

  return event.time;
}

function activityTone(category) {
  const tones = {
    Alert: { dot: "bg-rose-500", text: "text-rose-700" },
    Backup: { dot: "bg-sky-500", text: "text-sky-700" },
    Device: { dot: "bg-slate-500", text: "text-slate-700" },
    Enrollment: { dot: "bg-emerald-500", text: "text-emerald-700" },
    Update: { dot: "bg-amber-500", text: "text-amber-700" },
  };

  return tones[category] || { dot: "bg-slate-400", text: "text-slate-700" };
}

function Diagnostics() {
  return (
    <Screen title="Diagnostics" subtitle="Bounded support/debug capture, not arbitrary host access.">
      <TwoColumn>
        <InternalNoteSection title="Diagnostic capabilities">
          <Action label="Collect app status" status="backed" source="local-cloud-trust-boundaries.md" />
          <Action label="Collect connectivity check" status="backed" source="local-cloud-trust-boundaries.md" />
          <Action label="Collect recent error excerpt" status="backed" source="5 KB bounded excerpt" />
          <Action label="Request debug bundle" status="conditional" source="Diagnostics/Debug flow must explicitly enable artifact upload" />
          <Action label="Collect raw HA config snapshot" status="missing" source="explicitly not v0 diagnostics" />
        </InternalNoteSection>

        <InternalNoteSection title="Artifacts">
          <Field label="Diagnostic bundle metadata" value="Defined as table family" status="conditional" source="diagnostic_bundles" />
          <Field label="Debug bundle upload" value="Approved flow only" status="conditional" source="artifact-upload-broker.md" />
          <Field label="Unsolicited log upload" value="Not allowed" status="missing" source="artifact-upload-broker.md" />
        </InternalNoteSection>
      </TwoColumn>
    </Screen>
  );
}

function Alerts({ setPage, setSelectedSiteId, setSelectedDeviceId }) {
  const [alertTab, setAlertTab] = useState("current");
  const [showRecipientForm, setShowRecipientForm] = useState(false);
  const [recipientDraft, setRecipientDraft] = useState({
    name: "",
    email: "",
    role: "Recipient",
    disconnected: true,
    backup: true,
    updates: false,
    storage: false,
    scope: "All managed sites",
  });
  const [addedRecipients, setAddedRecipients] = useState([]);
  const alertRecipients = [
    {
      name: "Jamie Smith",
      email: "jamie@example.com",
      role: "Owner",
      disconnected: true,
      backup: true,
      updates: true,
      storage: false,
      scope: "All managed sites",
    },
    {
      name: "Alex Lee",
      email: "alex@example.com",
      role: "Technician",
      disconnected: true,
      backup: true,
      updates: false,
      storage: false,
      scope: "Smith Residence, Lee Residence",
    },
    {
      name: "Morgan Patel",
      email: "morgan@example.com",
      role: "Read-only",
      disconnected: true,
      backup: false,
      updates: false,
      storage: false,
      scope: "Lee Residence",
    },
    ...addedRecipients,
  ];

  return (
    <Screen title="Alerts" subtitle="Current site alerts and email notification settings.">
      <div className="inline-flex rounded-md border border-slate-300 bg-white p-1">
        {[
          ["current", "Current alerts"],
          ["recipients", "Email recipients"],
        ].map(([key, label]) => (
          <button
            key={key}
            type="button"
            onClick={() => setAlertTab(key)}
            className={`rounded px-3 py-1.5 text-sm font-medium ${
              alertTab === key
                ? "bg-slate-900 text-white"
                : "text-slate-600 hover:bg-slate-100"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {alertTab === "current" && (
        <section className="rounded-md border border-slate-300 bg-white p-5">
          <div className="mb-4">
            <h2 className="text-lg font-semibold">Current alerts</h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-slate-600">
              Active conditions across managed Home Assistant instances. These are
              operational items to review, not a live Home Assistant event feed.
            </p>
          </div>

          <Table
            columns={["Condition", "Site", "Latest signal", "Action"]}
            rows={currentAlerts.map((item) => [
              <span className="font-medium text-slate-950">{item.condition}</span>,
              item.site,
              item.detail,
              <TextButton
                onClick={() => {
                  setSelectedSiteId(item.siteId);
                  setSelectedDeviceId(item.deviceId);
                  setPage(item.page);
                }}
              >
                {item.actionLabel}
              </TextButton>,
            ])}
          />
        </section>
      )}

      {alertTab === "recipients" && (
        <section className="rounded-md border border-slate-300 bg-white p-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold">Email alerts</h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-slate-600">
              Send operational alerts to the integrator team when a Home Assistant instance disconnects,
              a backup fails, or the HomeSignal app needs attention.
            </p>
          </div>
          <button
            type="button"
            onClick={() => setShowRecipientForm((value) => !value)}
            className="rounded-md bg-[#03a9f4] px-4 py-2 text-sm font-medium text-white hover:bg-[#0288d1]"
          >
            {showRecipientForm ? "Cancel" : "Add recipient"}
          </button>
        </div>

        {showRecipientForm && (
          <div className="mt-5 rounded-md border border-[#b3e5fc] bg-[#e1f5fe] p-4">
            <div className="grid gap-3 lg:grid-cols-[1fr_1fr_180px]">
              <label className="text-sm font-medium text-slate-700">
                Name
                <input
                  value={recipientDraft.name}
                  onChange={(event) => setRecipientDraft({ ...recipientDraft, name: event.target.value })}
                  className="mt-1 w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm"
                  placeholder="Pat Morgan"
                />
              </label>
              <label className="text-sm font-medium text-slate-700">
                Email
                <input
                  value={recipientDraft.email}
                  onChange={(event) => setRecipientDraft({ ...recipientDraft, email: event.target.value })}
                  className="mt-1 w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm"
                  placeholder="pat@example.com"
                />
              </label>
              <label className="text-sm font-medium text-slate-700">
                Scope
                <select
                  value={recipientDraft.scope}
                  onChange={(event) => setRecipientDraft({ ...recipientDraft, scope: event.target.value })}
                  className="mt-1 w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm"
                >
                  <option>All managed sites</option>
                  <option>Smith Residence</option>
                  <option>Lee Residence</option>
                </select>
              </label>
            </div>

            <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <InlineCheck label="Disconnected" checked={recipientDraft.disconnected} onChange={(checked) => setRecipientDraft({ ...recipientDraft, disconnected: checked })} />
              <InlineCheck label="Backup failed" checked={recipientDraft.backup} onChange={(checked) => setRecipientDraft({ ...recipientDraft, backup: checked })} />
              <InlineCheck label="Updates" checked={recipientDraft.updates} onChange={(checked) => setRecipientDraft({ ...recipientDraft, updates: checked })} />
              <InlineCheck label="Storage" checked={recipientDraft.storage} onChange={(checked) => setRecipientDraft({ ...recipientDraft, storage: checked })} />
            </div>

            <div className="mt-4">
              <button
                type="button"
                onClick={() => {
                  const email = recipientDraft.email.trim() || "new-recipient@example.com";
                  setAddedRecipients([
                    ...addedRecipients,
                    {
                      ...recipientDraft,
                      name: recipientDraft.name.trim() || "New recipient",
                      email,
                    },
                  ]);
                  setRecipientDraft({
                    name: "",
                    email: "",
                    role: "Recipient",
                    disconnected: true,
                    backup: true,
                    updates: false,
                    storage: false,
                    scope: "All managed sites",
                  });
                  setShowRecipientForm(false);
                }}
                className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700"
              >
                Save recipient
              </button>
            </div>
          </div>
        )}

        <div className="mt-5 overflow-hidden rounded-md border border-slate-200">
          <div className="hidden grid-cols-[minmax(230px,2fr)_1fr_1fr_1fr_1fr_minmax(160px,1.2fr)] gap-4 border-b border-slate-200 bg-slate-50 px-4 py-3 text-xs font-semibold uppercase tracking-normal text-slate-500 lg:grid">
            <div>Email address</div>
            <div>Disconnected</div>
            <div>Backup failed</div>
            <div>Updates</div>
            <div>Storage</div>
            <div>Scope</div>
          </div>

          <div className="divide-y divide-slate-200">
            {alertRecipients.map((recipient) => (
              <EmailAlertRow key={recipient.email} recipient={recipient} />
            ))}
          </div>
        </div>
      </section>
      )}
    </Screen>
  );
}

function EmailAlertRow({ recipient }) {
  return (
    <div className="grid gap-3 px-4 py-4 lg:grid-cols-[minmax(230px,2fr)_1fr_1fr_1fr_1fr_minmax(160px,1.2fr)] lg:items-center">
      <div>
        <div className="font-medium text-slate-950">{recipient.email}</div>
        <div className="mt-1 text-sm text-slate-500">{recipient.name} · {recipient.role}</div>
      </div>

      <AlertToggle label="Disconnected" enabled={recipient.disconnected} />
      <AlertToggle label="Backup failed" enabled={recipient.backup} />
      <AlertToggle label="Updates" enabled={recipient.updates} />
      <AlertToggle label="Storage" enabled={recipient.storage} />

      <div>
        <div className="text-sm font-medium text-slate-800">{recipient.scope}</div>
        <button type="button" className="mt-1 text-xs text-[#0277bd] hover:underline">Edit scope</button>
      </div>
    </div>
  );
}

function AlertToggle({ label, enabled, status = "backed" }) {
  return (
    <div className="flex items-center justify-between gap-3 lg:block">
      <div className="mb-1 flex items-center gap-1 text-sm text-slate-700 lg:hidden">
        {label}
        <Coverage status={status} />
      </div>
      <div className="hidden items-center gap-1 text-xs text-slate-500 lg:flex">
        <Coverage status={status} />
      </div>
      <span
        className={`relative inline-flex h-6 w-11 rounded-full ${enabled ? "bg-slate-900" : "bg-slate-300"}`}
        aria-label={`${label} ${enabled ? "enabled" : "disabled"}`}
      >
        <span className={`mt-1 h-4 w-4 rounded-full bg-white transition ${enabled ? "ml-6" : "ml-1"}`} />
      </span>
    </div>
  );
}

function InlineCheck({ label, checked, onChange }) {
  return (
    <label className="flex items-center gap-2 rounded-md border border-[#b3e5fc] bg-white px-3 py-2 text-sm text-slate-700">
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="h-4 w-4"
      />
      {label}
    </label>
  );
}

function InternalAudit() {
  return (
    <Screen title="Internal audit" subtitle="Sensitive authority history and platform-owned audit review.">
      <Section title="Audit events">
        <SimpleList
          items={auditEvents.map((event) => ({
            text: event,
            status: "backed",
            note: "audit_events",
          }))}
        />
      </Section>
    </Screen>
  );
}

function Audit() {
  return (
    <Screen title="Internal audit" subtitle="Sensitive authority history is separate from operational logs.">
      <Section title="Audit events">
        <SimpleList
          items={auditEvents.map((event) => ({
            text: event,
            status: "backed",
            note: "audit_events",
          }))}
        />
      </Section>
    </Screen>
  );
}

function Admin() {
  return (
    <Screen title="Internal admin" subtitle="Platform-owner surfaces; separated from the integrator console.">
      <TwoColumn>
        <InternalNoteSection title="Policy and budgets">
          <Field label="Publish policy catalog" value="Resolved per-device policy records" status="backed" source="device_desired_state + edge projection" />
          <Field label="Plan tier editor" value="Admin-defined, needs concrete UI spec" status="partial" source="publish policy discussion" />
          <Field label="Live event stream pricing" value="Future" status="future" source="not v0" />
        </InternalNoteSection>

        <InternalNoteSection title="Platform operations">
          <Field label="Platform health findings" value="Future internal" status="future" source="platform_health_findings" />
          <Field label="Runaway device messaging monitor" value="Future internal" status="future" source="platform-health-monitoring-service.md" />
          <Field label="Service credential rotation" value="Neon/Postgres day-zero concern" status="partial" source="secrets-and-config.md" />
        </InternalNoteSection>
      </TwoColumn>

      <TwoColumn>
        <InternalNoteSection title="Reported state / advanced device facts">
          <Field label="Device ID" value="dev_123" status="backed" source="devices.device_id" />
          <Field label="Thing name" value="dev_123" status="backed" source="AWS IoT Thing name" />
          <Field label="Credential status" value="active" status="backed" source="device_credentials" />
          <Field label="Publish policy repair" value="refresh_publish_policy" status="backed" source="commands.refresh_publish_policy" />
        </InternalNoteSection>

        <InternalNoteSection title="Raw projections">
          <Field label="Edge desired/report projection" value="Compact fields only" status="backed" source="device_edge_state_projection" />
          <Field label="Shadow full document history" value="Not stored by default" status="missing" source="edge-state-adapter.md" />
          <Field label="Raw HA config viewer" value="Future/internal only; not v0" status="future" source="diagnostics boundary" />
        </InternalNoteSection>
      </TwoColumn>
    </Screen>
  );
}

/*
  Real app mount boundary

  The HomeSignal Manager UI below is the candidate React surface for the
  Home Assistant app ingress UI. Keep it local-app scoped: Status,
  Pairing, Permissions, and Advanced. The current implementation still uses
  mock data/state from this file; the real mount should replace those reads with
  a narrow app API adapter documented in
  design-docs/ha-app-ui-mount-plan.md.
*/
function HaApp({ haAppScreen, setHaAppScreen }) {
  const [pairingStage, setPairingStage] = useState("preflight");
  const [pairingCodeState, setPairingCodeState] = useState("success");
  const [permissionPolicy, setPermissionPolicy] = useState(initialHaAppPermissionPolicy);
  const [savedPermissionPolicy, setSavedPermissionPolicy] = useState(initialHaAppPermissionPolicy);
  const [permissionSavedAt, setPermissionSavedAt] = useState(null);
  const [autoPairStatus, setAutoPairStatus] = useState({ state: "idle", value: null });
  const [mockHaAppPairingState, setMockHaAppPairingState] = useState(() => readMockHaAppPairingState());
  const [mockHaAppBootstrapState, setMockHaAppBootstrapState] = useState(() => readMockHaAppBootstrapState());
  const [bootstrapViewState, setBootstrapViewState] = useState("checking");
  const [claimInviteCode, setClaimInviteCode] = useState(() => readAutoPairingContext()?.pairing_id || haAppState.claim_invite_code);
  const [claimStep, setClaimStep] = useState("guid");
  const [claimLookup, setClaimLookup] = useState({ state: "idle", result: null });
  const autoPairIframeRef = useRef(null);
  const autoPairRequestIdRef = useRef(null);
  const autoPairCompletedRef = useRef(false);
  const bootstrapHardStoppedRef = useRef(false);
  const initialStatusState = Object.prototype.hasOwnProperty.call(haAppStatusStates, haAppScreen) ? haAppScreen : "onboarding";
  const [haAppConnectionState, setHaAppConnectionState] = useState(initialStatusState);
  const isPairingScreen = haAppScreen === "pairing";
  const isPermissionsScreen = haAppScreen === "permissions";
  const isAdvancedScreen = haAppScreen === "advanced";
  const statusState = Object.prototype.hasOwnProperty.call(haAppStatusStates, haAppScreen) ? haAppScreen : haAppConnectionState;
  const headerStatusState = pairingStage === "connected" ? "healthy" : statusState;
  const connectionStatus = haAppShellStatus[headerStatusState];
  const activeHaAppPage = isPairingScreen ? "pairing" : isPermissionsScreen ? "permissions" : isAdvancedScreen ? "advanced" : "status";
  const displayVersion = getHaAppDisplayVersion(headerStatusState);
  const navigateHaAppPage = (key) => setHaAppScreen(key === "status" ? headerStatusState : key);
  const shouldRunBootstrap = !mockHaAppPairingState.has_ever_paired && !mockHaAppBootstrapState.has_run_bootstrap;
  const shouldLoadAutoPairingBridge = shouldRunBootstrap;
  const shouldShowBootstrapScreen = shouldRunBootstrap && bootstrapViewState === "checking";
  const lookupClaimRequest = (nextCode = claimInviteCode) => {
    setPairingStage("code");
    setClaimStep("approve");
    setPairingCodeState("loading");
    setClaimLookup({ state: "loading", result: null });

    window.setTimeout(() => {
      const result = mockClaimRequestForGuid(nextCode);
      setClaimLookup({ state: result.ok ? "ready" : "error", result });
      setPairingCodeState(result.ok ? "success" : "rate_limited");
    }, 450);
  };

  const setMockPairingStep = (key) => {
    if (key === "preflight") {
      setPairingStage("preflight");
      setPairingCodeState("success");
      setClaimStep("guid");
      setClaimLookup({ state: "idle", result: null });
      setHaAppConnectionState("onboarding");
      return;
    }

    if (key === "code") {
      const result = mockClaimRequestForGuid(claimInviteCode);
      setPairingStage("code");
      setPairingCodeState(result.ok ? "success" : "rate_limited");
      setClaimStep("approve");
      setClaimLookup({ state: result.ok ? "ready" : "error", result });
      setHaAppConnectionState("onboarding");
      return;
    }

    if (key === "connected") {
      setPairingStage("connected");
      setClaimStep("success");
      setHaAppConnectionState("healthy");
    }
  };

  const setMockInviteCondition = (key) => {
    setPairingStage("code");
    setClaimStep("approve");

    if (key === "loading") {
      setPairingCodeState("loading");
      setClaimLookup({ state: "loading", result: null });
      return;
    }

    if (key === "success") {
      const result = mockClaimRequestForGuid(claimInviteCode || "mock-good");
      setPairingCodeState("success");
      setClaimLookup({ state: "ready", result: result.ok ? result : mockClaimRequestForGuid("mock-good") });
      return;
    }

    if (key === "rate_limited") {
      const result = mockClaimRequestForGuid("mock-network");
      setPairingCodeState("rate_limited");
      setClaimLookup({ state: "error", result });
    }
  };

  useEffect(() => {
    if (Object.prototype.hasOwnProperty.call(haAppStatusStates, haAppScreen)) {
      setHaAppConnectionState(haAppScreen);
    }
  }, [haAppScreen]);

  useEffect(() => {
    if (!shouldRunBootstrap) {
      return undefined;
    }

    bootstrapHardStoppedRef.current = false;

    const visibleTimer = window.setTimeout(() => {
      setBootstrapViewState((current) => (current === "checking" ? "complete" : current));
    }, haAppBootstrapLoadingMaxMs);

    const hardStopTimer = window.setTimeout(() => {
      bootstrapHardStoppedRef.current = true;

      if (!autoPairCompletedRef.current) {
        setAutoPairStatus({ state: "idle", value: null });
        const nextBootstrap = writeMockHaAppBootstrapState({
          has_run_bootstrap: true,
          last_checked_at: new Date().toISOString(),
        });
        setMockHaAppBootstrapState(nextBootstrap);
        setBootstrapViewState("complete");
      }
    }, haAppBootstrapHardStopMs);

    return () => {
      window.clearTimeout(visibleTimer);
      window.clearTimeout(hardStopTimer);
    };
  }, [shouldRunBootstrap]);

  useEffect(() => {
    const onMessage = (event) => {
      const frameWindow = autoPairIframeRef.current?.contentWindow;
      if (!frameWindow || event.source !== frameWindow || typeof event.data !== "object" || event.data === null) return;
      if (bootstrapHardStoppedRef.current) return;

      if (event.data.type === "homesignal.auto_pairing.value" && event.data.request_id === autoPairRequestIdRef.current) {
        if (!event.data.ok) {
          if (!autoPairCompletedRef.current) {
            setAutoPairStatus({ state: "idle", value: null });
          }
          return;
        }

        autoPairCompletedRef.current = true;
        setMockHaAppBootstrapState(writeMockHaAppBootstrapState({
          has_run_bootstrap: true,
          last_checked_at: new Date().toISOString(),
        }));
        setBootstrapViewState("found");
        setAutoPairStatus({ state: "found", value: event.data.value });
        setClaimInviteCode(event.data.value.pairing_id);
        setClaimStep("guid");
        setClaimLookup({ state: "idle", result: null });
        setPairingStage("preflight");
        setPairingCodeState("success");
        setHaAppConnectionState("onboarding");
        setHaAppScreen("pairing");
      }
    };

    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, [setHaAppScreen]);

  const requestAutoPairingContext = () => {
    if (!shouldLoadAutoPairingBridge) return;
    if (autoPairCompletedRef.current) return;
    if (bootstrapHardStoppedRef.current) return;

    const frameWindow = autoPairIframeRef.current?.contentWindow;
    if (!frameWindow) return;

    const requestId = `get_${Date.now()}`;
    autoPairRequestIdRef.current = requestId;
    setAutoPairStatus({ state: "checking", value: null });
    frameWindow.postMessage(
      { type: "homesignal.auto_pairing.get", request_id: requestId },
      window.location.origin
    );
  };

  const requestAutoPairingContextAfterBridgeLoad = () => {
    window.setTimeout(requestAutoPairingContext, 100);
    window.setTimeout(requestAutoPairingContext, 700);
  };

  if (shouldShowBootstrapScreen) {
    return (
      <div className="mx-auto max-w-[960px] font-['Roboto',Arial,sans-serif]">
        <iframe
          ref={autoPairIframeRef}
          title="HomeSignal auto-pairing bridge"
          src={autoPairingBridgePath}
          onLoad={requestAutoPairingContextAfterBridgeLoad}
          className="hidden"
        />
        <HaMockControls
          mockHaAppPairingState={mockHaAppPairingState}
          mockHaAppBootstrapState={mockHaAppBootstrapState}
          resetLocalMockState={() => {
            clearMockHaAppLocalState();
            autoPairCompletedRef.current = false;
            setPairingStage("preflight");
            setPairingCodeState("success");
            setClaimInviteCode(haAppState.claim_invite_code);
            setClaimStep("guid");
            setClaimLookup({ state: "idle", result: null });
            setMockHaAppPairingState(readMockHaAppPairingState());
            setMockHaAppBootstrapState(readMockHaAppBootstrapState());
            setBootstrapViewState("checking");
            setAutoPairStatus({ state: "idle", value: null });
            setHaAppConnectionState("onboarding");
            setHaAppScreen("onboarding");
          }}
        />
        <HaBootstrapRunOnceView state={bootstrapViewState} />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[960px] font-['Roboto',Arial,sans-serif]">
      {shouldLoadAutoPairingBridge && (
        <iframe
          ref={autoPairIframeRef}
          title="HomeSignal auto-pairing bridge"
          src={autoPairingBridgePath}
          onLoad={requestAutoPairingContextAfterBridgeLoad}
          className="hidden"
        />
      )}
      <HaMockControls
        mockHaAppPairingState={mockHaAppPairingState}
        mockHaAppBootstrapState={mockHaAppBootstrapState}
        resetLocalMockState={() => {
          clearMockHaAppLocalState();
          autoPairCompletedRef.current = false;
          setPairingStage("preflight");
          setPairingCodeState("success");
          setClaimInviteCode(haAppState.claim_invite_code);
          setClaimStep("guid");
          setClaimLookup({ state: "idle", result: null });
          setMockHaAppPairingState(readMockHaAppPairingState());
          setMockHaAppBootstrapState(readMockHaAppBootstrapState());
          setBootstrapViewState("checking");
          setAutoPairStatus({ state: "idle", value: null });
          setHaAppConnectionState("onboarding");
          setHaAppScreen("onboarding");
        }}
      >
        <div className="flex flex-wrap gap-2">
          {[
            ["status", "Status"],
            ["pairing", "Pairing"],
            ["permissions", "Permissions"],
            ["advanced", "Advanced"],
          ].map(([key, label]) => (
            <button
              key={key}
              type="button"
              onClick={() => navigateHaAppPage(key)}
              className={`rounded-md border px-3 py-1.5 text-sm ${
                (key === "pairing" ? isPairingScreen : key === "permissions" ? isPermissionsScreen : key === "advanced" ? isAdvancedScreen : !isPairingScreen && !isPermissionsScreen && !isAdvancedScreen)
                  ? "border-amber-400 bg-amber-50 text-slate-900"
                  : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        {!isPairingScreen && !isPermissionsScreen && !isAdvancedScreen && (
          <div className="mt-3 border-t border-slate-200 pt-3">
            <div className="mb-2 text-xs font-semibold uppercase tracking-normal text-slate-500">Status state</div>
            <div className="flex flex-wrap gap-2">
              {Object.entries(haAppStatusStates).map(([key, item]) => (
                <button
                  key={key}
                  type="button"
                  onClick={() => setHaAppScreen(key)}
                  className={`rounded-md border px-3 py-1.5 text-sm ${
                    statusState === key
                      ? "border-[#03a9f4] bg-sky-50 text-slate-950"
                      : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
                  }`}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {isPairingScreen && (
          <div className="mt-3 border-t border-slate-200 pt-3">
            <div className="mb-2 text-xs font-semibold uppercase tracking-normal text-slate-500">Pairing state</div>
            <div className="flex flex-wrap gap-2">
              {[
                ["preflight", "Setup"],
                    ["code", "Approve"],
                ["connected", "Paired"],
              ].map(([key, label]) => (
                <button
                  key={key}
                  type="button"
                  onClick={() => setMockPairingStep(key)}
                  className={`rounded-md border px-3 py-1.5 text-sm ${
                    pairingStage === key
                      ? "border-[#03a9f4] bg-sky-50 text-slate-950"
                      : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>
            {pairingStage === "code" && (
              <div className="mt-3 border-t border-slate-200 pt-3">
                <div className="mb-2 text-xs font-semibold uppercase tracking-normal text-slate-500">Review condition</div>
                <div className="flex flex-wrap gap-2">
                  {[
                    ["loading", "Loading"],
                    ["success", "Loaded"],
                    ["rate_limited", "Rate limited"],
                  ].map(([key, label]) => (
                    <button
                      key={key}
                      type="button"
                      onClick={() => setMockInviteCondition(key)}
                      className={`rounded-md border px-3 py-1.5 text-sm ${
                        pairingCodeState === key
                          ? "border-[#03a9f4] bg-sky-50 text-slate-950"
                          : "border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
                      }`}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </HaMockControls>

      <div className="bg-white text-[#212121]">
        <div className="px-5 py-4 sm:px-8">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
              <h1 className="text-2xl font-normal tracking-normal text-[#212121] sm:text-3xl">HomeSignal Manager</h1>
              <span className="text-sm leading-5 text-[#616161]">Version {displayVersion}</span>
            </div>
            <div className="flex items-center gap-2 rounded-full border border-[#e0e0e0] px-3 py-1.5">
              <HaStateDot tone={connectionStatus.tone} size="sm" />
              <span className="text-sm font-medium text-[#616161]">{connectionStatus.label}</span>
            </div>
          </div>
        </div>
        <HaAppNav
          activePage={activeHaAppPage}
          onNavigate={navigateHaAppPage}
          variant="top"
        />
        <HaAppShell>
          {!isPairingScreen && !isPermissionsScreen && !isAdvancedScreen && (
            <HaStatusPage
              statusState={statusState}
              permissionPolicy={savedPermissionPolicy}
              autoPairStatus={autoPairStatus}
              setHaAppScreen={setHaAppScreen}
            />
          )}
          {isPairingScreen && (
            <HaPairing
              pairingStage={pairingStage}
              pairingCodeState={pairingCodeState}
              permissionPolicy={permissionPolicy}
              claimInviteCode={claimInviteCode}
              setClaimInviteCode={setClaimInviteCode}
              claimStep={claimStep}
              setClaimStep={setClaimStep}
              claimLookup={claimLookup}
              setClaimLookup={setClaimLookup}
              lookupClaimRequest={lookupClaimRequest}
              setPermissionPolicy={setPermissionPolicy}
              setPairingStage={setPairingStage}
              setPairingCodeState={setPairingCodeState}
              setHaAppScreen={setHaAppScreen}
              onPairingCommitted={(pairingId) => {
                setMockHaAppPairingState(writeMockHaAppPairingState({
                  has_ever_paired: true,
                  last_pairing_id: pairingId,
                  paired_at: new Date().toISOString(),
                }));
                setSavedPermissionPolicy(permissionPolicy);
                setPermissionSavedAt("Just now");
                removeAutoPairingContext();
                setAutoPairStatus({ state: "paired", value: null });
                setHaAppConnectionState("healthy");
              }}
            />
          )}
          {isPermissionsScreen && (
            <HaPermissionsPage
              permissionPolicy={permissionPolicy}
              savedPermissionPolicy={savedPermissionPolicy}
              permissionSavedAt={permissionSavedAt}
              setPermissionPolicy={setPermissionPolicy}
              onSave={() => {
                setSavedPermissionPolicy(permissionPolicy);
                setPermissionSavedAt("Just now");
              }}
            />
          )}
          {isAdvancedScreen && <HaAdvancedPage setHaAppScreen={setHaAppScreen} />}
        </HaAppShell>
        <HaAppNav
          activePage={activeHaAppPage}
          onNavigate={navigateHaAppPage}
          variant="bottom"
        />
      </div>
    </div>
  );
}

function HaMockControls({ mockHaAppPairingState, mockHaAppBootstrapState, resetLocalMockState, children }) {
  return (
    <div className="mb-4 rounded-md border border-slate-300 bg-white px-4 py-3 shadow-sm">
      <div className="mb-2 text-xs font-semibold uppercase tracking-normal text-slate-500">Mock page</div>
      <div className="mb-3 flex flex-wrap items-center gap-2 text-xs text-slate-600">
        <span>Mock app local storage:</span>
        <span className="rounded bg-slate-100 px-2 py-1 font-mono">
          has_ever_paired={String(mockHaAppPairingState.has_ever_paired)}
        </span>
        <span className="rounded bg-slate-100 px-2 py-1 font-mono">
          has_run_bootstrap={String(mockHaAppBootstrapState.has_run_bootstrap)}
        </span>
        <button
          type="button"
          onClick={resetLocalMockState}
          className="rounded border border-slate-300 px-2 py-1 hover:bg-slate-50"
        >
          Flush mock app state
        </button>
      </div>
      {children}
    </div>
  );
}

function HaBootstrapRunOnceView({ state }) {
  const isFound = state === "found";

  return (
    <div className="overflow-hidden rounded-xl border border-[#e0e0e0] bg-white text-[#212121]">
      <div className="mx-auto max-w-xl px-6 py-16 text-center">
        <div className={`mx-auto flex h-12 w-12 items-center justify-center rounded-full ${
          isFound ? "bg-emerald-700 text-white" : "bg-[#e1f5fe] text-[#039dcc]"
        }`}>
          {isFound ? (
            <svg aria-hidden="true" viewBox="0 0 24 24" className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
              <path d="M20 6 9 17l-5-5" />
            </svg>
          ) : (
            <span className="h-5 w-5 animate-spin rounded-full border-2 border-[#b3e5fc] border-t-[#039dcc]" />
          )}
        </div>
        <h1 className="mt-5 text-3xl font-normal tracking-normal text-[#212121]">HomeSignal Manager</h1>
        <div className="mt-4 text-base font-medium text-[#212121]">
          {isFound ? "Pairing code found" : "Loading..."}
        </div>
      </div>
    </div>
  );
}

const statusPageConfig = {
  onboarding: {
    updateAgeDays: 2,
    healthItems: haAppHealthSnapshot,
    lastReported: "Not sent yet",
    showManagedBy: false,
    attention: {
      tone: "info",
      title: "Ready to pair with HomeSignal",
      body: "This app has not been paired before. Pair it with the HomeSignal site you manage to create the durable site association.",
    },
    actions: ["pair", "create_account"],
  },
  healthy: {
    healthItems: haAppHealthySnapshot,
    lastReported: "May 14, 2026, 11:59 AM",
    showManagedBy: true,
    showOperationalSections: true,
    updateRows: [
      ["App version", "0.1.4", "Ready"],
      ["Auto-update", "On", "Ready"],
      ["Start on boot", "Auto", "Ready"],
      ["Watchdog", "On", "Ready"],
    ],
  },
  disconnected: {
    healthItems: haAppDisconnectedSnapshot,
    lastReported: "May 14, 2026, 11:12 AM",
    showManagedBy: true,
    attention: {
      tone: "warning",
      title: "Connection needs attention",
      body: "This app was previously paired with HomeSignal, but it is not currently reporting. Last connected May 14, 2026, 11:12 AM.",
    },
    actions: ["retry", "portal"],
  },
  outdated: {
    updateAgeDays: 6,
    healthItems: haAppOutdatedSnapshot,
    lastReported: "May 14, 2026, 11:59 AM",
    showManagedBy: true,
    showOperationalSections: true,
    updateRows: [
      ["App version", "0.1.3", "Needs attention"],
      ["Auto-update", "Check in Home Assistant", "Needs attention"],
      ["Start on boot", "Auto", "Ready"],
      ["Watchdog", "On", "Ready"],
    ],
  },
};

function getHaAppDisplayVersion(statusState) {
  return statusState === "healthy" || statusState === "disconnected"
    ? haAppState.latest_ha_app_version
    : haAppState.ha_app_version;
}

function HaStatusPage({ statusState, permissionPolicy, autoPairStatus, setHaAppScreen }) {
  const config = statusPageConfig[statusState] ?? statusPageConfig.onboarding;

  return (
    <div className="p-4">
      <div className="max-w-2xl">
        <HaAutoPairStatus status={autoPairStatus} />
        {config.updateAgeDays && <HaUpdateNotice updateAgeDays={config.updateAgeDays} />}
        {config.attention && (
          <HaStatusAttention
            attention={config.attention}
            actions={config.actions}
            setHaAppScreen={setHaAppScreen}
          />
        )}
        {config.showManagedBy && <HaManagedBySection />}

        <div className="mb-5">
          <HaHealthStatus items={config.healthItems} lastReported={config.lastReported} />
        </div>

        {config.showOperationalSections && <HaStatusOperationalSections permissionPolicy={permissionPolicy} />}
      </div>
    </div>
  );
}

function HaAutoPairStatus({ status }) {
  if (!status || status.state === "idle") return null;

  const isPaired = status.state === "paired";

  return (
    <div className={`mb-5 rounded-lg border px-4 py-3 text-sm ${
      isPaired ? "border-emerald-300 bg-emerald-50 text-emerald-950" : "border-sky-200 bg-sky-50 text-slate-800"
    }`}>
      <div className="font-semibold">
        {isPaired ? "Browser pairing code received" : "Checking browser pairing code"}
      </div>
      <p className={`mt-1 leading-6 ${isPaired ? "text-emerald-900" : "text-slate-600"}`}>
        {isPaired
          ? `Auto-pairing completed with ${status.value?.pairing_id}. The temporary browser context was removed.`
          : "The app is checking the HomeSignal portal bridge for a short-lived pairing code."}
      </p>
    </div>
  );
}

function HaStatusAttention({ attention, actions = [], setHaAppScreen }) {
  const toneClass = attention.tone === "warning" ? "border-amber-300 bg-amber-50 text-amber-950" : "border-sky-200 bg-sky-50 text-slate-800";
  const bodyClass = attention.tone === "warning" ? "text-amber-900" : "text-slate-600";

  return (
    <div className={`mb-5 rounded-lg border px-4 py-3 ${toneClass}`}>
      <div className="text-sm font-semibold">{attention.title}</div>
      <p className={`mt-1 max-w-2xl text-sm leading-6 ${bodyClass}`}>{attention.body}</p>
      {actions.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-2">
          {actions.includes("pair") && (
            <HaButton type="button" onClick={() => setHaAppScreen("pairing")} size="lg">
              Pair with HomeSignal
            </HaButton>
          )}
          {actions.includes("retry") && <HaButton type="button">Retry pairing</HaButton>}
          {actions.includes("create_account") && <HomeSignalAccountActionLink />}
          {actions.includes("portal") && <HomeSignalPortalActionLink />}
        </div>
      )}
    </div>
  );
}

function HaManagedBySection() {
  return <HaManagedByCard className="mb-5" />;
}

function HaManagedByCard({
  className = "",
  title = "Managed by",
  organization = haAppState.organization,
  site = haAppState.site,
  deviceId = haAppState.device_id,
  requesterName,
  requesterEmail,
  pairingCode,
}) {
  return (
    <div className={className}>
      <div className="rounded-lg border border-[#e0e0e0] bg-[#f7f8fa] px-5 py-5">
        <div className="text-sm font-semibold text-[#212121]">{title}</div>
        <div className="mt-4">
          <div>
            <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">Organization</div>
            <div className="mt-1 text-base font-medium text-[#212121]">{organization}</div>
            {requesterName && (
              <div className="mt-1 text-sm text-[#616161]">
                Requested by <span className="font-medium text-[#212121]">{requesterName}</span>
                {requesterEmail && <span className="break-all"> ({requesterEmail})</span>}
              </div>
            )}
          </div>
        </div>
        <div className="mt-5 grid gap-4 border-t border-[#eeeeee] pt-4 sm:grid-cols-2">
          <div>
            <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">Site</div>
            <div className="mt-1 text-base font-medium text-[#212121]">{site}</div>
          </div>
          <div>
            <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">Device ID</div>
            <div className="mt-1 font-mono text-sm text-[#212121]">{deviceId}</div>
          </div>
        </div>
        {pairingCode && (
          <div className="mt-4 border-t border-[#e0e0e0] pt-3">
            <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">Pairing code</div>
            <div className="mt-1 break-all font-mono text-sm font-medium text-[#212121]">{pairingCode}</div>
          </div>
        )}
      </div>
    </div>
  );
}

function HaPairingRequestSummary({ claimDetails }) {
  return (
    <div className="px-1">
      <div className="text-sm font-semibold text-[#212121]">Request to manage this Home Assistant</div>
      <div className="mt-4">
        <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">From</div>
        <div className="mt-1 text-base font-medium tracking-normal text-[#212121]">{claimDetails.organization}</div>
        <div className="mt-1 text-sm text-[#616161]">
          Requested by <span className="font-medium text-[#212121]">{claimDetails.requested_by}</span>
          <span className="break-all"> ({claimDetails.requester_email})</span>
        </div>
      </div>
    </div>
  );
}

function HaApprovalPanel({ claimDetails, pairingCode }) {
  return (
    <div className="mt-5 border-t border-[#eeeeee] px-1 pt-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <div>
          <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">Site</div>
          <div className="mt-1 text-base font-medium text-[#212121]">{claimDetails.site}</div>
        </div>
        <div>
          <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">Pairing code</div>
          <div className="mt-1 break-all font-mono text-sm font-medium text-[#212121]">{pairingCode}</div>
        </div>
      </div>
    </div>
  );
}

function HaRemoteManagementApproval({ permissionPolicy, setPermissionPolicy }) {
  return (
    <div className="mt-5 rounded-lg border border-[#e0e0e0] bg-white px-5 py-5">
      <div className="text-sm font-semibold text-[#212121]">Set remote access permissions</div>
      <p className="mt-1 text-sm leading-6 text-slate-600">
        Choose the permissions to approve for this pairing.
      </p>
      <div className="mt-3">
        <HaPermissionSwitchList compact unframed value={permissionPolicy} onChange={setPermissionPolicy} />
      </div>
    </div>
  );
}

function HaStatusOperationalSections({ permissionPolicy }) {
  return (
    <div className="rounded-lg border border-[#e0e0e0] bg-white">
      <div className="border-b border-[#eeeeee] px-4 py-3 text-sm font-semibold text-[#212121]">Remote management</div>
      <div className="px-4 py-1">
        <HaRemoteManagementSummary permissionPolicy={permissionPolicy} />
        <HaStatusRow label="Last command" value="None pending" status="Ready" />
        <HaStatusRow label="Policy" value="Current" status="Ready" />
      </div>
    </div>
  );
}

function HaRemoteManagementSummary({ permissionPolicy }) {
  const fullControl = permissionPolicy?.accessMode !== "custom";
  const permissions = fullControl
    ? fullControlPermissionLabels
    : haAppControlPolicy
        .filter((control) => permissionPolicy?.granularControls?.[control.key])
        .map((control) => control.label);

  return (
    <div className="border-b border-[#eeeeee] py-3 text-sm">
      <div className="flex items-start gap-2 font-medium text-[#212121]">
        <HaStateDot tone="success" size="sm" />
        <span>Access mode</span>
      </div>
      <div className="mt-2 min-w-0 pl-5">
        <div className="font-medium text-[#212121]">{fullControl ? "Full remote management" : "Specific permissions"}</div>
        <div className="mt-2 flex flex-wrap gap-2">
          {permissions.map((permission) => (
            <span key={permission} className="rounded-full border border-[#b3e5fc] bg-[#e1f5fe] px-3 py-1 text-xs font-medium text-[#0277bd]">
              {permission}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

function HaUpdateNotice({ updateAgeDays }) {
  // One product alert, two severity states:
  // - Under the stale threshold: show the gentle "Update available" banner.
  // - At/over the stale threshold: replace it with "Action required" because auto-update is probably not working.
  // Do not render both together; we want users to fix auto-update instead of only performing a one-off manual update.
  const [showInstructions, setShowInstructions] = useState(false);
  const isStale = updateAgeDays >= haAppState.stale_update_threshold_days;

  if (isStale) {
    return (
      <div className="mb-5 rounded-lg border-2 border-slate-950 bg-white p-4">
        <div className="mb-2 inline-flex rounded bg-red-600 px-2 py-1 text-xs font-semibold uppercase tracking-normal text-white">
          Action required
        </div>
        <div className="text-base font-semibold text-slate-950">HomeSignal auto-update may not be working</div>
        <p className="mt-1 max-w-2xl text-sm leading-6 text-slate-700">
          This app has been out of date for <span className="underline decoration-slate-600 underline-offset-2">{updateAgeDays} days</span>. To keep the HomeSignal app compatible with HomeSignal cloud features and supported Home Assistant versions,
          auto-update should be enabled.
        </p>
        <HaUpdateInstructions tone="serious" />
      </div>
    );
  }

  return (
    <div className="mb-4 rounded-lg border border-amber-300 bg-amber-50 p-4">
      <div>
        <div>
          <div className="flex items-center gap-2">
            <span className="inline-block h-2.5 w-2.5 rounded-full bg-amber-500" />
            <div className="text-sm font-semibold text-amber-950">Update available</div>
          </div>
          <p className="mt-1 text-sm leading-6 text-amber-900">
            HomeSignal app {haAppState.latest_ha_app_version} is available. You are running {haAppState.ha_app_version}.
            <button
              type="button"
              onClick={() => setShowInstructions((value) => !value)}
              className="ml-1 underline decoration-amber-700 underline-offset-2 hover:text-amber-950"
            >
              HomeSignal App Update instructions
            </button>
          </p>
        </div>
        {showInstructions && <HaUpdateInstructions tone="advisory" />}
      </div>
    </div>
  );
}

function HaUpdateInstructions({ tone = "advisory" }) {
  const textClass = tone === "serious" ? "text-slate-800" : "text-amber-950";

  return (
    <div className={`mt-3 rounded-md border p-3 ${tone === "serious" ? "border-slate-300 bg-slate-50" : "border-amber-200 bg-white/70"}`}>
      <div className={`text-sm font-semibold ${textClass}`}>HomeSignal App Update instructions</div>
      <ul className={`mt-2 list-disc space-y-1 pl-5 text-sm leading-6 ${textClass}`}>
        <li>Open the Home Assistant app settings page.</li>
        <li>
          Verify these settings:
          <ul className="mt-1 list-disc space-y-1 pl-5">
            <li className="grid grid-cols-[120px_auto] gap-2"><span>Autoupdate:</span><strong>enabled</strong></li>
            <li className="grid grid-cols-[120px_auto] gap-2"><span>Watchdog:</span><strong>enabled</strong></li>
            <li className="grid grid-cols-[120px_auto] gap-2"><span>Start on boot:</span><strong>enabled</strong></li>
          </ul>
        </li>
        <li>Install the latest HomeSignal app version if an update is available.</li>
      </ul>
      <HaButton type="button" className="mt-3">
        Go to app settings
      </HaButton>
    </div>
  );
}

function HaHealthStatus({ items = haAppHealthSnapshot, lastReported }) {
  const [showDetails, setShowDetails] = useState(false);
  const summaryOrder = ["HomeSignal agent", "Cloud paths", "Cloud access", "Telemetry", "Account status", "HomeSignal App Update"];
  const summaryItems = summaryOrder
    .map((label) => items.find(([itemLabel]) => itemLabel === label))
    .filter(Boolean)
    .slice(0, 4);
  const reportLabel = lastReported === "Not sent yet" ? "Not reporting yet" : `Last sent ${lastReported}`;

  return (
    <div className="rounded-lg border border-[#e0e0e0] bg-white">
      <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-[#eeeeee] px-4 py-3">
        <div className="text-sm font-semibold text-[#212121]">Health status</div>
        <div className="text-xs text-[#757575]">{reportLabel}</div>
      </div>
      <div className="divide-y divide-[#f1f1f1] px-4">
        {summaryItems.map(([label, value, detail, tone]) => (
          <HaHealthSummaryItem key={label} label={label} value={value} detail={detail} tone={tone} />
        ))}
      </div>
      <div className="border-t border-[#eeeeee] px-4 py-3">
        <button
          type="button"
          onClick={() => setShowDetails((value) => !value)}
          className="text-sm font-medium text-[#039dcc] underline underline-offset-2 hover:text-[#0277bd]"
        >
          {showDetails ? "Hide details" : "Show details"}
        </button>
      </div>
      {showDetails && (
        <div className="border-t border-[#eeeeee] px-4">
          {items.map(([label, value, detail, tone]) => (
            <HaHealthLine key={label} label={label} value={value} detail={detail} tone={tone} />
          ))}
        </div>
      )}
    </div>
  );
}

function HaHealthSnapshot({ items = haAppHealthSnapshot }) {
  return (
    <div className="rounded-lg border border-[#e0e0e0] bg-white px-4 py-1">
      {items.map(([label, value, detail, tone]) => (
        <HaHealthLine key={label} label={label} value={value} detail={detail} tone={tone} />
      ))}
    </div>
  );
}

function HaHealthSummaryItem({ label, value, detail, tone }) {
  const color = tone === "warning" ? "bg-amber-500" : tone === "neutral" ? "bg-slate-400" : "bg-emerald-500";

  return (
    <div className="grid max-w-4xl gap-x-4 gap-y-0.5 py-2.5 text-sm sm:grid-cols-[190px_minmax(0,1fr)_auto] sm:items-center">
      <div className="flex min-w-0 items-center gap-2 font-medium text-[#212121]">
        <span className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${color}`} />
        <span className="truncate">{label}</span>
      </div>
      <div className="min-w-0 pl-5 text-xs leading-4 text-[#616161] sm:pl-0">{detail}</div>
      <HaHealthStatusChip value={value} />
    </div>
  );
}

function HaHealthLine({ label, value, detail, tone }) {
  const color = tone === "warning" ? "bg-amber-500" : tone === "neutral" ? "bg-slate-400" : "bg-emerald-500";

  return (
    <div className="grid max-w-4xl gap-x-4 gap-y-0.5 border-b border-[#f1f1f1] py-2.5 text-sm last:border-b-0 sm:grid-cols-[190px_minmax(0,1fr)_auto] sm:items-center">
      <div className="flex min-w-0 items-center gap-2 font-medium text-[#212121]">
        <span className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${color}`} />
        <span className="truncate">{label}</span>
      </div>
      <div className="min-w-0 pl-5 text-xs leading-4 text-[#616161] sm:pl-0">{detail}</div>
      <HaHealthStatusChip value={value} />
    </div>
  );
}

function HaHealthStatusChip({ value }) {
  return (
    <div className="pl-5 sm:pl-0 sm:text-right">
      <span className="inline-flex rounded-full bg-[#f7f8fa] px-2.5 py-0.5 text-xs font-medium text-[#757575]">
        {value}
      </span>
    </div>
  );
}

function mockClaimRequestForGuid(guid) {
  const value = guid.trim();

  if (!value) {
    return {
      ok: false,
      tone: "warning",
      title: "Enter a pairing code",
      message: "Paste the pairing code from the HomeSignal pairing page or email.",
    };
  }

  if (value.length < 8) {
    return {
      ok: false,
      tone: "warning",
      title: "Pairing code is incomplete",
      message: "Check the pairing code and try again.",
    };
  }

  const errorFixtures = {
    "mock-expired": ["Pairing request expired", "Ask HomeSignal to create a new pairing request."],
    "mock-used": ["Pairing request already used", "This pairing code has already been claimed by another app."],
    "mock-cancelled": ["Pairing request cancelled", "The HomeSignal user cancelled this pairing request."],
    "mock-network": ["Could not reach HomeSignal", "Check internet access from this app, then try again."],
    "mock-claimed": ["This app is already paired", "Unpair this app before claiming it again."],
  };

  if (errorFixtures[value]) {
    return {
      ok: false,
      tone: value === "mock-network" ? "warning" : "danger",
      title: errorFixtures[value][0],
      message: errorFixtures[value][1],
    };
  }

  if (value.startsWith("mock-missing")) {
    return {
      ok: false,
      tone: "warning",
      title: "Pairing request not found",
      message: "The pairing code was not found or is no longer active.",
    };
  }

  return {
    ok: true,
    value: {
      guid: value,
      claim_request_id: "claim_req_01J00000000000000000000000",
      organization: haAppState.organization,
      site: haAppState.site,
      customer_name: haAppState.customer_name,
      service_address: haAppState.service_address,
      requested_by: haAppState.claim_creator_name,
      requester_email: haAppState.claim_creator_email,
      created_at: "May 16, 2026, 10:42 AM",
      expires_at: "May 16, 2026, 12:42 PM",
      local_ha_app_version: haAppState.ha_app_version,
      local_home_assistant_version: "2026.5.1",
      local_supervisor_version: "2026.05.0",
    },
  };
}

function HaPairing({
  pairingStage,
  pairingCodeState,
  permissionPolicy,
  claimInviteCode,
  setClaimInviteCode,
  claimStep,
  setClaimStep,
  claimLookup,
  setClaimLookup,
  lookupClaimRequest,
  setPermissionPolicy,
  setPairingStage,
  setPairingCodeState,
  setHaAppScreen,
  onPairingCommitted,
}) {
  const browserPairingContext = readAutoPairingContext();
  const browserGuid = browserPairingContext?.pairing_id || "";
  const [showUnpairConfirm, setShowUnpairConfirm] = useState(false);

  const claimDetails = claimLookup.result?.ok ? claimLookup.result.value : null;

  useEffect(() => {
    if (!browserGuid || claimStep !== "guid" || claimLookup.state !== "idle") return;
    setClaimInviteCode(browserGuid);
    lookupClaimRequest(browserGuid);
  }, [browserGuid, claimLookup.state, claimStep, lookupClaimRequest, setClaimInviteCode]);

  if (pairingStage === "connected") {
    return (
      <div className="p-4">
        <div className="max-w-2xl">
          <div className="mb-6">
            <HaPairingProgress stage={pairingStage} codeState={pairingCodeState} claimStep={claimStep} />
          </div>
          <div className="text-center">
            <div className="inline-flex items-center justify-center gap-2 text-2xl font-medium tracking-normal text-[#212121]">
              <span className="h-2.5 w-2.5 rounded-full bg-emerald-500" aria-hidden="true" />
              <span>Paired with HomeSignal</span>
            </div>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              This app is ready for approved remote management.
            </p>
          </div>

          <HaManagedByCard
            className="mt-6"
            requesterName={haAppState.claim_creator_name}
            requesterEmail={haAppState.claim_creator_email}
          />
          <div className="mt-10 flex flex-col items-center justify-center gap-4">
            <HaButton type="button" onClick={() => setHaAppScreen("healthy")}>
              Return to app status page
            </HaButton>
            <div className="flex flex-col items-center gap-5 pt-2">
              <a
                href="https://app.homesignal.local"
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center text-sm font-medium text-[#039dcc] underline underline-offset-2 hover:text-[#0277bd]"
              >
                Open HomeSignal portal
                <ExternalLinkIcon className="ml-1 h-3.5 w-3.5" />
              </a>
              <button
                type="button"
                onClick={() => setHaAppScreen("advanced")}
                className="text-sm font-medium text-[#039dcc] underline underline-offset-2 hover:text-[#0277bd]"
              >
                Advanced options
              </button>
            </div>
          </div>
        </div>
        {showUnpairConfirm && (
          <HaConfirmDialog
            title="Unpair from HomeSignal?"
            message="This will remove the HomeSignal cloud association for this Home Assistant app. Local Home Assistant will keep running."
            confirmLabel="Yes, unpair"
            cancelLabel="No, keep paired"
            onCancel={() => setShowUnpairConfirm(false)}
            onConfirm={() => {
              setShowUnpairConfirm(false);
              setHaAppScreen("onboarding");
            }}
          />
        )}
      </div>
    );
  }

  return (
    <div className="p-4">
      <div className="max-w-2xl">
        <div className="mb-6">
          <HaPairingProgress stage={pairingStage} codeState={pairingCodeState} claimStep={claimStep} />
        </div>
        <HaClaimFrame
          step={claimStep}
          browserGuid={browserGuid}
          claimInviteCode={claimInviteCode}
          setClaimInviteCode={setClaimInviteCode}
          claimLookup={claimLookup}
          claimDetails={claimDetails}
          permissionPolicy={permissionPolicy}
          setPermissionPolicy={setPermissionPolicy}
          onUseBrowserGuid={() => {
            setClaimInviteCode(browserGuid);
            lookupClaimRequest(browserGuid);
          }}
          onSelectMockCase={(value) => {
            setClaimInviteCode(value);
            lookupClaimRequest(value);
          }}
          onLookup={() => lookupClaimRequest()}
          onBackToGuid={() => {
            setClaimStep("guid");
            setPairingStage("preflight");
            setPairingCodeState("success");
          }}
          onCommitPairing={() => {
            onPairingCommitted(claimDetails?.guid || claimInviteCode.trim());
            setClaimStep("success");
            setPairingStage("connected");
          }}
        />
      </div>
    </div>
  );
}

function HaClaimFrame({
  step,
  browserGuid,
  claimInviteCode,
  setClaimInviteCode,
  claimLookup,
  claimDetails,
  permissionPolicy,
  setPermissionPolicy,
  onUseBrowserGuid,
  onSelectMockCase,
  onLookup,
  onBackToGuid,
  onCommitPairing,
}) {
  return (
    <div>
      {step === "guid" && (
        <HaClaimGuidStep
          browserGuid={browserGuid}
          claimInviteCode={claimInviteCode}
          setClaimInviteCode={setClaimInviteCode}
          onUseBrowserGuid={onUseBrowserGuid}
          onSelectMockCase={onSelectMockCase}
          onLookup={onLookup}
        />
      )}

      {step === "approve" && (
        <HaClaimReviewStep
          claimLookup={claimLookup}
          claimDetails={claimDetails}
          claimInviteCode={claimInviteCode}
          permissionPolicy={permissionPolicy}
          setPermissionPolicy={setPermissionPolicy}
          onBackToGuid={onBackToGuid}
          onCommitPairing={onCommitPairing}
        />
      )}
    </div>
  );
}

function HaClaimGuidStep({ browserGuid, claimInviteCode, setClaimInviteCode, onUseBrowserGuid, onSelectMockCase, onLookup }) {
  return (
    <div>
      <div className="rounded-lg border border-[#e0e0e0] bg-[#f7f8fa] px-5 py-5">
        {browserGuid && (
          <p className="mb-4 text-sm leading-6 text-[#616161]">
            A pairing code is available from this browser. Review the request before pairing.
          </p>
        )}

        <label className="block text-sm font-semibold text-[#212121]" htmlFor="claim-guid">
          Pairing code
        </label>
        <div className="mt-2 flex flex-col gap-3 sm:flex-row">
          <input
            id="claim-guid"
            type="text"
            value={claimInviteCode}
            onChange={(event) => setClaimInviteCode(event.target.value)}
            spellCheck={false}
            autoCapitalize="none"
            autoComplete="off"
            className="min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-3 py-3 font-mono text-sm font-semibold tracking-normal text-slate-950"
            aria-label="Pairing code"
          />
          <HaButton type="button" onClick={onLookup} disabled={!claimInviteCode.trim()}>
            Continue
          </HaButton>
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          {browserGuid && (
            <button
              type="button"
              onClick={onUseBrowserGuid}
              className="text-sm font-medium text-[#039dcc] underline underline-offset-2 hover:text-[#0277bd]"
            >
              Use browser pairing code
            </button>
          )}
        </div>
      </div>

      <details className="mt-4 rounded-md border border-dashed border-slate-300 bg-slate-50/70 px-4 py-3">
        <summary className="cursor-pointer select-none text-xs font-medium text-slate-500">Mock cases</summary>
        <div className="mt-3 flex flex-wrap gap-2">
          {["mock-good", "mock-expired", "mock-used", "mock-network", "mock-claimed"].map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => onSelectMockCase(value)}
              className="rounded-md border border-slate-300 bg-white px-2.5 py-1 font-mono text-xs text-slate-600 hover:bg-slate-100"
            >
              {value}
            </button>
          ))}
        </div>
      </details>
    </div>
  );
}

function HaClaimReviewStep({
  claimLookup,
  claimDetails,
  claimInviteCode,
  permissionPolicy,
  setPermissionPolicy,
  onBackToGuid,
  onCommitPairing,
}) {
  if (claimLookup.state === "loading") {
    return (
      <div className="rounded-lg border border-[#e0e0e0] bg-[#f7f8fa] px-5 py-5">
        <div className="flex items-center gap-3">
          <span className="h-5 w-5 animate-spin rounded-full border-2 border-slate-300 border-t-[#039dcc]" />
          <div className="text-sm font-semibold text-slate-900">Fetching pairing request</div>
        </div>
      </div>
    );
  }

  if (claimLookup.state === "error") {
    return (
      <div className="rounded-lg border border-[#e0e0e0] bg-[#f7f8fa] px-5 py-5">
        <HaClaimAlert
          tone={claimLookup.result?.tone || "warning"}
          title={claimLookup.result?.title || "Unable to load pairing request"}
          message={claimLookup.result?.message || "Try another pairing code."}
        />
        <div className="mt-4">
          <HaButton type="button" variant="secondary" onClick={onBackToGuid}>
            Enter a different pairing code
          </HaButton>
        </div>
      </div>
    );
  }

  if (!claimDetails) return null;

  return (
    <div>
      <div className="rounded-lg border border-[#e0e0e0] bg-[#f7f8fa] px-5 py-5">
        <HaPairingRequestSummary claimDetails={claimDetails} />
        <HaApprovalPanel
          claimDetails={claimDetails}
          pairingCode={claimDetails.guid || claimInviteCode}
        />
      </div>
      <HaRemoteManagementApproval permissionPolicy={permissionPolicy} setPermissionPolicy={setPermissionPolicy} />

      <div className="mt-5 flex flex-wrap items-center gap-2">
        <HaButton type="button" onClick={onCommitPairing}>
          Approve and Pair
        </HaButton>
        <HaButton type="button" variant="secondary" onClick={onBackToGuid}>
          Use a different pairing code
        </HaButton>
      </div>
    </div>
  );
}

function HaClaimAlert({ tone, title, message }) {
  const classes =
    tone === "success"
      ? "border-emerald-200 bg-emerald-50 text-emerald-950"
      : tone === "danger"
        ? "border-red-200 bg-red-50 text-red-950"
        : tone === "warning"
          ? "border-amber-300 bg-amber-50 text-amber-950"
          : "border-slate-200 bg-white text-slate-900";

  return (
    <div className={`rounded-md border px-4 py-3 ${classes}`}>
      <div className="text-sm font-semibold">{title}</div>
      <p className="mt-1 text-sm leading-6 opacity-85">{message}</p>
    </div>
  );
}

function HaPairingProgress({ stage, codeState, claimStep }) {
  const steps = [
    ["guid", "Enter code"],
    ["approve", "Approve pairing"],
    ["connected", "Paired"],
  ];
  const currentIndex =
    stage === "connected"
      ? 2
      : claimStep === "approve"
        ? 1
        : 0;
  const stepStates = steps.map(([key, label], index) => {
    const complete = stage === "connected" || index < currentIndex;
    const active = stage !== "connected" && index === currentIndex;
    const status =
      key === "approve" && claimStep === "approve" && codeState === "loading"
        ? "Fetching"
        : key === "approve" && claimStep === "approve" && codeState === "rate_limited"
          ? "Needs attention"
          : key === "connected" && active
            ? `Paired with ${haAppState.organization}`
            : complete
              ? "Complete"
              : active
                ? "Current"
                : "Pending";
    const markerClass = complete
      ? "border border-[#039dcc] bg-[#039dcc] text-white"
      : active
        ? "border-2 border-[#039dcc] bg-white text-[#0277bd]"
        : "border border-[#e0e0e0] bg-white text-[#757575]";

    return { key, label, index, complete, active, status, markerClass };
  });

  return (
    <div className="px-1 py-2">
      <div className="grid grid-cols-[minmax(70px,1fr)_minmax(36px,0.8fr)_minmax(90px,1fr)_minmax(36px,0.8fr)_minmax(70px,1fr)] items-start">
        {stepStates.map((step, index) => (
          <Fragment key={step.key}>
            <div className="min-w-0 text-center">
              <div className={`mx-auto flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold ${step.markerClass}`}>
                {step.complete ? (
                  <svg aria-hidden="true" viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2.7" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M20 6 9 17l-5-5" />
                  </svg>
                ) : (
                  step.index + 1
                )}
              </div>
              <div className={`mt-2 truncate text-sm font-medium ${step.active || step.complete ? "text-[#212121]" : "text-[#616161]"}`}>{step.label}</div>
              {step.active && <div className="mt-0.5 truncate text-xs text-[#757575]">{step.status}</div>}
            </div>
            {index < stepStates.length - 1 && (
              <div className={`mt-[14px] h-px ${index < currentIndex ? "bg-[#039dcc]" : "bg-[#e0e0e0]"}`} />
            )}
          </Fragment>
        ))}
      </div>
    </div>
  );
}

function HaConfirmDialog({ title, message, confirmLabel, cancelLabel, onConfirm, onCancel }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
      <div className="w-full max-w-sm rounded-lg border border-[#e0e0e0] bg-white p-5 shadow-xl" role="dialog" aria-modal="true" aria-labelledby="ha-confirm-title">
        <div id="ha-confirm-title" className="text-lg font-medium text-[#212121]">{title}</div>
        <p className="mt-2 text-sm leading-6 text-[#616161]">{message}</p>
        <div className="mt-5 flex flex-wrap justify-end gap-2">
          <HaButton type="button" variant="secondary" onClick={onCancel}>
            {cancelLabel}
          </HaButton>
          <HaButton type="button" variant="danger" onClick={onConfirm}>
            {confirmLabel}
          </HaButton>
        </div>
      </div>
    </div>
  );
}

function HaPermissionsPage({ permissionPolicy, savedPermissionPolicy, permissionSavedAt, setPermissionPolicy, onSave }) {
  const hasUnsavedChanges = JSON.stringify(permissionPolicy) !== JSON.stringify(savedPermissionPolicy);

  return (
    <div className="grid max-w-2xl gap-4 p-4">
      <HaCard
        className="min-w-0"
        title="Remote access permissions"
        subtitle="Choose which HomeSignal actions cloud users may request. Home Assistant grants the installed app local capability; this policy narrows what the cloud is allowed to use."
      >
        <div className="divide-y divide-slate-200 rounded-md border border-slate-200 bg-white">
          <HaPermissionSwitchList value={permissionPolicy} onChange={setPermissionPolicy} />
        </div>

        <div className="mt-5 flex flex-wrap items-center gap-3">
          <HaButton type="button" onClick={onSave} disabled={!hasUnsavedChanges}>
            Save permissions
          </HaButton>
          <div className="text-sm text-slate-600">
            {hasUnsavedChanges ? "Unsaved changes" : permissionSavedAt ? `Saved ${permissionSavedAt}` : "No changes"}
          </div>
        </div>
      </HaCard>

      <HaCard title="Unavailable permissions">
        <p className="mb-3 text-sm leading-6 text-slate-600">
          Future or unsupported controls stay locked until a specific command contract exists. Cloud users cannot silently grant themselves more access later.
        </p>
        {unsupportedHaAppControls.map(([label, detail]) => (
          <HaChecklistItem key={label} label={label} state="unavailable" detail={detail} />
        ))}
      </HaCard>
    </div>
  );
}

function HaAdvancedPage({ setHaAppScreen }) {
  const [showUnpairConfirm, setShowUnpairConfirm] = useState(false);
  const [logCaptureLevel, setLogCaptureLevel] = useState(haAppLogStorageSummary.capture_level);

  return (
    <div className="p-4">
      <div className="rounded-lg border border-[#e0e0e0] bg-white px-5 py-4">
        <div className="text-sm font-semibold text-[#212121]">Pairing</div>
        <div className="mt-4 grid gap-3">
          <HaAdvancedAction
            title="Reset pairing"
            detail="Retry the local pairing flow if this app is paired but not reporting correctly."
            action={<HaButton type="button" onClick={() => setHaAppScreen("pairing")}>Open pairing</HaButton>}
          />
          <HaAdvancedAction
            title="Unpair from HomeSignal"
            detail="Remove the HomeSignal cloud association for this app. Local Home Assistant will keep running."
            action={
              <button
                type="button"
                onClick={() => setShowUnpairConfirm(true)}
                className="text-sm font-medium text-rose-900/70 underline underline-offset-2 hover:text-rose-950"
              >
                Unpair from HomeSignal
              </button>
            }
          />
        </div>
      </div>

      <HaLogTailPanel
        captureLevel={logCaptureLevel}
        setCaptureLevel={setLogCaptureLevel}
      />

      <div className="mt-5 rounded-lg border border-[#e0e0e0] bg-white px-5 py-4">
        <div className="text-sm font-semibold text-[#212121]">Local metadata</div>
        <div className="mt-4 grid gap-x-6 gap-y-3 text-sm sm:grid-cols-[190px_1fr]">
          <div className="text-[#757575]">Installation ID</div>
          <div className="font-mono text-[#212121]">inst_01J00000000000000000000000</div>
          <div className="text-[#757575]">Device ID</div>
          <div className="font-mono text-[#212121]">{haAppState.device_id}</div>
          <div className="text-[#757575]">Claim display metadata</div>
          <div className="text-[#212121]">Revision 1 · last applied May 14, 2026, 12:03 PM</div>
          <div className="text-[#757575]">Local policy</div>
          <div className="text-[#212121]">Revision 1 · synced Just now</div>
          <div className="text-[#757575]">Policy hash</div>
          <div className="break-all font-mono text-xs text-[#212121]">sha256:8f8f9a3d4d38c7b2</div>
        </div>
      </div>
      {showUnpairConfirm && (
        <HaConfirmDialog
          title="Unpair from HomeSignal?"
          message="This will remove the HomeSignal cloud association for this Home Assistant app. Local Home Assistant will keep running."
          confirmLabel="Yes, unpair"
          cancelLabel="No, keep paired"
          onCancel={() => setShowUnpairConfirm(false)}
          onConfirm={() => {
            setShowUnpairConfirm(false);
            setHaAppScreen("onboarding");
          }}
        />
      )}
    </div>
  );
}

function HaLogTailPanel({ captureLevel, setCaptureLevel }) {
  const [query, setQuery] = useState("");
  const [minLevel, setMinLevel] = useState("debug");
  const [paused, setPaused] = useState(false);
  const [wrapLines, setWrapLines] = useState(true);
  const normalizedQuery = query.trim().toLowerCase();
  const visibleLogs = mockHaAppLogEntries.filter((entry) => {
    const meetsLevel = haAppLogLevelRank[entry.level] >= haAppLogLevelRank[minLevel];
    if (!meetsLevel) return false;
    if (!normalizedQuery) return true;

    return [entry.level, entry.component, entry.reason, entry.message]
      .join(" ")
      .toLowerCase()
      .includes(normalizedQuery);
  });
  const copyVisibleLogs = () => {
    copyTextToClipboard(visibleLogs.map(formatLogTailLine).join("\n"));
  };

  return (
    <div className="mt-5 rounded-lg border border-[#e0e0e0] bg-white px-5 py-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div className="text-sm font-semibold text-[#212121]">App logs</div>
          <p className="mt-1 max-w-2xl text-sm leading-5 text-[#616161]">
            App runtime logs from this app only. These are not Home Assistant system logs.
          </p>
        </div>
        <div className="flex items-center gap-2 rounded-full border border-[#e0e0e0] px-3 py-1.5 text-xs font-medium text-[#616161]">
          <HaStateDot tone={paused ? "neutral" : "success"} size="sm" />
          {paused ? "Paused" : "Live tail"}
        </div>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-4">
        <HaLogStat label="Capture level">
          <select
            value={captureLevel}
            onChange={(event) => setCaptureLevel(event.target.value)}
            className="w-full rounded-md border border-[#e0e0e0] bg-white px-2 py-1 text-sm text-[#212121] focus:border-[#039dcc] focus:outline-none"
          >
            {haAppLogLevels.map((level) => (
              <option key={level} value={level}>{level}</option>
            ))}
          </select>
        </HaLogStat>
        <HaLogStat label="Storage used" value={`${haAppLogStorageSummary.used} / ${haAppLogStorageSummary.budget}`} />
        <HaLogStat label="Retained from" value={formatShortDate(haAppLogStorageSummary.retained_from)} />
        <HaLogStat label="Suppressed" value={`${haAppLogStorageSummary.suppressed_count} entries`} />
      </div>

      <div className="mt-4 rounded-md border border-[#e0e0e0] bg-[#fafafa] p-3">
        <div className="flex flex-wrap items-center gap-2">
          <label className="min-w-[220px] flex-1">
            <span className="sr-only">Search logs</span>
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search logs"
              className="w-full rounded-md border border-[#e0e0e0] bg-white px-3 py-2 text-sm text-[#212121] placeholder:text-[#9e9e9e] focus:border-[#039dcc] focus:outline-none"
            />
          </label>
          <label className="flex items-center gap-2 text-sm text-[#616161]">
            Level
            <select
              value={minLevel}
              onChange={(event) => setMinLevel(event.target.value)}
              className="rounded-md border border-[#e0e0e0] bg-white px-2 py-2 text-sm text-[#212121] focus:border-[#039dcc] focus:outline-none"
            >
              {haAppLogLevels.map((level) => (
                <option key={level} value={level}>{level}+</option>
              ))}
            </select>
          </label>
          <button
            type="button"
            onClick={() => setPaused((current) => !current)}
            className="rounded-full px-3 py-2 text-sm font-medium text-[#039dcc] hover:bg-[#e1f5fe]"
          >
            {paused ? "Resume" : "Pause"}
          </button>
          <button
            type="button"
            onClick={() => setWrapLines((current) => !current)}
            className="rounded-full px-3 py-2 text-sm font-medium text-[#039dcc] hover:bg-[#e1f5fe]"
          >
            {wrapLines ? "No wrap" : "Wrap"}
          </button>
          <button
            type="button"
            onClick={copyVisibleLogs}
            className="rounded-full px-3 py-2 text-sm font-medium text-[#039dcc] hover:bg-[#e1f5fe]"
          >
            Copy visible
          </button>
        </div>

        <div className="mt-3 overflow-hidden rounded-md border border-[#263244] bg-[#0f172a] font-mono text-xs text-slate-100">
          <div className="grid grid-cols-[94px_72px_132px_1fr] gap-2 border-b border-slate-700/80 bg-slate-950/70 px-3 py-2 text-[11px] uppercase tracking-normal text-slate-400 max-sm:hidden">
            <div>Time</div>
            <div>Level</div>
            <div>Component</div>
            <div>Message</div>
          </div>
          <div className="max-h-[380px] overflow-auto">
            {visibleLogs.length > 0 ? (
              visibleLogs.map((entry) => (
                <div
                  key={entry.id}
                  className="grid gap-2 border-b border-slate-800/80 px-3 py-2 last:border-b-0 sm:grid-cols-[94px_72px_132px_1fr]"
                >
                  <div className="whitespace-nowrap text-slate-400">{formatLogTailTime(entry.ts)}</div>
                  <div><HaLogLevelBadge level={entry.level} /></div>
                  <div className="truncate text-slate-300">{entry.component}</div>
                  <div className={wrapLines ? "min-w-0 leading-5 text-slate-100" : "min-w-0 truncate text-slate-100"}>
                    <span className="text-slate-500">{entry.reason}</span>
                    <span className="text-slate-600"> | </span>
                    {entry.message}
                  </div>
                </div>
              ))
            ) : (
              <div className="px-3 py-8 text-center text-sm text-slate-400">No log entries match the current filters.</div>
            )}
          </div>
        </div>

        <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-[#757575]">
          <span>{visibleLogs.length} visible entries</span>
          <span>Newest retained: {formatShortDate(haAppLogStorageSummary.retained_to)}</span>
        </div>
      </div>
    </div>
  );
}

function HaLogStat({ label, value, children }) {
  return (
    <div className="rounded-md border border-[#eeeeee] bg-white px-3 py-2">
      <div className="text-xs font-medium uppercase tracking-normal text-[#757575]">{label}</div>
      <div className="mt-1 text-sm font-medium text-[#212121]">{children || value}</div>
    </div>
  );
}

function HaLogLevelBadge({ level }) {
  const styles =
    level === "error"
      ? "border-red-400 bg-red-500/15 text-red-200"
      : level === "warning"
        ? "border-amber-400 bg-amber-500/15 text-amber-200"
        : level === "debug"
          ? "border-slate-500 bg-slate-500/15 text-slate-300"
          : "border-sky-400 bg-sky-500/15 text-sky-200";

  return (
    <span className={`inline-flex min-w-14 justify-center rounded border px-1.5 py-0.5 text-[11px] font-semibold uppercase ${styles}`}>
      {level}
    </span>
  );
}

function formatLogTailTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("en-US", {
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function formatLogTailLine(entry) {
  return `${entry.ts} ${entry.level.toUpperCase()} ${entry.component} ${entry.reason} ${entry.message}`;
}

function HaAdvancedAction({ title, detail, action }) {
  return (
    <div className="grid gap-3 border-b border-[#eeeeee] py-3 last:border-b-0 xl:grid-cols-[1fr_auto] xl:items-center">
      <div>
        <div className="text-sm font-medium text-[#212121]">{title}</div>
        <div className="mt-1 text-sm leading-5 text-[#616161]">{detail}</div>
      </div>
      <div>{action}</div>
    </div>
  );
}

function HaStep({ label, state }) {
  const styles =
    state === "ready"
      ? "border-emerald-300 bg-emerald-50 text-emerald-900"
      : state === "needs_attention"
        ? "border-amber-300 bg-amber-50 text-amber-900"
        : "border-slate-300 bg-slate-50 text-slate-600";

  const text =
    state === "ready"
      ? "Ready"
      : state === "needs_attention"
        ? "Needs attention"
        : "Not available";

  return (
    <div className={`rounded-md border px-3 py-3 ${styles}`}>
      <div className="text-sm font-semibold">{label}</div>
      <div className="mt-1 text-xs">{text}</div>
    </div>
  );
}

function HaStatusRow({ label, value, status, note }) {
  const normalized = status.toLowerCase().replace(/\s+/g, "_");
  const tone =
    normalized === "ready"
      ? "text-emerald-700"
      : normalized === "needs_attention"
        ? "text-amber-700"
        : "text-slate-500";

  return (
    <div className="grid grid-cols-[minmax(130px,0.9fr)_1fr] gap-3 border-b border-[#eeeeee] py-3 text-sm last:border-b-0">
      <div className="flex items-start gap-2 font-medium text-[#212121]">
        <HaStateDot tone={normalized === "ready" ? "success" : normalized === "needs_attention" ? "warning" : "neutral"} size="sm" />
        <span>{label}</span>
      </div>
      <div className="min-w-0">
        <div className="font-medium text-[#212121]">{value}</div>
        <div className={`text-xs ${tone}`}>{status}</div>
        {note && <div className="text-xs text-[#757575]">{note}</div>}
      </div>
    </div>
  );
}

function HaChecklistItem({ label, state, detail }) {
  const text =
    state === "ready"
      ? "Ready"
      : state === "needs_attention"
        ? "Needs attention"
        : "Not available";

  return (
    <div className="flex items-start gap-2 border-b border-slate-200 py-2 last:border-b-0">
      <span className="mt-1">
        <HaStateDot tone={state === "ready" ? "success" : state === "needs_attention" ? "warning" : "neutral"} size="sm" />
      </span>
      <div className="min-w-0">
        <div className="text-sm font-medium text-slate-900">{label}</div>
        <div className="text-xs text-slate-500">{detail || text}</div>
      </div>
    </div>
  );
}

function HaCompactReadiness({ label, value, status }) {
  const ready = status === "Ready";

  return (
    <div className="flex items-start gap-2 rounded-md border border-slate-200 bg-white px-3 py-2">
      <span className="mt-1">
        <HaStateDot tone={ready ? "success" : "warning"} size="sm" />
      </span>
      <div className="min-w-0">
        <div className="text-sm font-semibold text-slate-900">{label}</div>
        <div className="text-xs leading-5 text-slate-600">{value}</div>
      </div>
    </div>
  );
}

function HaSwitchVisual({ enabled }) {
  return (
    <span className={`relative inline-flex h-[14px] w-[34px] shrink-0 rounded-full ${enabled ? "bg-[#9bd7ea]" : "bg-slate-300"}`}>
      <span
        className={`absolute top-1/2 h-5 w-5 -translate-y-1/2 rounded-full shadow transition ${
          enabled ? "left-[17px] bg-[#039dcc]" : "left-[-3px] bg-white"
        }`}
      />
    </span>
  );
}

function HaPermissionSwitchList({ compact = false, unframed = false, value = initialHaAppPermissionPolicy, onChange }) {
  const [infoState, setInfoState] = useState(null);
  const [showFullPermissionDetails, setShowFullPermissionDetails] = useState(false);
  const hoverTimer = useRef(null);
  const accessMode = value.accessMode;
  const granularControls = value.granularControls;
  const fullControl = accessMode === "full";

  const clearHoverTimer = () => {
    if (hoverTimer.current) {
      window.clearTimeout(hoverTimer.current);
      hoverTimer.current = null;
    }
  };

  const openOnHover = (key) => {
    clearHoverTimer();
    hoverTimer.current = window.setTimeout(() => {
      setInfoState({ key, mode: "hover" });
    }, 300);
  };

  const closeInfo = (key) => {
    clearHoverTimer();
    setInfoState((current) => (current?.key === key ? null : current));
  };

  const toggleClicked = (key) => {
    clearHoverTimer();
    setInfoState((current) => (current?.key === key && current.mode === "clicked" ? null : { key, mode: "clicked" }));
  };

  useEffect(() => {
    if (!infoState) {
      return undefined;
    }

    const closeOnOutsideClick = (event) => {
      if (!event.target.closest("[data-ha-info-popover]")) {
        setInfoState(null);
      }
    };
    const closeOnPointerMove = (event) => {
      if (infoState.mode === "clicked" && !event.target.closest("[data-ha-info-popover]")) {
        setInfoState(null);
      }
    };
    const closeOnEscape = (event) => {
      if (event.key === "Escape") {
        setInfoState(null);
      }
    };

    document.addEventListener("pointerdown", closeOnOutsideClick);
    document.addEventListener("pointermove", closeOnPointerMove);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsideClick);
      document.removeEventListener("pointermove", closeOnPointerMove);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [infoState]);

  useEffect(() => () => clearHoverTimer(), []);

  const setAccessMode = (nextMode) => {
    if (nextMode === "custom") {
      setShowFullPermissionDetails(false);
    }
    onChange?.({ ...value, accessMode: nextMode });
  };

  const toggleGranularControl = (key) => {
    onChange?.({
      ...value,
      granularControls: {
        ...granularControls,
        [key]: !granularControls[key],
      },
    });
  };

  return (
    <>
      <div className={`grid gap-3 ${unframed ? "" : "p-4"}`}>
        <HaAccessModeOption
          id={`ha-access-full-${compact ? "compact" : "full"}`}
          checked={accessMode === "full"}
          title="Grant full permissions"
          badge="Default"
          alwaysShowChildren
          onSelect={() => setAccessMode("full")}
        >
          <div className="mt-1 text-sm leading-5 text-slate-600">
            Allow broad remote management permissions.
            <button
              type="button"
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                setShowFullPermissionDetails((current) => !current);
              }}
              className="mt-1 block font-medium text-[#039dcc] underline underline-offset-2 hover:text-[#0277bd]"
              aria-expanded={showFullPermissionDetails}
            >
              {showFullPermissionDetails ? "Hide permissions" : "See all permissions"}
            </button>
          </div>
          {showFullPermissionDetails && (
            <div className="mt-3 grid gap-3 rounded-md border border-slate-200 bg-white/80 p-3 sm:grid-cols-2">
              {fullControlPermissionGroups.map(([group, permissions]) => (
                <div key={group}>
                  <div className="text-xs font-semibold uppercase tracking-normal text-slate-500">{group}</div>
                  <ul className="mt-1 space-y-1 text-sm leading-5 text-slate-700">
                    {permissions.map((permission) => (
                      <li key={permission}>{permission}</li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          )}
        </HaAccessModeOption>

        <HaAccessModeOption
          id={`ha-access-custom-${compact ? "compact" : "full"}`}
          checked={accessMode === "custom"}
          title="Choose custom permissions"
          detail="Select individual remote permissions before pairing."
          onSelect={() => setAccessMode("custom")}
        />
      </div>

      {accessMode === "custom" && (
        <div className={`${unframed ? "mt-3 rounded-md border border-slate-200" : "border-t border-slate-200"}`}>
          {haAppControlPolicy.map((control) => {
            const effectiveControl = { ...control, enabled: granularControls[control.key] };
            return (
              <HaSwitchRow
                key={control.key}
                control={effectiveControl}
                compact={compact}
                infoOpen={infoState?.key === control.key}
                infoMode={infoState?.key === control.key ? infoState.mode : "closed"}
                onInfoEnter={() => openOnHover(control.key)}
                onInfoLeave={() => closeInfo(control.key)}
                onInfoClick={() => toggleClicked(control.key)}
                onToggle={() => toggleGranularControl(control.key)}
              />
            );
          })}
        </div>
      )}
    </>
  );
}

function HaAccessModeOption({ id, checked, title, badge, detail, alwaysShowChildren = false, onSelect, children }) {
  return (
    <label
      htmlFor={id}
      className={`block cursor-pointer rounded-md border p-3 ${
        checked ? "border-slate-200 bg-sky-50/70" : "border-slate-200 bg-white hover:bg-slate-50"
      }`}
    >
      <div className="flex items-start gap-3">
        <input
          id={id}
          type="radio"
          name={`${id.includes("compact") ? "compact" : "full"}-ha-access-mode`}
          checked={checked}
          onChange={onSelect}
          className="mt-0.5 h-4 w-4 border-slate-300 accent-[#03a9f4]"
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <div className="text-sm font-semibold text-slate-950">{title}</div>
            {badge && (
              <span className="rounded-full border border-slate-200 bg-white/75 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-normal text-slate-500">
                {badge}
              </span>
            )}
          </div>
          {detail && <div className="mt-1 text-sm leading-5 text-slate-600">{detail}</div>}
          {(checked || alwaysShowChildren) && children}
        </div>
      </div>
    </label>
  );
}

function HaSwitchRow({ control, compact = false, infoOpen = false, infoMode = "closed", onInfoEnter, onInfoLeave, onInfoClick, onToggle }) {
  return (
    <div className={`grid grid-cols-[1fr_auto] items-center gap-4 px-4 ${compact ? "py-3" : "py-4"}`}>
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <div className="text-base font-normal leading-6 text-slate-950">{control.label}</div>
          <HaInfoHint
            control={control}
            open={infoOpen}
            mode={infoMode}
            onEnter={onInfoEnter}
            onLeave={onInfoLeave}
            onClick={onInfoClick}
          />
        </div>
        <p className="mt-1 text-sm leading-5 text-slate-600">{control.description}</p>
        {!compact && (
          <div className="mt-2 flex flex-wrap gap-2 text-xs">
            <span className="rounded border border-slate-300 bg-slate-50 px-2 py-1 text-slate-600">{control.boundary}</span>
            <span className={`rounded border px-2 py-1 ${
              control.audit === "sensitive"
                ? "border-amber-300 bg-amber-50 text-amber-800"
                : "border-slate-300 bg-slate-50 text-slate-600"
            }`}>
              {control.audit} audit
            </span>
          </div>
        )}
      </div>
      {onToggle ? (
        <button
          type="button"
          aria-label={`${control.enabled ? "Disable" : "Enable"} ${control.label}`}
          aria-pressed={control.enabled}
          onClick={onToggle}
          className="rounded-full focus:outline-none focus:ring-2 focus:ring-[#03a9f4] focus:ring-offset-2"
        >
          <HaSwitchVisual enabled={control.enabled} />
        </button>
      ) : (
        <HaSwitchVisual enabled={control.enabled} />
      )}
    </div>
  );
}

function HaInfoHint({ control, open, mode, onEnter, onLeave, onClick }) {
  return (
    <span
      data-ha-info-popover
      className="relative inline-flex"
      onMouseEnter={onEnter}
      onMouseLeave={onLeave}
    >
      <button
        type="button"
        aria-label={`More about ${control.label}`}
        aria-expanded={open}
        onClick={(event) => {
          event.stopPropagation();
          onClick();
        }}
        className={`inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border text-[10px] font-semibold leading-none ${
          open
            ? "border-[#03a9f4] bg-sky-50 text-[#0288d1]"
            : "border-slate-300 bg-slate-50 text-slate-600 hover:border-slate-400"
        }`}
      >
        i
      </button>
      {open && (
        <span className="absolute left-0 top-7 z-20 w-72 rounded-md border border-slate-300 bg-white p-3 text-left text-xs leading-5 text-slate-700 shadow-lg">
          <span className="mb-1 flex items-center justify-between gap-3">
            <span className="block font-semibold text-slate-950">{control.label}</span>
            {mode === "clicked" && <span className="text-[10px] font-semibold uppercase tracking-normal text-slate-400">Open</span>}
          </span>
          <span className="mt-1 block">
            <span className="font-semibold">Why: </span>
            {control.why}
          </span>
          <span className="mt-1 block">
            <span className="font-semibold">Who: </span>
            {control.actor}
          </span>
          <span className="mt-1 block">
            <span className="font-semibold">Boundary: </span>
            {control.boundary}
          </span>
        </span>
      )}
    </span>
  );
}

function SchemaCoverage() {
  const rows = Object.entries(modelCoverage).map(([name, status]) => [
    <CoverageText status={status}>{name}</CoverageText>,
    schema[status],
  ]);

  return (
    <Screen title="Schema coverage" subtitle="Data-model backing for the mock. Warning rows should not be treated as implemented schema.">
      <Section title="Table and feature coverage">
        <Table columns={["Model / feature", "Coverage"]} rows={rows} />
      </Section>
    </Screen>
  );
}

function HomeIcon({ category, wiringId }) {
  const normalized = category || "residential";
  const label =
    normalized === "business"
      ? "Business site"
      : normalized === "other"
        ? "Site"
        : "Residential site";
  const symbol = normalized === "business" ? "▦" : normalized === "other" ? "•" : "⌂";
  const icon = (
    <span title={label} className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded text-base leading-none text-slate-500">
      {symbol}
    </span>
  );

  if (wiringId) {
    return <WiringFrame id={wiringId}>{icon}</WiringFrame>;
  }

  return icon;
}

function PresenceDot({ state, wiringId }) {
  const color =
    state === "online"
      ? "bg-emerald-500"
      : state === "degraded"
        ? "bg-amber-500"
        : "bg-slate-400";
  const dot = <span className={`inline-block h-3 w-3 rounded-full ${color}`} />;

  if (wiringId) {
    return <WiringFrame id={wiringId}>{dot}</WiringFrame>;
  }

  return dot;
}

function StatusPill({ state, label, withDot = true }) {
  const classes =
    state === "online"
      ? "border-emerald-300 bg-emerald-50 text-emerald-800"
      : state === "warning"
        ? "border-amber-300 bg-amber-50 text-amber-900"
        : "border-slate-300 bg-slate-50 text-slate-700";

  return (
    <span className={`inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs ${classes}`}>
      {withDot && <PresenceDot state={state === "warning" ? "degraded" : state} />}
      {label}
    </span>
  );
}

function DeviceVersionSummary({ device }) {
  const behind = device.latest_home_assistant_version && device.home_assistant_version !== device.latest_home_assistant_version;

  return (
    <div>
      <div className="font-medium">HA {device.home_assistant_version}</div>
      <div className="mt-1 text-xs text-slate-500">
        {behind ? (
          <span><span aria-hidden="true">↑</span> Update {device.latest_home_assistant_version}</span>
        ) : (
          "Current"
        )}
      </div>
    </div>
  );
}

function BackupSummary({ backup }) {
  if (!backup) {
    return (
      <div>
        <div className="font-medium text-slate-700">No backup yet</div>
        <div className="mt-1 text-xs text-slate-500">Not scheduled</div>
      </div>
    );
  }

  const succeeded = backup.status === "succeeded";
  return (
    <div>
      <div className="font-medium text-slate-800">
        {succeeded ? "Current" : "Needs attention"}
      </div>
      {succeeded ? (
        <div className="mt-1 text-xs text-slate-500">Last success: {formatDay(backup.last_success_at)}</div>
      ) : (
        <div className="mt-1 space-y-0.5 text-xs text-slate-500">
          <div className="text-amber-700">Last backup failed: {formatDay(backup.last_failure_at)}</div>
          <div>Last success: {formatDay(backup.last_success_at)}</div>
        </div>
      )}
    </div>
  );
}

function ConnectionSummary({ device }) {
  const connected = device.presence === "online";

  return (
    <div>
      <div className="flex items-center gap-2">
        <PresenceDot state={device.presence} />
        <span className="font-medium">{connected ? "Connected" : "Disconnected"}</span>
      </div>
      {!connected && (
        <div className="mt-1 text-xs text-slate-500">Last seen: {formatRelativeTime(device.last_seen_at)}</div>
      )}
    </div>
  );
}

function ArtifactSummary({ status, size }) {
  const stored = status === "stored";

  return (
    <div>
      <div className="font-medium text-slate-800">{stored ? "Stored" : "Not stored"}</div>
      <div className="mt-1 text-xs text-slate-500">
        {stored && size ? `${size} MB offsite copy` : "No offsite artifact"}
      </div>
    </div>
  );
}

function EmptySummary({ title, detail }) {
  return (
    <div>
      <div className="font-medium text-slate-700">{title}</div>
      <div className="mt-1 text-xs text-slate-500">{detail}</div>
    </div>
  );
}

function ReviewFact({ label, value, warning = false }) {
  return (
    <div className="rounded-md border border-slate-200 bg-slate-50 p-3">
      <div className="flex items-center gap-1 text-xs font-semibold uppercase tracking-normal text-slate-500">
        {label}
        {warning && <Warn inline />}
      </div>
      <div className="mt-1 text-sm font-medium text-slate-900">{value}</div>
    </div>
  );
}

function UpdateCell({ current, latest }) {
  const updateAvailable = latest && current !== latest;

  return (
    <div>
      <div className="font-medium">{current || "Unknown"}</div>
      <div className={`mt-1 text-xs ${updateAvailable ? "text-slate-600" : "text-slate-500"}`}>
        {updateAvailable ? (
          <span><span aria-hidden="true">↑</span> Update {latest}</span>
        ) : (
          "Current"
        )}
      </div>
    </div>
  );
}

function VersionDiff({ current, latest }) {
  const currentVersion = current || "Unknown";
  const latestVersion = latest || "Unknown";
  const behind = latest && current && latest !== current;

  return (
    <div>
      <div className="font-medium">{currentVersion}</div>
      <div className={`text-xs ${behind ? "text-amber-700" : "text-slate-500"}`}>
        {behind ? `Latest ${latestVersion}` : "Current"}
      </div>
    </div>
  );
}

function formatShortDate(value) {
  if (!value) return "None";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(date);
}

function formatDay(value) {
  if (!value) return "None";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
  }).format(date);
}

function formatRelativeTime(value) {
  if (!value) return "Unknown";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const diffMs = Date.now() - date.getTime();
  const diffHours = Math.max(1, Math.round(diffMs / (1000 * 60 * 60)));

  if (diffHours < 24) {
    return `${diffHours} ${diffHours === 1 ? "hour" : "hours"} ago`;
  }

  const diffDays = Math.round(diffHours / 24);
  return `${diffDays} ${diffDays === 1 ? "day" : "days"} ago`;
}

function VersionField({ label, current, latest, source }) {
  const status = latest && current && latest !== current ? "partial" : "backed";

  return (
    <div className="grid grid-cols-[180px_1fr] gap-3 border-b border-slate-200 py-2 text-sm last:border-b-0">
      <div className="flex items-center gap-1 font-medium text-slate-700">
        {label}
        <Coverage status={status} />
      </div>
      <div className="min-w-0">
        <VersionDiff current={current} latest={latest} />
        <div className="text-xs text-slate-500">{source}</div>
      </div>
    </div>
  );
}

function ToggleRow({ label, enabled, status, source }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-slate-200 py-3 last:border-b-0">
      <div>
        <div className="flex items-center gap-1 text-sm font-medium">
          {label}
          <Coverage status={status} />
        </div>
        <div className="text-xs text-slate-500">{source}</div>
      </div>
      <div className="flex items-center gap-3">
        <span className={`relative inline-flex h-6 w-11 rounded-full ${enabled ? "bg-slate-900" : "bg-slate-300"}`}>
          <span className={`mt-1 h-4 w-4 rounded-full bg-white transition ${enabled ? "ml-6" : "ml-1"}`} />
        </span>
      </div>
    </div>
  );
}

function Screen({ title, subtitle, children }) {
  return (
    <div className="mx-auto max-w-7xl">
      <header className="mb-5">
        <h1 className="text-2xl font-semibold tracking-normal">{title}</h1>
        <p className="mt-1 text-sm text-slate-600">{subtitle}</p>
      </header>
      <div className="space-y-4">{children}</div>
    </div>
  );
}

function Section({ title, children }) {
  return (
    <section className="rounded-md border border-slate-300 bg-white p-4">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-normal text-slate-700">
        {title}
      </h2>
      {children}
    </section>
  );
}

function InternalNoteSection({ title, children }) {
  return (
    <section className="rounded-md border border-dashed border-amber-300 bg-amber-50/40 p-4">
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <h2 className="text-sm font-semibold uppercase tracking-normal text-slate-700">
          {title}
        </h2>
        <span className="rounded border border-amber-300 bg-white px-2 py-0.5 text-xs text-amber-800">
          Mock-only internal note
        </span>
      </div>
      {children}
    </section>
  );
}

const haAppShellStatus = {
  onboarding: { tone: "neutral", label: "Not paired with HomeSignal" },
  healthy: { tone: "success", label: "Paired with HomeSignal cloud" },
  disconnected: { tone: "warning", label: "Disconnected from HomeSignal cloud" },
  outdated: { tone: "success", label: "Paired with HomeSignal cloud" },
};

function HaAppShell({ children }) {
  return (
    <div className="bg-white font-['Roboto',Arial,sans-serif] text-[#212121]">
      <div className="bg-white">{children}</div>
    </div>
  );
}

function HaAppNav({ activePage, onNavigate, variant }) {
  const items = [
    { key: "status", label: "Status" },
    { key: "pairing", label: "Pairing" },
    { key: "permissions", label: "Permissions" },
    { key: "advanced", label: "Advanced" },
  ];

  if (variant === "bottom") {
    return (
      <nav className="fixed bottom-0 left-0 right-0 z-40 border-t border-[#e0e0e0] bg-white pb-[env(safe-area-inset-bottom)] shadow-[0_-2px_8px_rgba(0,0,0,0.08)] sm:hidden" aria-label="HomeSignal Manager navigation">
        <div className="grid grid-cols-4">
          {items.map((item) => {
            const active = activePage === item.key;
            return (
              <button
                key={item.key}
                type="button"
                onClick={() => onNavigate(item.key)}
                className={`min-w-0 px-2 pb-3 pt-3 text-center text-xs font-medium ${
                  active ? "text-[#039dcc]" : "text-[#616161]"
                }`}
              >
                <span className={`mx-auto mb-1 block h-0.5 w-8 rounded-full ${active ? "bg-[#039dcc]" : "bg-transparent"}`} />
                {item.label}
              </button>
            );
          })}
        </div>
      </nav>
    );
  }

  return (
    <nav className="hidden border-b border-[#e0e0e0] bg-white px-5 sm:block sm:px-8" aria-label="HomeSignal Manager navigation">
      <div className="flex gap-7">
        {items.map((item) => {
          const active = activePage === item.key;
          return (
            <button
              key={item.key}
              type="button"
              onClick={() => onNavigate(item.key)}
              className={`relative -mb-px min-h-10 px-0 py-2.5 text-sm font-medium ${
                active ? "text-[#039dcc]" : "text-[#616161] hover:text-[#212121]"
              }`}
            >
              {item.label}
              {active && <span className="absolute bottom-0 left-0 right-0 h-0.5 rounded-t bg-[#039dcc]" />}
            </button>
          );
        })}
      </div>
    </nav>
  );
}

function HaCard({ title, subtitle, children, className = "" }) {
  return (
    <section className={`rounded-lg border border-[#e0e0e0] bg-white p-4 shadow-none ${className}`}>
      {title && (
        <div className="mb-3">
          <h2 className="text-sm font-semibold text-[#212121]">{title}</h2>
          {subtitle && <p className="mt-1 text-sm leading-5 text-[#616161]">{subtitle}</p>}
        </div>
      )}
      {children}
    </section>
  );
}

function HaButton({ children, variant = "primary", size = "md", className = "", ...props }) {
  const disabled = Boolean(props.disabled);
  const variantClass =
    disabled
      ? "cursor-not-allowed border border-[#e0e0e0] bg-[#eeeeee] text-[#9e9e9e] shadow-none"
      : variant === "secondary"
        ? "border border-transparent bg-white text-[#039dcc] shadow-none hover:bg-[#e1f5fe]"
        : variant === "warning"
        ? "border border-amber-300 bg-white text-amber-950 hover:bg-amber-50"
          : variant === "dark"
            ? "bg-slate-950 text-white hover:bg-slate-800"
            : variant === "danger"
              ? "bg-red-700 text-white hover:bg-red-800"
              : "bg-[#039dcc] text-white hover:bg-[#0288d1]";
  const sizeClass = size === "lg" ? "px-6 py-3 text-base" : "px-5 py-2.5 text-sm";

  return (
    <button type="button" className={`inline-flex items-center rounded-full font-medium shadow-none ${variantClass} ${sizeClass} ${className}`} {...props}>
      {children}
    </button>
  );
}

function HaStateDot({ tone, size = "md" }) {
  const color =
    tone === "success"
      ? "bg-emerald-500"
      : tone === "active"
        ? "bg-[#03a9f4]"
        : tone === "warning"
        ? "bg-amber-500"
        : "bg-slate-400";
  const sizeClass = size === "sm" ? "h-2.5 w-2.5" : "h-3 w-3";

  return <span className={`inline-block rounded-full ${sizeClass} ${color}`} />;
}

function TwoColumn({ children }) {
  return <div className="grid gap-4 lg:grid-cols-2">{children}</div>;
}

function MetricGrid({ children }) {
  return <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">{children}</div>;
}

function Metric({ label, value, status, source, onClick }) {
  const className = `rounded-md border border-slate-300 bg-white p-4 text-left ${
    onClick ? "transition hover:border-slate-500 hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-slate-900" : ""
  }`;
  const content = (
    <>
      <div className="text-2xl font-semibold">{value}</div>
      <div className="mt-1 flex items-center gap-1 text-sm text-slate-700">
        {label}
        <Coverage status={status} />
      </div>
      <div className="mt-1 text-xs text-slate-500">{source}</div>
    </>
  );

  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={className}>
        {content}
      </button>
    );
  }

  return (
    <div className={className}>
      {content}
    </div>
  );
}

function Field({ label, value, status, source }) {
  return (
    <div className="grid grid-cols-[180px_1fr] gap-3 border-b border-slate-200 py-2 text-sm last:border-b-0">
      <div className="flex items-center gap-1 font-medium text-slate-700">
        {label}
        <Coverage status={status} />
      </div>
      <div className="min-w-0">
        <div className="break-words text-slate-950">{value ?? "None"}</div>
        <div className="text-xs text-slate-500">{source}</div>
      </div>
    </div>
  );
}

function ExternalLinkIcon({ className = "ml-2 h-4 w-4" }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className={className} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 7h10v10" />
      <path d="M7 17 17 7" />
      <path d="M15 21H5a2 2 0 0 1-2-2V9" />
    </svg>
  );
}

function HomeSignalPortalLink() {
  return (
    <a
      href="https://app.homesignal.local"
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center font-medium text-[#039dcc] underline underline-offset-2 hover:text-[#0277bd]"
    >
      HomeSignal Portal
      <ExternalLinkIcon className="ml-1 h-3.5 w-3.5" />
    </a>
  );
}

function HomeSignalPortalActionLink() {
  return (
    <a
      href="https://app.homesignal.local"
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center self-center text-sm font-medium text-[#039dcc] underline underline-offset-2 hover:text-[#0277bd]"
    >
      Open HomeSignal portal
      <ExternalLinkIcon className="ml-1 h-3.5 w-3.5" />
    </a>
  );
}

function HomeSignalAccountActionLink() {
  return (
    <a
      href="https://app.homesignal.local/signup"
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center self-center rounded-full border border-[#039dcc] bg-white px-5 py-2.5 text-sm font-medium text-[#039dcc] hover:bg-[#e1f5fe] hover:text-[#0277bd]"
    >
      Create HomeSignal Account
      <ExternalLinkIcon className="ml-1 h-3.5 w-3.5" />
    </a>
  );
}

function Action({ label, status, source }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-slate-200 py-2 last:border-b-0">
      <div>
        <div className="flex items-center gap-1 text-sm font-medium">
          {label}
          <Coverage status={status} />
        </div>
        <div className="text-xs text-slate-500">{source}</div>
      </div>
    </div>
  );
}

function Step({ label, status, source }) {
  return (
    <div className="border-b border-slate-200 py-2 last:border-b-0">
      <div>
        <div className="flex items-center gap-1 text-sm font-medium">
          {label}
          <Coverage status={status} />
        </div>
        <div className="text-xs text-slate-500">{source}</div>
      </div>
    </div>
  );
}

function Table({ columns, rows }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b border-slate-300 text-left text-xs uppercase tracking-normal text-slate-500">
            {columns.map((column) => (
              <th key={column} className="px-2 py-2 font-semibold">
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr key={rowIndex} className="border-b border-slate-200 last:border-b-0">
              {row.map((cell, cellIndex) => (
                <td key={cellIndex} className="px-2 py-2 align-top">
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SimpleList({ items }) {
  return (
    <div className="divide-y divide-slate-200">
      {items.map((item) => (
        <div key={item.text} className="flex items-start justify-between gap-3 py-2 text-sm">
          <div>
            <div className="flex items-center gap-1">
              {item.text}
              <Coverage status={item.status} />
            </div>
            <div className="text-xs text-slate-500">{item.note}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

function CoverageText({ status, children }) {
  return (
    <span className="inline-flex items-center gap-1">
      {children}
      <Coverage status={status} compact />
    </span>
  );
}

function Coverage({ status, compact = false }) {
  if (status === "backed") {
    return null;
  }

  const label =
    status === "partial"
      ? "partial"
      : status === "conditional"
        ? "conditional"
        : status === "future"
          ? "future"
          : "missing";

  return (
    <span
      title={schema[status]}
      className={`inline-flex items-center text-xs text-amber-600 ${compact ? "" : ""}`}
    >
      <Warn inline />
      <span className="sr-only">{label}</span>
    </span>
  );
}

function Warn({ inline = false }) {
  return (
    <span aria-label="warning" className={inline ? "text-amber-600" : "text-amber-600"}>
      ⚠
    </span>
  );
}

function TextButton({ children, onClick }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-md border border-slate-300 px-2 py-1 text-xs text-slate-800 hover:bg-slate-100"
    >
      {children}
    </button>
  );
}
