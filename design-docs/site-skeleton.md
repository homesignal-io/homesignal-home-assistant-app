import React, { useState } from "react";

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

const nav = [
  "Dashboard",
  "Accounts",
  "Customers",
  "Sites",
  "Enrollment",
  "Devices",
  "Backups",
  "Updates",
  "Diagnostics",
  "Alerts",
  "Audit",
  "Admin",
  "HA App",
  "Schema Coverage",
];

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
    display_name: "Smith Residence",
    email: "owner@example.com",
    phone: "(555) 010-2214",
    notes: "Primary managed residence.",
    status: "active",
    created_at: "2026-05-01T15:22:00Z",
    updated_at: "2026-05-13T18:10:00Z",
    archived_at: null,
  },
  {
    id: "cust_102",
    account_id: "acct_123",
    display_name: "Lee Residence",
    email: "lee@example.com",
    phone: null,
    notes: "Backup testing site.",
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
    account_id: "acct_123",
    customer_record_id: "cust_101",
    status: "active",
    service_address: "14 Maple Lane, Raleigh, NC",
    created_at: "2026-05-01T15:24:00Z",
    updated_at: "2026-05-13T18:20:00Z",
    archived_at: null,
    buildings: [{ id: "bldg_1", name: "Main House" }],
    zones: [{ id: "zone_1", building_id: "bldg_1", name: "Whole Home" }],
  },
  {
    id: "site_124",
    name: "Lee Residence",
    account_id: "acct_123",
    customer_record_id: "cust_102",
    status: "active",
    service_address: "99 Lake Road, Cary, NC",
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
    supervisor_version: "2026.05.0",
    ha_app_version: "0.1.4",
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
    supervisor_version: "2026.05.0",
    ha_app_version: "0.1.3",
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
      enabled_event_families: ["agent_alarm"],
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
  "Claim invite created for Smith Residence",
  "Device dev_123 claim finalized",
  "Backup trigger issued for dev_123",
  "Update rollout intent changed for rollout_456",
  "Credential rotation completed for dev_123",
];

const haAppState = {
  local_state: "CLAIMED",
  claim_invite_code: null,
  device_id: "dev_123",
  thing_name: "dev_123",
  config_path: "/config/device.json",
  cert_path: "/config/iot/device.pem",
  private_key_path: "/config/iot/private.key",
  cloud_connection: "connected",
  last_policy_version: "ppv_123",
  ha_app_version: "0.1.4",
  desired_ha_app_version: "0.1.4",
  last_error_excerpt: "No recent errors.",
};

export default function HomeSignalProductSkeleton() {
  const [page, setPage] = useState("Dashboard");
  const [selectedSiteId, setSelectedSiteId] = useState("site_123");
  const [selectedDeviceId, setSelectedDeviceId] = useState("dev_123");

  const selectedSite = sites.find((site) => site.id === selectedSiteId) || sites[0];
  const selectedDevice =
    devices.find((device) => device.id === selectedDeviceId) || devices[0];

  return (
    <div className="min-h-screen bg-slate-100 text-slate-950">
      <div className="flex min-h-screen">
        <aside className="w-72 shrink-0 border-r border-slate-300 bg-white p-4">
          <div className="mb-5">
            <div className="text-lg font-semibold">HomeSignal</div>
            <div className="text-xs text-slate-500">Integrator console skeleton</div>
          </div>

          <nav className="space-y-1">
            {nav.map((item) => (
              <button
                key={item}
                onClick={() => setPage(item)}
                className={`w-full rounded-md px-3 py-2 text-left text-sm ${
                  page === item
                    ? "bg-slate-900 text-white"
                    : "text-slate-700 hover:bg-slate-100"
                }`}
              >
                {item}
              </button>
            ))}
          </nav>

          <div className="mt-6 border-t border-slate-200 pt-4 text-xs text-slate-500">
            <div className="mb-1 font-medium text-slate-700">Legend</div>
            <div><Warn inline /> Future, conditional, or intentionally not v0-backed</div>
            <div><Badge label="v0" /> Backed v0 surface</div>
          </div>
        </aside>

        <main className="flex-1 p-6">
          {page === "Dashboard" && (
            <Dashboard
              setPage={setPage}
              setSelectedSiteId={setSelectedSiteId}
              setSelectedDeviceId={setSelectedDeviceId}
            />
          )}
          {page === "Accounts" && <Accounts />}
          {page === "Customers" && <Customers />}
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
            <DeviceDetail
              device={selectedDevice}
              site={sites.find((s) => s.id === selectedDevice.site_id)}
            />
          )}
          {page === "Backups" && <Backups />}
          {page === "Updates" && <Updates />}
          {page === "Diagnostics" && <Diagnostics />}
          {page === "Alerts" && <Alerts />}
          {page === "Audit" && <Audit />}
          {page === "Admin" && <Admin />}
          {page === "HA App" && <HaApp />}
          {page === "Schema Coverage" && <SchemaCoverage />}
        </main>
      </div>
    </div>
  );
}

