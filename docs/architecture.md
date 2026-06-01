# Architecture

> Authorship: agent-authored from observed API behavior and source code.

`cpt-crawl` is a small CLI over Combustion's cloud path.

```text
local auth bundle
  -> Firebase Secure Token API
  -> Firestore user/probe metadata
  -> Combustion data API
  -> JSON/CSV files or live JSON snapshots
```

## Packages

- `cmd/cpt-crawl`: CLI flags, command routing, file layout.
- `internal/combustion`: auth refresh, Firestore decoding, session API, CSV writing, completeness checks.

## Auth Boundary

The repo does not contain user tokens or the vendor Firebase Web API key. The operator provides them at runtime through an encrypted auth bundle or environment variables.

Firebase `idToken` values are short-lived. The client refreshes before expiry and retries once after a 401.

## Data Flow

1. Decrypt auth bundle, or read a local plaintext path from `CPT_FIREBASE_AUTH_JSON`.
2. Refresh Firebase `idToken`.
3. Read Firestore `users/<USER_KEY>` to find associated probes.
4. Read Firestore `probes/<DEVICE_KEY>/probe_status/current` for current session IDs.
5. Use `v1/sessions` for historical session IDs.
6. Use `v1/session` for sequence ranges.
7. Use `v1/session_data` in 1000-row chunks.
8. Sort rows by sequence number and fail the command if any expected row is missing.

`dump-history` writes the returned `v1/sessions` index, dumps each unique `device_session_id` selected by the command, and requires every expected sequence row from `v1/session_data` to be present. It exits nonzero if the session index exceeds the 100-page guard.

The session index output keeps the raw `v1/sessions` object under `source` so fields not yet modeled by Go structs are still available to agents.

## Live Windows

`window` and `watch` do not subscribe to a push stream. They poll the latest session metadata and fetch recent tail rows. `window` emits those rows once. `watch` keeps the last emitted sequence number per probe/session and writes JSON Lines only for newly seen samples.

## Metadata Boundary

- The API is unofficial and may change.
- The implemented surfaces are `users/<USER_KEY>`, `probes/<DEVICE_KEY>/probe_status/current`, `v1/sessions`, `v1/session`, and `v1/session_data`.
- Raw `v1/sessions` objects are preserved under `source`.
- Raw `v1/session` responses are written as `session.json`.
- Fields not present in those responses are not added.
- `dump-history` is intentionally row-complete rather than fast.
- Output files contain private cook history.
