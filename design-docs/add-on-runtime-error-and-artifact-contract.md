# Add-On Runtime Error And Artifact Contract

This spec defines how the claimed HomeSignal add-on reports runtime errors, advertises locally available diagnostics, and fulfills cloud-authorized artifact upload requests. It complements the add-on skeleton, enrollment contract, topic design, state-change policy propagation, and artifact upload broker specs.

The goal is a useful failure path without creating an error storm, arbitrary local file upload, IoT-delivered signed URLs, or MQTT blob channel.

## Scope

Applies after a device is claimed and using durable AWS IoT credentials.

In scope:

- small structured runtime error events
- routine device-originated structured log events governed by publish policy
- policy application failure reporting
- local diagnostic excerpts
- `more_logs_available` hints
- temporary cloud-initiated debug capture
- cloud-authorized `upload_artifact` command handling
- error-log artifact upload completion reporting
- local recursion guards and suppression counters

Out of scope:

- enrollment/bootstrap API calls
- arbitrary local file upload
- cloud-side authorization policy
- customer-visible alert policy
- full diagnostics product workflow
- topology parsing as product state
- customer-initiated debug mode

## Principles

- Routine device-originated logs are governed by publish policy, including
  level, event/byte budgets, diagnostic excerpt allowance, and freshness.
- Device may announce artifact availability; cloud grants upload capability.
- Cloud authorization is required before any artifact bytes leave the device.
- Command ACK semantics follow `command-lifecycle.md`: ACK means accepted/rejected, not mere receipt.
- Runtime event/command result channels carry small JSON facts, not blobs.
- IoT Core carries only tiny artifact command notices; command details, upload negotiation, signed URL issuance, completion, and result reporting use the mTLS Agent HTTPS API defined in `artifact-upload-broker.md`.
- Object storage holds bytes; Postgres artifact records hold authority and queryable metadata.
- Upload failures must not recursively generate more log-upload requests.
- Local add-on behavior must fail closed for unsafe commands and fail quiet for recursive diagnostics loops.

## Routine Device Logs

Routine device logs are structured events, not raw log streams.

The add-on may emit routine runtime log summaries only when allowed by the
active publish policy. The policy controls minimum level, max events per
window, max bytes per window, diagnostic excerpt allowance, and health snapshot
cadence.

Standing levels:

- `quiet`: critical agent alarms and required health only
- `normal`: warning/critical runtime log summaries and low-rate health
- `verbose`: selected info-level runtime facts under strict caps

`debug` is not a standing routine log level. Rich debug capture uses temporary
debug capture commands.

Routine log summaries must be collapsed by component, reason code, and time
window where practical. Runtime messages must include
`applied_publish_policy_version` and must not include
private keys, claim invite codes, tokens, signed URLs, cookies, raw secrets, broad
Home Assistant config, or unbounded stack traces.

## Temporary Debug Capture

Temporary debug capture is cloud-initiated by an authorized internal/support
actor. It is not a customer-facing local toggle.

The add-on may accept debug capture commands only when:

- command type is allowlisted, such as `enable_debug_capture`,
  `disable_debug_capture`, `collect_debug_snapshot`, or `request_debug_bundle`
- command details are fetched through the mTLS Agent HTTPS API
- the command has a bounded `debug_session_id`, `expires_at`, redaction profile,
  and max local bytes/events
- requested capture categories are allowlisted
- the local add-on can enforce expiry without another cloud call

Allowed v0 capture categories:

- HomeSignal add-on runtime status
- cloud connectivity checks
- Agent HTTPS auth status summaries
- AWS IoT connectivity status summaries
- recent HomeSignal command lifecycle summaries
- publish-policy version/freshness facts
- update readiness/status facts
- backup status facts
- redacted recent add-on log excerpts

Disallowed v0 capture categories:

- arbitrary local files
- arbitrary shell output
- broad Home Assistant configuration snapshots
- private keys, tokens, claim invite codes, cookies, signed URLs, or raw secrets
- unrestricted Home Assistant service-call history

Debug capture must automatically expire locally. If upload fails, the add-on
must follow the artifact upload recursion guard and must not create an error
storm while trying to report debug-mode errors.

## Error Event Shape

When the add-on detects an important runtime error, it emits a bounded
`agent_alarm` event.

Required fields:

```json
{
  "alarm_type": "publish_policy_apply_failed",
  "severity": "warning",
  "operation": "apply_publish_policy_from_shadow",
  "reason_code": "unsupported_policy_schema",
  "local_error_id": "err_01J00000000000000000000000",
  "first_seen_at": "2026-05-13T18:00:00Z",
  "last_seen_at": "2026-05-13T18:00:00Z",
  "occurrence_count": 1,
  "more_logs_available": true,
  "local_artifact_ref": "logref_01J00000000000000000000000"
}
```

Optional contextual fields:

- `command_id`
- `artifact_request_id`
- `artifact_purpose`
- `attempted_policy_version`
- `active_policy_version`
- `diagnostic_excerpt`
- `diagnostic_excerpt_truncated`
- `suppressed_count`

`diagnostic_excerpt` may contain a redacted text excerpt capped at 5 KB total for the full event. It must not include secrets, signed URLs, raw policy blobs, local file contents beyond the small excerpt, or large stack traces.

If more useful local logs exist, set `more_logs_available=true` and include `local_artifact_ref`. The add-on must not upload those logs until the cloud sends an allowlisted artifact upload command.

## Policy Application Failure

If applying publish policy from shadow desired state or a `refresh_publish_policy` command fails:

