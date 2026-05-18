package main

type uiView struct {
	Ready              readyResponse
	Status             statusResponse
	PairingCode        string
	PairingCodeVisible bool
	Message            string
}

func newUIView(snapshot EnrollmentSnapshot, ready readyResponse) uiView {
	pairingVisible := snapshot.PairingCode != "" && (snapshot.ClaimState == ClaimStateUnclaimed || snapshot.ClaimState == ClaimStatePairingPending)
	return uiView{
		Ready:              ready,
		Status:             newStatusResponse(snapshot),
		PairingCode:        snapshot.PairingCode,
		PairingCodeVisible: pairingVisible,
		Message:            snapshot.EnrollmentStatusMessage,
	}
}

// uiHTML is the temporary skeleton ingress page. The production HomeSignal
// Manager UI should be mounted as static React assets served by this Go runtime;
// see design-docs/ha-app-ui-mount-plan.md for the adapter and route contract.
const uiHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>HomeSignal</title>
  <style>
    :root { color-scheme: light dark; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; padding: 2rem; background: Canvas; color: CanvasText; }
    main { max-width: 760px; margin: 0 auto; }
    h1 { margin: 0 0 0.5rem; font-size: 1.75rem; letter-spacing: 0; }
    h2 { margin: 1.5rem 0 0.75rem; font-size: 1.1rem; letter-spacing: 0; }
    .status { display: inline-block; margin: 1rem 0; padding: 0.35rem 0.65rem; border-radius: 0.4rem; border: 1px solid ButtonBorder; }
    .code { display: inline-block; margin: 0.5rem 0 1rem; padding: 0.6rem 0.8rem; border: 1px solid ButtonBorder; border-radius: 0.4rem; font-size: 1.5rem; font-weight: 700; letter-spacing: 0.08em; }
    dl { display: grid; grid-template-columns: minmax(8rem, 15rem) 1fr; gap: 0.75rem 1rem; }
    dt { font-weight: 650; }
    dd { margin: 0; overflow-wrap: anywhere; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .muted { color: GrayText; }
  </style>
</head>
<body>
  <main>
    <h1>HomeSignal</h1>
    <p>{{ .Message }}</p>
    <div class="status">{{ .Status.ClaimState }}</div>

    {{ if .PairingCodeVisible }}
      <h2>Pairing Code</h2>
      <div class="code">{{ .PairingCode }}</div>
      {{ if .Status.PairingCodeExpiry }}<p class="muted">Expires at <code>{{ .Status.PairingCodeExpiry }}</code></p>{{ end }}
    {{ else if eq .Status.ClaimState "CLAIMED" }}
      <h2>Claimed Device</h2>
      <dl>
        <dt>Device ID</dt><dd><code>{{ .Status.DeviceID }}</code></dd>
        <dt>IoT Thing</dt><dd><code>{{ .Status.IoTThingName }}</code></dd>
      </dl>
    {{ else if eq .Status.ClaimState "REVOKED" }}
      <h2>Revoked</h2>
      <p class="muted">This app is in a safe revoked state. Release cleanup is not implemented in this build.</p>
    {{ else }}
      <h2>Enrollment</h2>
      <p class="muted">A claimable pairing code is not currently available.</p>
    {{ end }}

    <h2>Status</h2>
    <dl>
      <dt>Installation ID</dt><dd><code>{{ .Status.InstallationID }}</code></dd>
      <dt>Version</dt><dd><code>{{ .Status.Version }}</code></dd>
      <dt>HomeSignal API</dt><dd>{{ .Status.HomeSignalConfigured }}</dd>
      <dt>AWS IoT</dt><dd>{{ .Status.AWSIoTConfigured }}</dd>
      <dt>Options loaded</dt><dd>{{ .Ready.OptionsLoaded }}</dd>
      <dt>Supervisor token</dt><dd>{{ .Ready.SupervisorToken }}</dd>
      <dt>Core API</dt><dd><code>{{ .Ready.CoreAPIBaseURL }}</code></dd>
      {{ if .Ready.DegradedReason }}<dt>Degraded reason</dt><dd>{{ .Ready.DegradedReason }}</dd>{{ end }}
    </dl>
  </main>
</body>
</html>
`
