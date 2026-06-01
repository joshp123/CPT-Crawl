# Combustion Cloud Playbook

> Authorship: agent-authored reverse-engineering notes. Not official vendor documentation.

This is the low-level map behind `cpt-crawl`. Most users should start with the README.

## Auth Shape

Combustion uses Firebase Auth with Apple as an identity provider. The Firebase Web API key is app-public configuration, but it is not stored in this repo because public secret scanners flag it as a Google API key.

Runtime auth input is a local JSON bundle, preferably age-encrypted:

```json
{
  "apiKey": "APP_PUBLIC_FIREBASE_WEB_API_KEY",
  "localId": "FIREBASE_LOCAL_ID",
  "refreshToken": "FIREBASE_REFRESH_TOKEN",
  "email": "optional@example.com"
}
```

`cpt-crawl` refreshes the Firebase `idToken` from `refreshToken` and uses the short-lived `idToken` for Firestore and Combustion API requests.

Agent bootstrap commands for extracting the app-public Firebase key and exchanging Apple auth for Firebase tokens live in [.agents/auth-bootstrap.txt](../.agents/auth-bootstrap.txt).

## Discovery

- Firebase project: `combustion-production-apps`
- Firestore user key: uppercase UUIDv5 of Firebase `localId` using namespace `c6639a3c-0b0a-4dd9-8cc1-046a2da8a5f1`
- User document: `users/<USER_KEY>`
- Probe status: `probes/<DEVICE_KEY>/probe_status/current`

Probe associations come from the user document. Current session IDs come from probe status.

Metadata rule: preserve the exact raw objects returned by `v1/sessions` and `v1/session`. Do not add derived label fields that are not present in those responses.

## Data API

Base URL:

```text
https://data-api.combustion.inc
```

Headers:

```text
Authorization: Bearer <Firebase idToken>
Content-Type: application/json
CI-AppVersion: v3.2.4
CI-OSVersion: 35
CI-Locale: en-US
CI-DateTime: <current ISO timestamp>
```

Endpoints:

```text
GET /v1/sessions?uid=<localId>&device_serial_number=<serial>&device_type=1&page=<n>&page_size=100
GET /v1/session?uid=<localId>&device_serial_number=<serial>&device_type=1&device_session_id=<session_id>
GET /v1/session_data?uid=<localId>&device_serial_number=<serial>&device_type=1&device_session_id=<session_id>&sequence_number_ranges=<urlencoded [[start,end]]>&page=1
```

Important: split `sequence_number_ranges` into inclusive chunks of at most 1000 and request `page=1` for each chunk. The implemented path is chunked and then checked for missing sequence numbers.

## Commands

- `cpt-crawl sessions`: list indexed sessions per probe.
- `cpt-crawl dump`: dump current sessions.
- `cpt-crawl dump-history`: dump each unique retrievable session returned by the session index.
- `cpt-crawl window --minutes 5`: emit recent rows once.
- `cpt-crawl watch --interval 30s`: follow new samples as JSON Lines. It does not reprint the whole recent window.
