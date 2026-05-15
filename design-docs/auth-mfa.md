Add MFA and step-up authorization to the existing HomeSignal authorization service.

Goal:
Integrate MFA-aware authorization into the existing auth and command issuance flow without redesigning the service architecture.

Existing service remains responsible for:
- authentication
- org/site membership
- capability resolution
- authorization
- command envelope creation

This addendum introduces:
- MFA enrollment state
- MFA freshness tracking
- step-up challenge flow
- capability-class MFA policy
- MFA assurance fields in auth and command context

Core model:
MFA policy is capability-class based, not role-name based.

Capability classes:

read:
  MFA optional

write:
  MFA enrollment required

run:
  MFA enrollment required
  recent MFA required

admin:
  MFA enrollment required
  recent MFA required

recovery:
  disabled in release zero
  future: MFA enrollment required + recent MFA + advanced recovery mode

Examples:

read:
- view_site
- view_health
- view_topology

write:
- edit_site_metadata
- change_notification_settings

run:
- refresh_status
- refresh_topology
- upload_diagnostics
- restart_agent
- update_agent

admin:
- invite_user
- remove_user
- change_permissions
- transfer_ownership

recovery:
- restore_backup
- rollback_version

Authentication assurance levels:

aal1:
  normal authenticated session

aal2_email:
  recent email OTP challenge completed

aal2_totp:
  recent TOTP challenge completed

Release zero MFA support:
- email OTP required for step-up
- TOTP optional if enabled
- SMS not supported by default

Capability assurance requirements:

read:
  minimum_assurance: aal1

write:
  minimum_assurance: aal1
  MFA enrollment required

run:
  minimum_assurance: aal2_email

admin:
  minimum_assurance: aal2_email

recovery:
  disabled in release zero
  future minimum_assurance: aal2_totp

Important:
MFA is not required at login by default.

Login establishes identity.
Step-up establishes recent authority for sensitive actions.

Flow:
1. User logs in normally.
2. User performs read-only actions without MFA.
3. User requests write/run/admin action.
4. Authorization resolves capability class and required assurance.
5. If MFA is required and missing/stale:
   return STEP_UP_REQUIRED or MFA_ENROLLMENT_REQUIRED.
6. Client prompts for email OTP or TOTP challenge.
7. Service verifies challenge.
8. Session mfa_verified_at and auth_assurance_level are updated.
9. User retries action.
10. Authorization succeeds if assurance and freshness requirements are satisfied.

Freshness windows:
admin: 10 minutes
run: 15 minutes
recovery: 5 minutes future-only

Data model additions:

User MFA state:
- user_id
- mfa_enrolled
- enrolled_factors
- preferred_factor
- created_at
- updated_at

Session/auth context:
- session_id
- user_id
- authenticated_at
- mfa_verified_at
- auth_assurance_level
- mfa_method

Authorization context additions:
- capability_class
- required_assurance
- auth_assurance_level
- mfa_verified_at
- mfa_age_seconds
- required_mfa_window_seconds
- denial_reason

Authorization flow:
1. Authenticate user.
2. Resolve org/site membership.
3. Resolve effective capabilities.
4. Resolve capability class.
5. Verify capability exists.
6. Evaluate MFA policy for capability class.
7. If MFA enrollment required and no MFA enrolled:
   deny MFA_ENROLLMENT_REQUIRED.
8. If session assurance is below required assurance:
   deny STEP_UP_REQUIRED.
9. If recent MFA required and MFA stale:
   deny MFA_TOO_OLD.
10. If authorization succeeds:
    continue existing command issuance flow.

Step-up flow:
When authorization returns STEP_UP_REQUIRED:
- API responds 403 STEP_UP_REQUIRED
- response includes:
  - required_assurance
  - capability_class
  - required_mfa_window_seconds
  - available_factors
  - challenge_id or step_up_token

Client:
1. prompts for MFA
2. submits challenge response
3. service verifies factor
4. updates session assurance
5. retries original request

MFA enrollment flow:
When authorization returns MFA_ENROLLMENT_REQUIRED:
- API responds 403 MFA_ENROLLMENT_REQUIRED
- client routes user to MFA enrollment
- service verifies enrolled factor before enabling MFA
- user retries action

Command envelope additions:
- capability_class
- required_assurance
- auth_assurance_level
- mfa_verified_at
- mfa_age_seconds
- policy_version

Release zero policy:
- read actions allowed without MFA
- write actions require MFA enrollment
- run actions require recent step-up
- admin actions require recent step-up
- recovery actions disabled

Denial codes:
- MFA_ENROLLMENT_REQUIRED
- STEP_UP_REQUIRED
- MFA_TOO_OLD
- MFA_VERIFICATION_FAILED
- CAPABILITY_DISABLED
- RECOVERY_DISABLED

Audit:
Audit:
- MFA enrollment required
- step-up required
- MFA success
- MFA failure
- action approved with MFA context
- action denied due to stale MFA

Acceptance criteria:
- Read-only actions work without MFA.
- Write actions require enrolled MFA.
- Run/admin actions require recent step-up.
- User with stale MFA receives STEP_UP_REQUIRED.
- Successful MFA updates auth assurance state.
- Retried run/admin action succeeds if MFA is fresh.
- Capability checks happen before MFA grants action.
- Command envelope contains MFA assurance fields.
- Recovery actions remain disabled in release zero.