1. Report the bounded reason through shadow reported state or command result, depending on the path.
2. Keep enforcing the last accepted policy or conservative default.
3. Emit `agent_alarm` with one of:
   - `publish_policy_apply_failed`
   - `publish_policy_rejected_suspicious`
4. Include a redacted diagnostic excerpt up to 5 KB when helpful.
5. Set `more_logs_available=true` only when a local redacted log bundle can be generated safely.

Security-relevant failures use `warning` or `critical`, including wrong device, rollback attempt, malformed policy from cloud, invalid signature/MAC when introduced, or repeated invalid policy delivery.

## Artifact Upload Command Notice

The add-on accepts artifact upload only after a cloud-authorized command notice. The IoT command notice is intentionally tiny and must not contain signed URLs, large command details, secrets, or upload payloads.

The add-on should ACK `accepted` or `rejected` through the Agent HTTPS API within the command ACK window after validating that the command type is allowlisted and that it can fetch command details.

```text
POST /agent/commands/{command_id}/ack
```

Command type:

```text
upload_artifact
```

Command notice payload:

```json
{
  "command_id": "cmd_01J00000000000000000000000",
  "type": "upload_artifact"
}
```

After receiving the notice, the add-on retrieves command details over HTTPS:

```text
GET /agent/commands/{command_id}
```

Then it requests an upload session:

```text
POST /agent/commands/{command_id}/artifact-upload
```

The Agent HTTPS API authenticates the add-on with mTLS, derives device identity from the client certificate, verifies the command belongs to that device, and returns a short-lived signed upload URL.

The add-on must validate before upload:

- command type is allowlisted
- `purpose` is allowlisted
- `local_artifact_ref` exists and maps to generated content, not an arbitrary path
- command details came from the mTLS Agent HTTPS API
- signed URL is HTTPS
- destination host matches approved object-storage patterns
- method, expiry, max bytes, and content type match the upload session
- generated artifact is redacted according to the requested profile
- generated artifact size is within limit

If validation fails, reject the command and emit only a bounded error event under the recursion rules below.

## Upload Completion Result

After attempting upload, the add-on reports upload completion and terminal command lifecycle result through the mTLS Agent HTTPS API.

Upload completion endpoint:

```text
POST /agent/artifact-uploads/{upload_id}/complete
```

Command result endpoint:

```text
POST /agent/commands/{command_id}/result
```

Successful result:

```json
{
  "command_id": "cmd_01J00000000000000000000000",
  "artifact_request_id": "art_01J00000000000000000000000",
  "purpose": "error_log_bundle",
  "status": "uploaded",
  "size_bytes": 34567,
  "sha256": "hex-or-base64-digest",
  "content_type": "application/gzip",
  "generated_at": "2026-05-13T18:01:00Z",
  "redaction_profile": "default_v1",
  "manifest_sha256": "hex-or-base64-digest"
}
```

Failed result:

```json
{
  "command_id": "cmd_01J00000000000000000000000",
  "artifact_request_id": "art_01J00000000000000000000000",
  "purpose": "error_log_bundle",
  "status": "failed",
  "reason_code": "upload_url_expired",
  "retryable": false
}
```

The result must not contain the signed URL. Signed URLs must not be sent over IoT Core.

## Diagnostic Recursion Guard

Upload failures may emit one small `agent_alarm` summary, but they must not recursively request more logs.

Rules:

- `artifact_upload_failed` alarms are rate-limited and collapsed by device, artifact purpose, failure reason, and time window.
- Upload-failure alarms must set `more_logs_available=false` by default.
- A failure while uploading `error_log_bundle` must never trigger another `error_log_bundle` request.
- Artifact upload code must not use the artifact-upload path to report errors caused by artifact upload.
- Local suppression counters are tracked and included in the next health telemetry snapshot.
- If cloud repeatedly requests an upload that the add-on rejects as invalid, the add-on emits a bounded alarm and then suppresses repeats for the collapse window.

Initial upload-related alarm type:

```text
artifact_upload_failed
```

## Device-Initiated Artifact Availability

Some artifacts are discovered locally before the cloud knows they are useful.

Allowed device-initiated availability signals:

- `error_log_bundle` after a significant add-on error
- `topology_snapshot` after topology changes, when topology upload is enabled by policy

The add-on sends only a small event:

```json
{
  "artifact_purpose": "topology_snapshot",
  "local_artifact_ref": "toporef_01J00000000000000000000000",
  "estimated_size_bytes": 45000,
  "content_type": "application/json",
  "schema_version": "1",
  "sha256_if_available": "hex-or-base64-digest"
}
```

Cloud may ignore it, rate-limit it, or create a command row and send a tiny artifact command notice. The availability event itself is not upload authorization.

## Local Persistence

The add-on should keep a small local diagnostic registry:

- `local_error_id`
- `local_artifact_ref`
- artifact purpose
- generated/redaction status
- estimated size
- first seen / last seen
- suppression counters
- expiry for local diagnostic material

This registry is local metadata only. It must not store cloud authority, signed URLs after expiry, or customer-visible state.

## Acceptance Criteria

- Error events are useful without requesting a blob.
- Diagnostic excerpts are capped at 5 KB total per runtime event.
- `more_logs_available` never grants upload capability by itself.
- `upload_artifact` is ACK-required and allowlisted.
- Signed URLs are never sent over IoT Core, logged, or echoed in results.
- Upload failures do not recursively create more log-upload requests.
- Cloud can correlate alarm, command, artifact record, upload result, and stored object.
