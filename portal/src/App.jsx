import { useEffect, useMemo, useState } from "react";
import { getActivity, getDashboard, getDevices } from "./api.js";
import { primaryActionLabel, routeForPrimaryAction } from "./actions.js";

const pages = ["Dashboard", "Devices", "Alerts", "Activity", "Enrollment", "Settings"];
const publicPages = new Set([...pages, "Device Detail"]);

function readRoute() {
  if (typeof window === "undefined") {
    return { page: "Dashboard", deviceId: "", focus: "" };
  }

  const params = new URLSearchParams(window.location.hash.replace(/^#/, ""));
  const page = params.get("page") || "Dashboard";

  return {
    page: publicPages.has(page) ? page : "Dashboard",
    deviceId: params.get("device") || "",
    focus: params.get("focus") || "",
  };
}

function writeRoute(route) {
  const params = new URLSearchParams();
  params.set("page", route.page || "Dashboard");
  if (route.deviceId) params.set("device", route.deviceId);
  if (route.focus) params.set("focus", route.focus);
  window.history.pushState(null, "", `#${params.toString()}`);
}

function usePortalData() {
  const [state, setState] = useState({
    status: "loading",
    dashboard: null,
    devices: null,
    activity: null,
    error: null,
  });

  useEffect(() => {
    let active = true;

    Promise.all([getDashboard(), getDevices(), getActivity()])
      .then(([dashboard, devices, activity]) => {
        if (!active) return;
        setState({ status: "ready", dashboard, devices, activity, error: null });
      })
      .catch((error) => {
        if (!active) return;
        setState((current) => ({ ...current, status: "error", error }));
      });

    return () => {
      active = false;
    };
  }, []);

  return state;
}

export default function App() {
  const data = usePortalData();
  const [route, setRoute] = useState(readRoute);

  useEffect(() => {
    const onHashChange = () => setRoute(readRoute());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  const deviceRows = data.devices?.devices || data.dashboard?.managed_home_assistants || [];
  const activityRows = data.activity?.activity || data.dashboard?.activity || [];
  const selectedRow =
    deviceRows.find((row) => row.device.device_id === route.deviceId) || deviceRows[0] || null;

  const navigate = (nextRoute) => {
    const normalized = typeof nextRoute === "string" ? { page: nextRoute } : nextRoute;
    writeRoute(normalized);
    setRoute({ page: normalized.page || "Dashboard", deviceId: normalized.deviceId || "", focus: normalized.focus || "" });
  };

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand-lockup">
          <div className="brand-mark">HS</div>
          <div>
            <div className="brand-title">HomeSignal</div>
            <div className="brand-subtitle">Integrator portal</div>
          </div>
        </div>
        <nav className="nav-list" aria-label="Primary">
          {pages.map((page) => (
            <button
              key={page}
              type="button"
              className={`nav-item ${route.page === page ? "active" : ""}`}
              onClick={() => navigate(page)}
            >
              {page}
            </button>
          ))}
        </nav>
      </aside>

      <main className="main">
        {data.status === "loading" && <LoadingState />}
        {data.status === "error" && <ErrorState error={data.error} />}
        {data.status === "ready" && (
          <>
            {route.page === "Dashboard" && (
              <Dashboard
                dashboard={data.dashboard}
                deviceRows={deviceRows}
                activityRows={activityRows}
                onNavigate={navigate}
              />
            )}
            {route.page === "Devices" && (
              <DevicesPage deviceRows={deviceRows} onNavigate={navigate} />
            )}
            {route.page === "Device Detail" && (
              <DeviceDetailPage row={selectedRow} focus={route.focus} onNavigate={navigate} />
            )}
            {route.page === "Alerts" && <AlertsPage deviceRows={deviceRows} />}
            {route.page === "Activity" && <ActivityPage activityRows={activityRows} />}
            {route.page === "Enrollment" && <EnrollmentPage deviceRows={deviceRows} />}
            {route.page === "Settings" && <SettingsPage dashboard={data.dashboard} />}
          </>
        )}
      </main>
    </div>
  );
}

function LoadingState() {
  return (
    <section className="page narrow">
      <div className="skeleton-line wide" />
      <div className="skeleton-line" />
      <div className="skeleton-panel" />
    </section>
  );
}

function ErrorState({ error }) {
  return (
    <section className="page narrow">
      <PageHeader title="Portal unavailable" subtitle="The read models could not be loaded." />
      <div className="notice danger">{error?.message || "Unknown error"}</div>
    </section>
  );
}

function Dashboard({ dashboard, deviceRows, activityRows, onNavigate }) {
  const summary = dashboard.summary;
  const needsReviewRows = deviceRows.filter((row) => row.issues.length > 0);

  return (
    <section className="page">
      <PageHeader
        eyebrow="Overview"
        title={dashboard.dashboard_state === "action_required" ? "Action Required" : "All Managed Sites Healthy"}
        subtitle={`${summary.managed_sites} sites and ${summary.managed_devices} Home Assistant instances under management.`}
        action={<button className="primary-button" type="button" onClick={() => onNavigate("Enrollment")}>Pair Home Assistant</button>}
      />

      <MetricStrip summary={summary} />

      <section className="section-band">
        <SectionHeader
          title="Managed Home Assistants"
          detail={needsReviewRows.length > 0 ? `${needsReviewRows.length} need review` : "No open issues"}
        />
        <div className="row-stack">
          {deviceRows.map((row) => (
            <ManagedHomeAssistantRow key={row.device.device_id} row={row} onNavigate={onNavigate} />
          ))}
        </div>
      </section>

      <section className="section-band two-column">
        <div>
          <SectionHeader title="Open Issues" detail={`${summary.open_issue_count} total`} />
          <IssueList rows={deviceRows} />
        </div>
        <div>
          <SectionHeader title="Recent Activity" detail="Latest customer-visible events" />
          <ActivityList rows={activityRows.slice(0, 4)} compact />
        </div>
      </section>
    </section>
  );
}

function MetricStrip({ summary }) {
  const metrics = [
    ["Online", `${summary.online_devices}/${summary.managed_devices}`, "Managed instances reporting"],
    ["Sites Needing Review", summary.sites_needing_review, "Presence, backup, or update attention"],
    ["Backup Issues", summary.backup_issue_count, "Failed or overdue backups"],
    ["App Updates", summary.app_update_attention_count, "Attention after grace period"],
    ["Email Alerts", titleCase(summary.email_alerts_status), "Recipient configuration"],
  ];

  return (
    <div className="metric-strip">
      {metrics.map(([label, value, detail]) => (
        <div className="metric" key={label}>
          <div className="metric-label">{label}</div>
          <div className="metric-value">{value}</div>
          <div className="metric-detail">{detail}</div>
        </div>
      ))}
    </div>
  );
}

function DevicesPage({ deviceRows, onNavigate }) {
  const [filter, setFilter] = useState("all");
  const visibleRows = useMemo(() => {
    if (filter === "needs_review") return deviceRows.filter((row) => row.issues.length > 0);
    if (filter === "online") return deviceRows.filter((row) => row.device.presence === "online");
    if (filter === "disconnected") return deviceRows.filter((row) => row.device.presence === "disconnected");
    return deviceRows;
  }, [deviceRows, filter]);

  return (
    <section className="page">
      <PageHeader
        eyebrow="Fleet"
        title="Managed Home Assistants"
        subtitle="Review connection, versions, backups, storage, and update attention across every managed site."
      />
      <SegmentedControl
        value={filter}
        onChange={setFilter}
        options={[
          ["all", "All"],
          ["needs_review", "Needs review"],
          ["online", "Online"],
          ["disconnected", "Disconnected"],
        ]}
      />
      <div className="row-stack">
        {visibleRows.length === 0 ? (
          <EmptyState title="No devices match this filter" detail="Change the fleet filter to show other managed instances." />
        ) : (
          visibleRows.map((row) => (
            <ManagedHomeAssistantRow key={row.device.device_id} row={row} onNavigate={onNavigate} expanded />
          ))
        )}
      </div>
    </section>
  );
}

function ManagedHomeAssistantRow({ row, onNavigate, expanded = false }) {
  const primaryIssue = row.issues[0];
  const actionRoute = routeForPrimaryAction(row.primary_action, row);

  return (
    <article className="device-row">
      <div className="device-row-main">
        <div className="site-heading">
          <SiteIcon category={row.site_category} />
          <div>
            <button
              type="button"
              className="link-button strong"
              onClick={() => onNavigate({ page: "Device Detail", deviceId: row.device.device_id })}
            >
              {row.site_name}
            </button>
            <div className="muted">{row.customer_display_name} / {row.compact_location}</div>
          </div>
        </div>

        <StatusSummary row={row} />
      </div>

      <div className="device-row-grid">
        <VersionCell label="Home Assistant" current={row.home_assistant.installed_version} latest={row.home_assistant.latest_version} />
        <VersionCell label="HomeSignal app" current={row.ha_app.installed_version} latest={row.ha_app.desired_version} status={row.ha_app.update_status} />
        <BackupCell backup={row.backup} />
        <StorageCell storage={row.storage} />
      </div>

      <div className="device-row-footer">
        <div className="issue-inline">
          {primaryIssue ? (
            row.issues.map((issue) => <IssuePill key={issue.issue_code} issue={issue} />)
          ) : (
            <span className="quiet">No open issues</span>
          )}
        </div>
        <button className="secondary-button" type="button" onClick={() => onNavigate(actionRoute)}>
          {primaryActionLabel(row.primary_action)}
        </button>
      </div>

      {expanded && primaryIssue && (
        <div className="issue-detail-list">
          {row.issues.map((issue) => (
            <div className="issue-detail" key={`${row.device.device_id}-${issue.issue_code}`}>
              <span className={`severity-dot ${issue.severity}`} />
              <div>
                <div className="issue-title">{issue.label}</div>
                <div className="muted">{issue.detail}</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </article>
  );
}

function DeviceDetailPage({ row, focus, onNavigate }) {
  if (!row) {
    return (
      <section className="page narrow">
        <PageHeader title="Device not found" subtitle="The selected Home Assistant instance is not present in the read model." />
        <button className="secondary-button" type="button" onClick={() => onNavigate("Devices")}>Back to devices</button>
      </section>
    );
  }

  return (
    <section className="page">
      <PageHeader
        eyebrow="Device detail"
        title={row.site_name}
        subtitle={`${row.customer_display_name} / ${row.compact_location}`}
        action={<button className="secondary-button" type="button" onClick={() => onNavigate("Devices")}>Back to devices</button>}
      />

      {focus && (
        <div className="notice info">{focusMessage(focus)}</div>
      )}

      <section className="section-band two-column">
        <div>
          <SectionHeader title="Reported State" detail="Latest platform read model" />
          <dl className="fact-list">
            <Fact label="Connection" value={presenceLabel(row.device.presence)} detail={`Last seen ${formatDateTime(row.device.last_seen_at)}`} />
            <Fact label="Home Assistant" value={row.home_assistant.installed_version} detail={latestDetail(row.home_assistant.latest_version)} />
            <Fact label="Supervisor" value={row.supervisor.installed_version || "Unknown"} detail={latestDetail(row.supervisor.latest_version)} />
            <Fact label="HomeSignal app" value={row.ha_app.installed_version || "Unknown"} detail={`Desired ${row.ha_app.desired_version || "unknown"} / ${titleCase(row.ha_app.update_status || "unknown")}`} />
            <Fact label="Storage" value={titleCase(row.storage.status)} detail={row.storage.detail} />
          </dl>
        </div>

        <div>
          <SectionHeader title="Backup" detail={backupLabel(row.backup)} />
          <dl className="fact-list">
            <Fact label="Status" value={titleCase(row.backup.status || "unknown")} />
            <Fact label="Last success" value={formatDateTime(row.backup.last_success_at)} />
            <Fact label="Last failure" value={formatDateTime(row.backup.last_failure_at)} />
          </dl>
        </div>
      </section>

      <section className="section-band">
        <SectionHeader title="Issues" detail={`${row.issues.length} open`} />
        {row.issues.length === 0 ? (
          <EmptyState title="No issues on this device" detail="Presence, backups, storage, and app update posture are clear." />
        ) : (
          <div className="issue-list">
            {row.issues.map((issue) => (
              <IssueCard issue={issue} key={issue.issue_code} />
            ))}
          </div>
        )}
      </section>

      <details className="advanced-details">
        <summary>Advanced identifiers</summary>
        <dl className="fact-list compact">
          <Fact label="Device ID" value={row.device.device_id} />
          <Fact label="Thing name" value={row.device.thing_name} />
          <Fact label="Site ID" value={row.site_id} />
        </dl>
      </details>
    </section>
  );
}

function AlertsPage({ deviceRows }) {
  const issues = deviceRows.flatMap((row) => row.issues.map((issue) => ({ ...issue, site_name: row.site_name })));
  const [email, setEmail] = useState("");
  const [recipients, setRecipients] = useState([]);

  const addRecipient = (event) => {
    event.preventDefault();
    const trimmed = email.trim();
    if (!trimmed) return;
    setRecipients((current) => [
      ...current,
      {
        id: `recipient_${current.length + 1}`,
        email: trimmed,
        status: "pending",
        subscriptions: {
          device_disconnected: true,
          backup_failed_or_overdue: true,
          app_update_attention: true,
        },
      },
    ]);
    setEmail("");
  };

  const toggleSubscription = (recipientId, key) => {
    setRecipients((current) =>
      current.map((recipient) =>
        recipient.id === recipientId
          ? {
              ...recipient,
              subscriptions: {
                ...recipient.subscriptions,
                [key]: !recipient.subscriptions[key],
              },
            }
          : recipient
      )
    );
  };

  return (
    <section className="page">
      <PageHeader
        eyebrow="Alerts"
        title="Alert Center"
        subtitle="Active issues and the email recipients who should be notified."
      />

      <section className="section-band two-column">
        <div>
          <SectionHeader title="Open Alerts" detail={`${issues.length} current`} />
          {issues.length === 0 ? (
            <EmptyState title="No open alerts" detail="All managed sites are currently clear." />
          ) : (
            <div className="issue-list">
              {issues.map((issue) => (
                <IssueCard issue={issue} key={`${issue.device_id}-${issue.issue_code}`} siteName={issue.site_name} />
              ))}
            </div>
          )}
        </div>

        <div>
          <SectionHeader title="Email Recipients" detail={`${recipients.length} configured`} />
          <form className="recipient-form" onSubmit={addRecipient}>
            <label className="field-label" htmlFor="recipient-email">Email address</label>
            <div className="inline-form">
              <input
                id="recipient-email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                type="email"
                placeholder="alerts@example.com"
              />
              <button className="primary-button" type="submit">Add</button>
            </div>
          </form>

          <div className="recipient-list">
            {recipients.length === 0 ? (
              <EmptyState title="No alert recipients" detail="Add an email address to receive selected alert families." />
            ) : (
              recipients.map((recipient) => (
                <div className="recipient-row" key={recipient.id}>
                  <div className="recipient-heading">
                    <div>
                      <div className="strong-text">{recipient.email}</div>
                      <div className="muted">{titleCase(recipient.status)}</div>
                    </div>
                    <button
                      className="text-button"
                      type="button"
                      onClick={() => setRecipients((current) => current.filter((item) => item.id !== recipient.id))}
                    >
                      Remove
                    </button>
                  </div>
                  <SubscriptionToggle label="Offline alerts" checked={recipient.subscriptions.device_disconnected} onChange={() => toggleSubscription(recipient.id, "device_disconnected")} />
                  <SubscriptionToggle label="Backup failed or overdue" checked={recipient.subscriptions.backup_failed_or_overdue} onChange={() => toggleSubscription(recipient.id, "backup_failed_or_overdue")} />
                  <SubscriptionToggle label="App update attention" checked={recipient.subscriptions.app_update_attention} onChange={() => toggleSubscription(recipient.id, "app_update_attention")} />
                </div>
              ))
            )}
          </div>
        </div>
      </section>
    </section>
  );
}

function ActivityPage({ activityRows }) {
  return (
    <section className="page">
      <PageHeader
        eyebrow="Timeline"
        title="Activity"
        subtitle="Recent events your team can act on or audit."
      />
      <ActivityList rows={activityRows} />
    </section>
  );
}

function EnrollmentPage({ deviceRows }) {
  const [claimCode, setClaimCode] = useState("");
  const site = deviceRows[0];
  const canConfirm = claimCode.trim().length >= 6;

  return (
    <section className="page narrow">
      <PageHeader
        eyebrow="Enrollment"
        title="Pair Home Assistant"
        subtitle="Pair a Home Assistant instance to one managed site."
      />
      <section className="section-band">
        <form className="claim-form" onSubmit={(event) => event.preventDefault()}>
          <label className="field-label" htmlFor="claim-code">Pairing code</label>
          <input
            id="claim-code"
            value={claimCode}
            onChange={(event) => setClaimCode(event.target.value)}
            placeholder="hs-claim-..."
          />
        </form>

        {canConfirm && site && (
          <div className="claim-review">
            <div className="muted">Ready to pair</div>
            <h2>{site.site_name}</h2>
            <p>{site.customer_display_name} / {site.compact_location}</p>
            <button className="primary-button" type="button">Confirm pairing</button>
          </div>
        )}
      </section>
    </section>
  );
}

function SettingsPage({ dashboard }) {
  return (
    <section className="page narrow">
      <PageHeader
        eyebrow="Settings"
        title="Account Settings"
        subtitle="Account-level defaults for the current workspace."
      />
      <section className="section-band">
        <dl className="fact-list">
          <Fact label="Managed sites" value={dashboard.summary.managed_sites} />
          <Fact label="Managed devices" value={dashboard.summary.managed_devices} />
          <Fact label="Email alerts" value={titleCase(dashboard.summary.email_alerts_status)} />
        </dl>
      </section>
    </section>
  );
}

function IssueList({ rows }) {
  const issues = rows.flatMap((row) => row.issues.map((issue) => ({ ...issue, site_name: row.site_name })));

  if (issues.length === 0) {
    return <EmptyState title="No open issues" detail="The public read model has no current customer-visible issues." />;
  }

  return (
    <div className="issue-list">
      {issues.map((issue) => (
        <IssueCard issue={issue} key={`${issue.device_id}-${issue.issue_code}`} siteName={issue.site_name} />
      ))}
    </div>
  );
}

function IssueCard({ issue, siteName }) {
  return (
    <article className={`issue-card ${issue.severity}`}>
      <div className="issue-card-heading">
        <span className={`severity-dot ${issue.severity}`} />
        <div>
          <div className="issue-title">{issue.label}</div>
          {siteName && <div className="muted">{siteName}</div>}
        </div>
      </div>
      <p>{issue.detail}</p>
      <div className="issue-meta">{titleCase(issue.source_area)} / {primaryActionLabel(issue.primary_action)}</div>
    </article>
  );
}

function ActivityList({ rows, compact = false }) {
  if (rows.length === 0) {
    return <EmptyState title="No activity yet" detail="Customer-visible events will appear here." />;
  }

  return (
    <div className={`activity-list ${compact ? "compact" : ""}`}>
      {rows.map((row) => (
        <article className="activity-row" key={row.activity_id}>
          <span className={`severity-dot ${row.severity}`} />
          <div>
            <div className="activity-heading">
              <span className="strong-text">{row.subject_label}</span>
              <span className="muted">{formatDateTime(row.occurred_at)}</span>
            </div>
            <div>{row.detail}</div>
            <div className="activity-meta">{titleCase(row.category)} / {titleCase(row.action)} / {row.actor_label}</div>
          </div>
        </article>
      ))}
    </div>
  );
}

function StatusSummary({ row }) {
  const hasIssues = row.issues.length > 0;
  return (
    <div className="status-summary">
      <StatusPill state={hasIssues ? "warning" : row.device.presence} label={hasIssues ? "Needs review" : presenceLabel(row.device.presence)} />
      <div className="muted">Last seen {formatRelativeTime(row.device.last_seen_at)}</div>
    </div>
  );
}

function VersionCell({ label, current, latest, status }) {
  const behind = latest && current && latest !== current;
  return (
    <div className="table-cell">
      <div className="cell-label">{label}</div>
      <div className="cell-value">{current || "Unknown"}</div>
      <div className={behind || status === "attention" ? "cell-detail warning" : "cell-detail"}>
        {behind ? `Latest ${latest}` : status ? titleCase(status) : "Current"}
      </div>
    </div>
  );
}

function BackupCell({ backup }) {
  return (
    <div className="table-cell">
      <div className="cell-label">Backup</div>
      <div className="cell-value">{backupLabel(backup)}</div>
      <div className={backup.status === "failed" ? "cell-detail warning" : "cell-detail"}>
        {backup.status === "failed"
          ? `Failed ${formatDateTime(backup.last_failure_at)}`
          : `Success ${formatDateTime(backup.last_success_at)}`}
      </div>
    </div>
  );
}

function StorageCell({ storage }) {
  return (
    <div className="table-cell">
      <div className="cell-label">Storage</div>
      <div className="cell-value">{titleCase(storage.status || "unknown")}</div>
      <div className={storage.status === "warning" ? "cell-detail warning" : "cell-detail"}>{storage.detail}</div>
    </div>
  );
}

function StatusPill({ state, label }) {
  const normalized = state === "online" ? "online" : state === "disconnected" ? "critical" : "warning";
  return (
    <span className={`status-pill ${normalized}`}>
      <span className={`severity-dot ${normalized === "critical" ? "critical" : normalized === "warning" ? "warning" : "ok"}`} />
      {label}
    </span>
  );
}

function IssuePill({ issue }) {
  return (
    <span className={`issue-pill ${issue.severity}`}>
      {issue.label}
    </span>
  );
}

function SubscriptionToggle({ label, checked, onChange }) {
  return (
    <label className="subscription-row">
      <span>{label}</span>
      <input type="checkbox" checked={checked} onChange={onChange} />
    </label>
  );
}

function SegmentedControl({ value, onChange, options }) {
  return (
    <div className="segmented-control">
      {options.map(([key, label]) => (
        <button
          key={key}
          type="button"
          className={value === key ? "active" : ""}
          onClick={() => onChange(key)}
        >
          {label}
        </button>
      ))}
    </div>
  );
}

function PageHeader({ eyebrow, title, subtitle, action }) {
  return (
    <header className="page-header">
      <div>
        {eyebrow && <div className="eyebrow">{eyebrow}</div>}
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
      {action && <div className="page-action">{action}</div>}
    </header>
  );
}

function SectionHeader({ title, detail }) {
  return (
    <div className="section-header">
      <h2>{title}</h2>
      {detail && <span>{detail}</span>}
    </div>
  );
}

function EmptyState({ title, detail }) {
  return (
    <div className="empty-state">
      <div className="strong-text">{title}</div>
      <div className="muted">{detail}</div>
    </div>
  );
}

function Fact({ label, value, detail }) {
  return (
    <div className="fact">
      <dt>{label}</dt>
      <dd>
        <div>{value || "None"}</div>
        {detail && <span>{detail}</span>}
      </dd>
    </div>
  );
}

function SiteIcon({ category }) {
  const normalized = category || "site";
  const label = normalized === "business" ? "B" : normalized === "other" ? "S" : "H";
  return <span className="site-icon" aria-hidden="true">{label}</span>;
}

function titleCase(value) {
  return String(value || "")
    .replace(/_/g, " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function presenceLabel(value) {
  if (value === "online") return "Online";
  if (value === "disconnected") return "Disconnected";
  if (value === "degraded") return "Degraded";
  return "Unknown";
}

function backupLabel(backup) {
  if (!backup?.status) return "Unknown";
  if (backup.status === "succeeded") return "Current";
  if (backup.status === "failed") return "Failed";
  if (backup.status === "overdue") return "Overdue";
  return titleCase(backup.status);
}

function focusMessage(focus) {
  if (focus === "view_backups") return "Reviewing backup posture for this device.";
  if (focus === "view_updates") return "Reviewing update posture for this device.";
  return "Reviewing this device.";
}

function latestDetail(latest) {
  return latest ? `Latest ${latest}` : "Latest unavailable";
}

function formatDateTime(value) {
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

function formatRelativeTime(value) {
  if (!value) return "unknown";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;

  const diffMs = Math.max(0, Date.now() - date.getTime());
  const diffHours = Math.max(1, Math.round(diffMs / 36e5));

  if (diffHours < 24) return `${diffHours} ${diffHours === 1 ? "hour" : "hours"} ago`;

  const diffDays = Math.round(diffHours / 24);
  return `${diffDays} ${diffDays === 1 ? "day" : "days"} ago`;
}