function Dashboard({ setPage, setSelectedSiteId, setSelectedDeviceId }) {
  const attention = [
    {
      label: "Lee Residence backup failed",
      status: "backed",
      detail: "Backup Service + command result + backup status",
      action: () => {
        setSelectedSiteId("site_124");
        setSelectedDeviceId("dev_124");
        setPage("Backups");
      },
    },
    {
      label: "Device dev_124 is degraded",
      status: "backed",
      detail: "device_presence + device_latest_state",
      action: () => {
        setSelectedDeviceId("dev_124");
        setPage("Devices");
      },
    },
    {
      label: "Possible future product alert",
      status: "future",
      detail: "Alerting is future/productized later",
      action: () => setPage("Alerts"),
    },
  ];

  return (
    <Screen title="Dashboard" subtitle="Operational view for an integrator account.">
      <MetricGrid>
        <Metric label="Active sites" value="2" status="backed" source="sites.status" />
        <Metric label="Claimed devices" value="2" status="backed" source="devices.status" />
        <Metric label="Needs attention" value="2" status="backed" source="API Facade issue projection" />
        <Metric label="Product alerts" value="0" status="backed" source="alerts / alert_events" />
      </MetricGrid>

      <Section title="Attention queue">
        <Table
          columns={["Item", "Backed by", "Action"]}
          rows={attention.map((item) => [
            <CoverageText status={item.status}>{item.label}</CoverageText>,
            item.detail,
            <TextButton onClick={item.action}>Open</TextButton>,
          ])}
        />
      </Section>

      <Section title="Recent activity">
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

function Customers() {
  return (
    <Screen title="Customers" subtitle="Account-scoped customer records, not customer login accounts.">
      <Section title="Customer records">
        <Table
          columns={["Customer", "Email", "Phone", "Status", "Schema"]}
          rows={customers.map((customer) => [
            customer.display_name,
            customer.email || "None",
            customer.phone || "None",
            customer.status,
            <CoverageText status="backed">customers</CoverageText>,
          ])}
        />
      </Section>

      <Section title="Customer portal checks">
        <Field label="Customer login" value="Not v0" status="missing" source="account-site-service.md" />
        <Field label="Neighbor/caregiver sharing" value="Out of scope v0" status="future" source="account-site-service.md" />
      </Section>
    </Screen>
  );
}

function Sites({ selectedSiteId, setSelectedSiteId, setSelectedDeviceId, setPage }) {
  return (
    <Screen title="Sites" subtitle="Business hierarchy: account -> customer -> site -> building -> zone -> device.">
      <Section title="Sites list">
        <Table
          columns={["Site", "Customer", "Address", "Status", "Device", "Open"]}
          rows={sites.map((site) => {
            const customer = customers.find((item) => item.id === site.customer_record_id);
            const device = devices.find((item) => item.site_id === site.id);
            return [
              <CoverageText status="backed">{site.name}</CoverageText>,
              customer?.display_name || "None",
              site.service_address,
              site.status,
              device ? device.id : "None",
              <TextButton
                onClick={() => {
                  setSelectedSiteId(site.id);
                  if (device) setSelectedDeviceId(device.id);
                  setPage("Devices");
                }}
              >
                Open
              </TextButton>,
            ];
          })}
        />
      </Section>

      <Section title="Selected site model">
        <Field label="Buildings" value="First-class model; may be collapsed in UI" status="backed" source="buildings" />
        <Field label="Zones" value="Device placement target" status="backed" source="zones" />
        <Field label="Topology snapshot" value="Not v0" status="future" source="topology_snapshots" />
        <Field label="Browser geolocation" value="Not stored" status="missing" source="explicitly excluded" />
      </Section>
    </Screen>
  );
}

function Enrollment({ selectedSite }) {
  return (
    <Screen title="Enrollment" subtitle="Pairing and claim flow rooted in documented enrollment contracts.">
      <TwoColumn>
        <Section title="Portal claim flow">
          <Step status="backed" label="Create site claim invite" source="device_claim_invites" />
          <Step status="backed" label="Email or share GUID claim code" source="device_claim_invite_email_deliveries" />
          <Step status="backed" label="App verifies invite and displays details" source="device_claim_verifications" />
          <Step status="backed" label="Local user confirms details" source="claim_verification confirm" />
          <Step status="backed" label="Finalize claim and persist credential metadata" source="devices + device_credentials" />
        </Section>

        <Section title="Claim context mock">
          <Field label="Target site" value={selectedSite.name} status="backed" source="sites" />
          <Field label="Claim invite" value="GUID-style code, hashed at rest" status="backed" source="device_claim_invites" />
          <Field label="HA instance UUID" value="ha_8f1d..." status="backed" source="recognition_signals" />
          <Field label="Machine ID" value="2f8f8c..." status="backed" source="recognition_signals" />
          <Field label="Same-account repair" value="Allowed when authorized" status="backed" source="device-lifecycle.md" />
          <Field label="History transfer" value="Not v0" status="future" source="explicit future product feature" />
        </Section>
      </TwoColumn>
    </Screen>
  );
}

function DeviceDetail({ device, site }) {
  return (
    <Screen title="Device detail" subtitle={`${device.id} at ${site?.name || "Unknown site"}`}>
      <TwoColumn>
        <Section title="Identity and presence">
          <Field label="Device ID" value={device.id} status="backed" source="devices.device_id" />
          <Field label="AWS IoT Thing name" value={device.thing_name} status="backed" source="devices + AWS IoT" />
          <Field label="Site" value={site?.name} status="backed" source="sites" />
          <Field label="Zone" value={device.zone_id} status="backed" source="zones" />
          <Field label="Presence" value={device.presence} status="backed" source="device_presence" />
          <Field label="Last seen" value={device.last_seen_at} status="backed" source="device_presence.last_seen_at" />
          <Field label="Credential status" value={device.credential_status} status="backed" source="device_credentials" />
        </Section>

        <Section title="Latest state">
          <Field label="Home Assistant version" value={device.home_assistant_version} status="backed" source="device_latest_state" />
          <Field label="Supervisor version" value={device.supervisor_version} status="backed" source="device_latest_state" />
          <Field label="App version" value={device.ha_app_version} status="backed" source="device_latest_state" />
          <Field label="Storage" value={device.storage_status} status="backed" source="device_latest_state" />
          <Field label="Topology browser" value="Not exposed v0" status="future" source="topology_snapshots" />
          <Field label="Raw HA config viewer" value="Not allowed" status="missing" source="diagnostics boundary" />
        </Section>
      </TwoColumn>

      <TwoColumn>
        <Section title="Edge desired/reported state">
          <Field label="Desired publish policy" value={edgeState.desired.publish_policy.version} status="backed" source="homesignal_edge.publish_policy" />
          <Field label="Reported publish policy" value={edgeState.reported.publish_policy.active_version} status="backed" source="homesignal_edge.publish_policy" />
          <Field label="Desired app version" value={edgeState.desired.update.desired_version} status="backed" source="homesignal_edge.update" />
          <Field label="Reported app version" value={edgeState.reported.update.current_version} status="backed" source="homesignal_edge.update" />
          <Field label="Shadow full document history" value="Not stored by default" status="missing" source="edge-state-adapter.md" />
        </Section>

        <Section title="Device actions">
          <Action label="Refresh publish policy" status="backed" source="commands.refresh_publish_policy" />
          <Action label="Trigger backup" status="backed" source="commands.trigger_backup + backups" />
          <Action label="Request bounded diagnostics" status="conditional" source="Diagnostics flow must explicitly enable" />
          <Action label="Restart app" status="missing" source="No v0 command authority" />
          <Action label="Release device" status="backed" source="device lifecycle + audit_events" />
        </Section>
      </TwoColumn>
    </Screen>
  );
}

function Backups() {
  return (
    <Screen title="Backups" subtitle="V0 logical service with status, trigger, and optional offsite backup artifacts.">
      <Section title="Backup status">
        <Table
          columns={["Site", "Device", "Status", "Last success", "Artifact", "Schema"]}
          rows={backups.map((backup) => {
            const site = sites.find((item) => item.id === backup.site_id);
            return [
              site?.name || backup.site_id,
              backup.device_id,
              backup.status,
              backup.last_success_at || "None",
              backup.artifact_status,
              <CoverageText status="backed">backups</CoverageText>,
            ];
          })}
        />
      </Section>

      <Section title="Backup product boundaries">
        <Field label="Trigger backup" value="Command-class bounded attempt" status="backed" source="commands + backups" />
        <Field label="Offsite backup bytes" value="Allowed v0 under Backup Service" status="backed" source="artifact-upload-broker.md" />
        <Field label="Restore backup" value="Future" status="future" source="device-broker.md later list" />
      </Section>
    </Screen>
  );
}

function Updates() {
  return (
    <Screen title="Updates" subtitle="CI/CD publishes releases; shadow desired state tracks rollout intent and convergence.">
      <Section title="Release channels and rollouts">
        <Table
          columns={["Version", "Channel", "Rollout", "Status", "Published via"]}
          rows={releases.map((release) => [
            <CoverageText status="backed">{release.version}</CoverageText>,
            release.channel,
            release.rollout_id,
            release.status,
            release.published_via,
          ])}
        />
      </Section>

      <TwoColumn>
        <Section title="Shadow update projection">
          <Field label="Desired version" value={edgeState.desired.update.desired_version} status="backed" source="homesignal_edge.update.desired_version" />
          <Field label="Channel" value={edgeState.desired.update.channel} status="backed" source="homesignal_edge.update.channel" />
          <Field label="Rollout ID" value={edgeState.desired.update.rollout_id} status="backed" source="homesignal_edge.update.rollout_id" />
          <Field label="Reported version" value={edgeState.reported.update.current_version} status="backed" source="homesignal_edge.update.current_version" />
        </Section>

        <Section title="Update guardrails">
          <Field label="Binary install over IoT" value="Not v0" status="missing" source="update-architecture.md" />
          <Field label="Artifact/download URL in shadow" value="Not allowed" status="missing" source="edge-state-adapter.md" />
          <Field label="Local-supervisor apply command" value="Future only" status="future" source="command-lifecycle.md" />
          <Field label="Unsupported app versions visible in UI" value="Required" status="backed" source="migration-strategy.md" />
        </Section>
      </TwoColumn>
    </Screen>
  );
}

function Diagnostics() {
  return (
    <Screen title="Diagnostics" subtitle="Bounded support/debug capture, not arbitrary host access.">
      <TwoColumn>
        <Section title="Diagnostic capabilities">
          <Action label="Collect app status" status="backed" source="local-cloud-trust-boundaries.md" />
          <Action label="Collect connectivity check" status="backed" source="local-cloud-trust-boundaries.md" />
          <Action label="Collect recent error excerpt" status="backed" source="5 KB bounded excerpt" />
          <Action label="Request debug bundle" status="conditional" source="Diagnostics/Debug flow must explicitly enable artifact upload" />
          <Action label="Collect raw HA config snapshot" status="missing" source="explicitly not v0 diagnostics" />
        </Section>

        <Section title="Artifacts">
          <Field label="Diagnostic bundle metadata" value="Defined as table family" status="conditional" source="diagnostic_bundles" />
          <Field label="Debug bundle upload" value="Approved flow only" status="conditional" source="artifact-upload-broker.md" />
          <Field label="Unsolicited log upload" value="Not allowed" status="missing" source="artifact-upload-broker.md" />
        </Section>
      </TwoColumn>
    </Screen>
  );
}

function Alerts() {
  return (
    <Screen title="Alerts" subtitle="Product alerting and email delivery are v0 surfaces owned by Alerting and Notification Service.">
      <Section title="Alert lifecycle">
        <Field label="Backup failed candidate" value="Backed by backup status/event" status="backed" source="backups + device events" />
        <Field label="Device stale/degraded" value="Backed by presence/latest state" status="backed" source="device_presence" />
        <Field label="Product alert lifecycle" value="V0" status="backed" source="alerts / alert_events" />
        <Field label="Customer notification delivery" value="V0 email via provider adapter" status="backed" source="Notification Service + Resend adapter" />
      </Section>
    </Screen>
  );
}

function Audit() {
  return (
    <Screen title="Audit / activity" subtitle="Sensitive authority history is separate from operational logs.">
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
    <Screen title="Admin / operations" subtitle="Internal surfaces; many are intentionally future.">
      <TwoColumn>
        <Section title="Policy and budgets">
          <Field label="Publish policy catalog" value="Resolved per-device policy records" status="backed" source="device_desired_state + edge projection" />
          <Field label="Plan tier editor" value="Admin-defined, needs concrete UI spec" status="partial" source="publish policy discussion" />
          <Field label="Live event stream pricing" value="Future" status="future" source="not v0" />
        </Section>

        <Section title="Platform operations">
          <Field label="Platform health findings" value="Future internal" status="future" source="platform_health_findings" />
          <Field label="Runaway device messaging monitor" value="Future internal" status="future" source="platform-health-monitoring-service.md" />
          <Field label="Service credential rotation" value="Neon/Postgres launch requirement" status="backed" source="secrets-and-config.md + deployment-readiness-matrix.md" />
        </Section>
      </TwoColumn>
    </Screen>
  );
}

function HaApp() {
  return (
    <Screen title="Home Assistant app mock" subtitle="Local app UI inside Home Assistant, not the SaaS portal.">
      <TwoColumn>
        <Section title="Local claim status">
          <Field label="Local state" value={haAppState.local_state} status="backed" source="/config/device.json" />
          <Field label="Device ID" value={haAppState.device_id} status="backed" source="/config/device.json" />
          <Field label="Thing name" value={haAppState.thing_name} status="backed" source="/config/device.json" />
          <Field label="Certificate path" value={haAppState.cert_path} status="backed" source="/config/iot/*" />
          <Field label="Private key path" value="Stored locally, never shown" status="backed" source="/config/iot/*" />
        </Section>

        <Section title="Local operational status">
          <Field label="Cloud connection" value={haAppState.cloud_connection} status="backed" source="local status + telemetry" />
          <Field label="Active publish policy" value={haAppState.last_policy_version} status="backed" source="homesignal_edge.publish_policy" />
          <Field label="Installed app version" value={haAppState.ha_app_version} status="backed" source="local observation + reported.update" />
          <Field label="Desired app version" value={haAppState.desired_ha_app_version} status="backed" source="homesignal_edge.update" />
          <Field label="Support debug mode toggle" value="Cloud-authorized only" status="conditional" source="observability.md" />
          <Field label="Manual raw file upload" value="Not allowed" status="missing" source="artifact upload boundary" />
        </Section>
      </TwoColumn>

      <Section title="Unclaimed app screen">
        <Field label="Claim invite code" value="Entered locally when unclaimed" status="backed" source="device_claim_invites" />
        <Field label="Verification token" value="Stored secret file; never shown" status="backed" source="enrollment-claiming-contract.md" />
        <Field label="Recognition signals" value="HA UUID, machine ID, versions, hostname" status="backed" source="recognition_signals" />
      </Section>
    </Screen>
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

function TwoColumn({ children }) {
  return <div className="grid gap-4 lg:grid-cols-2">{children}</div>;
}

function MetricGrid({ children }) {
  return <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">{children}</div>;
}

function Metric({ label, value, status, source }) {
  return (
    <div className="rounded-md border border-slate-300 bg-white p-4">
      <div className="flex items-center justify-between gap-2">
        <div className="text-2xl font-semibold">{value}</div>
        <Coverage status={status} />
      </div>
      <div className="mt-1 text-sm text-slate-700">{label}</div>
      <div className="mt-1 text-xs text-slate-500">{source}</div>
    </div>
  );
}

function Field({ label, value, status, source }) {
  return (
    <div className="grid grid-cols-[180px_1fr_auto] gap-3 border-b border-slate-200 py-2 text-sm last:border-b-0">
      <div className="font-medium text-slate-700">{label}</div>
      <div className="min-w-0">
        <div className="break-words text-slate-950">{value ?? "None"}</div>
        <div className="text-xs text-slate-500">{source}</div>
      </div>
      <Coverage status={status} />
    </div>
  );
}

function Action({ label, status, source }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-slate-200 py-2 last:border-b-0">
      <div>
        <div className="text-sm font-medium">{label}</div>
        <div className="text-xs text-slate-500">{source}</div>
      </div>
      <Coverage status={status} />
    </div>
  );
}

function Step({ label, status, source }) {
  return (
    <div className="flex items-start gap-3 border-b border-slate-200 py-2 last:border-b-0">
      <Coverage status={status} />
      <div>
        <div className="text-sm font-medium">{label}</div>
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
            <div>{item.text}</div>
            <div className="text-xs text-slate-500">{item.note}</div>
          </div>
          <Coverage status={item.status} />
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
    return compact ? (
      <span className="text-xs text-emerald-700">v0</span>
    ) : (
      <Badge label="v0" />
    );
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
      className={`inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs ${
        compact ? "border-transparent px-0 py-0" : "border-amber-300 bg-amber-50 text-amber-900"
      }`}
    >
      <Warn inline /> {!compact && label}
    </span>
  );
}

function Badge({ label }) {
  return (
    <span className="inline-flex rounded-md border border-emerald-300 bg-emerald-50 px-2 py-1 text-xs text-emerald-800">
      {label}
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
