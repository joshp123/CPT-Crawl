# CPT-Crawl

> Authorship: agent-authored. Not official Combustion Inc. documentation.

Unofficial Go CLI for exporting Combustion Predictive Thermometer cloud data as JSON and CSV.

It is for users who already own Combustion probes and want their own cook history in files an agent can read. It does not pair probes, control devices, or use BLE.

## What This Does

Combustion's app can show cook history. `cpt-crawl` gets that same kind of thermometer history out of the cloud and onto disk without opening the app.

The useful unit is a **session**: one probe's recorded time-series for one cook or one continuous probe run. A session contains thousands of **samples**. Each sample is one timestamped row with probe temperatures (`T1` through `T8`), virtual core/surface/ambient temperatures, prediction fields, and a sequence number.

`cpt-crawl` turns that cloud data into:

- JSON files for agents and scripts.
- Combustion-style CSV files for spreadsheets and inspection.
- A completeness summary that says how many rows were expected, how many were written, and whether any sequence numbers are missing.
- Optional live JSON Lines for agents that should watch a cook while it is happening.

Example: a real three-probe cook produced `20,650`, `20,647`, and `23,946` rows, all with `missing_count: 0`. That means the export captured every expected sample from those three probe sessions.

## Command Guide

First time through: install the binary, create the auth bundle in [Auth](#auth), run `sessions` to prove the account can see probes, then choose `dump`, `dump-history`, `window`, or `watch`.

| If you want... | Run | What you get |
|---|---|---|
| To see what cook sessions exist before exporting | `sessions` | JSON on stdout, keyed by probe serial |
| The current/latest session for each probe | `dump` | JSON, CSV, and `summary.json` files under `--out` |
| A full local archive of indexed historical sessions | `dump-history` | One folder per probe/session plus a root `summary.json` |
| Current context for an agent right now | `window --minutes N` | Recent rows on stdout once, then exit |
| Continuous live monitoring | `watch --interval 30s` | JSON Lines on stdout for new samples |

Use `sessions` when you want to know what the cloud has before exporting. It lists the indexed sessions Combustion knows about for each associated probe.

```sh
cpt-crawl sessions --auth-age ~/.config/cpt-crawl/firebase-auth.json.age
```

An anonymized, shortened success output looks like this. The real output includes more raw source fields, but the important proof is: the account can see probe serials and cloud session IDs.

```json
{
  "21123CB8": [
    {
      "device_serial_number": "21123CB8",
      "device_session_id": "3597489004",
      "sample_period": 5000,
      "started_at": "2026-06-01T16:59:47.12284Z",
      "sequence_number_ranges": [[0, 23945]]
    }
  ]
}
```

`sample_period` is milliseconds between samples, and `sequence_number_ranges` shows the first and last sample numbers the cloud lists for that session.

Use `dump` when you want the current or most recent active session for each probe. This is the normal command during or right after a cook. It writes JSON, CSV, and `summary.json` under the output directory. If a selected probe has no retrievable current session/status, the command exits with an error for that probe; use `sessions` or `dump-history` to inspect older indexed sessions.

```sh
cpt-crawl dump --auth-age ~/.config/cpt-crawl/firebase-auth.json.age --out outputs/current
```

Use `dump-history` when you want an archive. It walks the cloud session index and dumps each unique retrievable historical session.

```sh
cpt-crawl dump-history --auth-age ~/.config/cpt-crawl/firebase-auth.json.age --out outputs/history
```

Use `window` when an agent needs a snapshot of what is happening now. A recent window means "samples from the active session whose `sampled_at` time falls within the last N minutes." For example, if a probe is sampling every five seconds, `--minutes 5` is about 60 rows per probe. It does not dump the whole cook; it prints just the recent rows once, then exits.

```sh
cpt-crawl window --auth-age ~/.config/cpt-crawl/firebase-auth.json.age --minutes 5
```

Use `watch` when an agent should keep monitoring. It polls the cloud and prints JSON Lines for new samples as they appear. One line means one newly observed sample for one probe/session. It does not reprint the prior window snapshot on each poll.

```sh
mkdir -p outputs
cpt-crawl watch --auth-age ~/.config/cpt-crawl/firebase-auth.json.age --interval 30s | tee outputs/live.jsonl
```

In practice:

- After a cook, run `dump` and hand the output directory to an agent.
- For a personal archive, run `dump-history`.
- For a live cook assistant, run `window` once for current context, then `watch` for the stream of new rows.
- If a summary says `missing_count: 0`, every expected sequence row for that session was written.

## Status

Experimental. The cloud API is reverse engineered and can change without notice. Keep tokens and exported cook data private.

## Requirements

- Go 1.24 or newer.
- `age` only if you use the encrypted `--auth-age` path.
- A Combustion account that already has probes associated in the official app.

## Install

With Go 1.24 or newer:

```sh
go install github.com/joshp123/cpt-crawl/cmd/cpt-crawl@v0.1.3
```

If your shell cannot find `cpt-crawl` after install, add `$(go env GOPATH)/bin` or `GOBIN` to `PATH`.

If `go install` fails with an `unknown revision` or module proxy cache error, retry by fetching directly from GitHub:

```sh
GOPROXY=direct GONOSUMDB=github.com/joshp123/cpt-crawl \
  go install github.com/joshp123/cpt-crawl/cmd/cpt-crawl@v0.1.3
```

From a checkout:

```sh
git clone https://github.com/joshp123/cpt-crawl.git
cd cpt-crawl
mkdir -p ./bin
go build -o ./bin/cpt-crawl ./cmd/cpt-crawl
```

## Auth

`cpt-crawl` needs a Firebase refresh token for your Combustion account and the app-public Firebase Web API key used by the Combustion app. Do not commit either value.

The Firebase Web API key is vendor app configuration, not your personal secret. It is still omitted here because GitHub secret scanning treats literal Google API keys as alerts. Provide it at runtime through `apiKey` in the auth bundle, `--api-key`, or `CPT_FIREBASE_API_KEY`.

Install alone is not enough to run the commands; the CLI needs a local Firebase auth bundle for your Combustion account. Today this is mostly for people comfortable having a coding agent perform the one-time Firebase token bootstrap. There is currently no standalone `cpt-crawl login` command.

The supported auth source is the Combustion-backed Firebase account used by the official app, with Apple as the identity provider. The bootstrap exchanges that sign-in for a Firebase `refreshToken` and `localId`, combines them with the app-public Firebase `apiKey`, and saves the local bundle shown below.

The human-level flow is:

1. Use the official Combustion app normally so your probes are associated with your account.
2. Have a coding agent follow [.agents/auth-bootstrap.txt](.agents/auth-bootstrap.txt) to exchange Sign in with Apple/Firebase auth into a local Firebase refresh token.
3. Store that token outside the checkout, preferably encrypted as `~/.config/cpt-crawl/firebase-auth.json.age`.
4. Run `cpt-crawl sessions --auth-age ~/.config/cpt-crawl/firebase-auth.json.age` to prove the account can see your probes.

Until `cpt-crawl` has a login command, [.agents/auth-bootstrap.txt](.agents/auth-bootstrap.txt) is the supported bootstrap procedure for obtaining `refreshToken`, `localId`, and `apiKey`.

After that first bootstrap, you do not sign in again for normal dumps; the CLI refreshes short-lived Firebase `idToken` values from the stored refresh token.

Preferred auth bundle:

```json
{
  "apiKey": "APP_PUBLIC_FIREBASE_WEB_API_KEY",
  "localId": "FIREBASE_LOCAL_ID",
  "refreshToken": "FIREBASE_REFRESH_TOKEN",
  "email": "optional@example.com"
}
```

Preferred encrypted storage. Create the plaintext JSON outside the checkout first, then encrypt it and remove the plaintext copy.

```sh
mkdir -p ~/.config/cpt-crawl
age -r "$(ssh-keygen -y -f ~/.ssh/id_ed25519)" \
  -o ~/.config/cpt-crawl/firebase-auth.json.age \
  /path/to/firebase-auth.json
chmod 600 ~/.config/cpt-crawl/firebase-auth.json.age
trash /path/to/firebase-auth.json || rm /path/to/firebase-auth.json
```

Then run:

```sh
cpt-crawl sessions --auth-age ~/.config/cpt-crawl/firebase-auth.json.age
```

Local plaintext storage is also supported for throwaway experiments. Create the JSON file outside the checkout first, then copy it into place:

```sh
mkdir -p ~/.config/cpt-crawl
install -m 600 /path/to/firebase-auth.json ~/.config/cpt-crawl/firebase-auth.json
CPT_FIREBASE_AUTH_JSON=~/.config/cpt-crawl/firebase-auth.json cpt-crawl sessions
```

Do not use the plaintext path in a repository checkout, shell transcript you plan to publish, or shared machine.

The bootstrap file is agent-facing and intentionally not product copy.

## Useful Flags

- `--auth-age`: age-encrypted auth bundle.
- `--identity`: age identity, default `~/.ssh/id_ed25519`.
- `--api-key`: Firebase Web API key, if not in the auth bundle.
- `--serials`: comma-separated probe serial filter.
- `--out`: output path for commands that write files. `watch` writes to stdout; use shell redirection or `tee`.
- `--interval` / `-n`: `watch` polling interval. Duration values like `30s` work; bare numbers like `-n 5` mean seconds.
- `CPT_FIREBASE_AUTH_JSON`: local plaintext auth bundle path, for private experiments only.

## Outputs

`sessions` prints JSON to stdout by default. The top-level keys are probe serials; each value is the session index returned for that probe. Use `--out sessions.json` if you want to save that index to a file.

`dump` writes:

- `summary.json`: completeness summary for every exported session.
- `session-<serial>-<session_id>.json`: raw session metadata from the cloud.
- `session-data-<serial>-<session_id>.json`: exported rows plus probe/session metadata.
- `ProbeData_<serial>_<session_id>_cloud.csv`: Combustion-style CSV for spreadsheet use.

The `session-data` JSON uses the same sample field names shown in the `window` and `watch` examples. A shortened excerpt looks like this:

```json
{
  "probe": {"serial": "21123CB8"},
  "status": {"session_id": 3597489004, "sample_period": 5000},
  "rows": [
    {
      "sampled_at": "2026-06-01T16:59:47.12284Z",
      "sequence_number": 23945,
      "t1": 25.5,
      "t2": 25.75,
      "virtual_core": 25.5,
      "virtual_surface": 25.95,
      "virtual_ambient": 23.85,
      "prediction_state": 2,
      "prediction_value_seconds": 131071
    }
  ]
}
```

`dump-history` writes:

- `<out>/summary.json`: completeness summary for the whole archive run.
- `<out>/sessions-<serial>.json`: session index for that probe.
- `<out>/<serial>/<session_id>/session.json`: raw session metadata.
- `<out>/<serial>/<session_id>/session-data.json`: exported rows plus probe/session metadata.
- `<out>/<serial>/<session_id>/ProbeData_<serial>_<session_id>_cloud.csv`: CSV rows for that session.

`dump-history` saves each unique historical session the cloud lists for your probes. The current safety guard is 100 pages of 100 sessions per probe. If an account exceeds that, `cpt-crawl` stops with an error instead of silently skipping data, and the guard should be raised after inspecting the API response shape.

`window` emits one JSON snapshot with recent `rows` for each probe. A shortened snapshot looks like this; the `rows` array is truncated to one row and many row fields are omitted for readability:

```json
{
  "generated_at": "2026-06-01T17:00:00Z",
  "minutes": 5,
  "probes": [
    {
      "serial": "21123CB8",
      "session_id": 3597489004,
      "rows_returned": 60,
      "rows_in_window": 60,
      "rows": [
        {
          "sampled_at": "2026-06-01T16:59:47.12284Z",
          "sequence_number": 23945,
          "t1": 25.5,
          "t2": 25.75,
          "virtual_core": 25.5,
          "virtual_surface": 25.95,
          "virtual_ambient": 23.85
        }
      ]
    }
  ]
}
```

`watch` emits JSON Lines: one object for the latest sample when a probe/session is first seen, then one object for each newer sample. It prints nothing on polling intervals with no new sample. A single line has probe/session identity plus the sample row; some row fields are omitted here for readability:

```json
{"generated_at":"2026-06-01T17:00:00Z","serial":"21123CB8","session_id":3597489004,"sampled_at":"2026-06-01T16:59:47.12284Z","sequence_number":23945,"t1":25.5,"t2":25.75,"virtual_core":25.5,"virtual_surface":25.95,"virtual_ambient":23.85}
```

## Real Cook Example

This example came from a real authenticated `cpt-crawl dump` on 2026-06-01. Probe serials and session IDs have been anonymized but kept internally consistent; row counts, temperatures, timestamps, sequence numbers, and CSV shape are from the real dump.

The important first check is row completeness. This is a selected-fields excerpt from `summary.json`; path fields, chunk logs, and empty `missing` arrays are omitted here:

```json
[
  {
    "serial": "2111009D",
    "session_id": 2626918122,
    "expected_rows": 20650,
    "rows": 20650,
    "missing_count": 0,
    "min_seq": 0,
    "max_seq": 20649
  },
  {
    "serial": "2111D39B",
    "session_id": 4596393370,
    "expected_rows": 20647,
    "rows": 20647,
    "missing_count": 0,
    "min_seq": 0,
    "max_seq": 20646
  },
  {
    "serial": "21123CB8",
    "session_id": 3597489004,
    "expected_rows": 23946,
    "rows": 23946,
    "missing_count": 0,
    "min_seq": 0,
    "max_seq": 23945
  }
]
```

Those three summaries mean three probe sessions were exported, every sequence from `0` through `max_seq` was present, and no expected row was missing.

In the captured example dump, the generated CSVs were large because they contain one line per sample plus eight metadata/header lines:

```sh
$ wc -l /tmp/cpt-example-dump/ProbeData_*_cloud.csv
   20658 /tmp/cpt-example-dump/ProbeData_2111009D_2626918122_cloud.csv
   20655 /tmp/cpt-example-dump/ProbeData_2111D39B_4596393370_cloud.csv
   23954 /tmp/cpt-example-dump/ProbeData_21123CB8_3597489004_cloud.csv
   65267 total
```

Here is an anonymized excerpt from the first CSV. Only probe serials and session IDs were replaced; the shown timestamps, sequence numbers, temperatures, and CSV shape are from the real dump. `Timestamp` is seconds since the start of that session. `SequenceNumber` increases by one each sample. `Sample Period` is milliseconds, so `5000` means samples arrive every five seconds. `T1` through `T8` are physical probe sensors; the virtual columns are Combustion's derived core, surface, and ambient values. The prediction columns are Combustion's prediction fields; in this excerpt they are mostly unset/default values, so the temperature, timestamp, and sequence columns are the useful parts.

```csv
Combustion Inc. Probe Data
Source: Combustion cloud API chunked dump
CSV version: 4
Probe S/N: 2111009D
Sample Period: 5000
Created: 2026-06-01T16:58:23.287327Z

Timestamp,SessionID,SequenceNumber,T1,T2,T3,T4,T5,T6,T7,T8,VirtualCoreTemperature,VirtualSurfaceTemperature,VirtualAmbientTemperature,EstimatedCoreTemperature,PredictionSetPoint,VirtualCoreSensor,VirtualSurfaceSensor,VirtualAmbientSensor,PredictionState,PredictionMode,PredictionType,PredictionValueSeconds
0.000,2626918122,0,23.95,23.80,23.75,23.75,24.05,23.80,23.75,23.70,23.95,23.80,23.70,23.90,0.00,0,2,3,0,0,0,131071
5.000,2626918122,1,23.90,23.85,23.75,23.75,24.00,23.80,23.80,23.65,23.90,23.80,23.65,23.90,0.00,0,2,3,0,0,0,131071
10.000,2626918122,2,23.90,23.85,23.75,23.75,24.05,23.85,23.80,23.70,23.90,23.85,23.70,23.90,0.00,0,2,3,0,0,0,131071
...
103240.000,2626918122,20648,23.55,23.70,23.80,24.15,25.45,25.60,25.55,25.05,23.55,24.15,25.60,23.50,0.00,0,0,1,2,0,0,131071
103245.000,2626918122,20649,23.60,23.70,23.90,24.35,25.60,25.85,25.90,25.25,23.60,24.35,25.90,23.50,0.00,0,0,2,2,0,0,131071
```

The first few rows show the probe near room temperature at the start of the session. The tail rows show the last exported samples. An agent can use the CSV or JSON rows to answer questions like "is the cook still heating," "how fast is the core changing," or "what happened in the last 20 minutes."

The other two probe CSVs have the same columns and matched the row counts shown in `summary.json`.

## Data Boundary

`cpt-crawl` exports the cook data it has actually proven how to retrieve: session lists, session metadata, time-series temperature samples, and CSVs made from those samples.

It does not invent cook names, food labels, notes, recipes, or app UI fields that did not come back from the cloud APIs. If a future app version exposes more metadata, the right behavior is to preserve and model those real fields, not guess them from temperatures or timing.

The implemented cloud path is:

- session index fields from `v1/sessions`, including the raw source object
- session metadata from `v1/session`
- time-series rows from `v1/session_data`
- CSV rows derived from those time-series samples

`dump-history` preserves the returned session index and dumps each unique retrievable session ID from that index. It exits nonzero if the session index is larger than the current safety guard or expected sequence rows are missing.

Endpoint coverage is limited to `v1/sessions`, `v1/session`, `v1/session_data`, and the Firestore user/probe documents used to find your probes. Raw `v1/sessions` entries are preserved under `source`, and raw `v1/session` metadata is written as `session.json`. Sample rows include the temperature and prediction fields modeled by the current Go structs; unknown future sample fields may need a code update before they appear in JSON or CSV output.

## Architecture

See [docs/architecture.md](docs/architecture.md).

Short version:

1. decrypt local auth bundle
2. refresh Firebase `idToken`
3. read Firestore user/probe metadata
4. call Combustion `v1/sessions`, `v1/session`, and `v1/session_data`
5. split `session_data` ranges into 1000-row chunks and verify completeness

## Safety

- This is unofficial and not affiliated with Combustion Inc.
- Exported cook data can include probe serials, timestamps, and account-linked IDs.
- `outputs/` is ignored because real dumps are private.
- Do not paste refresh tokens, full auth callback URLs containing `code`/`state`, auth bundles, or raw dumps into GitHub issues.

## Development

With Go on your `PATH`:

```sh
go test ./...
go build -o /tmp/cpt-crawl ./cmd/cpt-crawl
```

With the repo devenv:

```sh
devenv shell go test ./...
devenv shell go build -o /tmp/cpt-crawl ./cmd/cpt-crawl
```
