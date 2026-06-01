# North Star

> Authorship: agent-authored project framing.

Primary user: a local operator or coding agent that needs reliable access to the operator's own Combustion CPT data without opening the mobile app.

## Goals

1. Dump current probe sessions completely, with row completeness checks.
2. Expose recent rows and new-sample monitoring output as JSON.
3. Dump each unique retrievable session returned by the paginated cloud session index, and fail if the index exceeds the current page guard.
4. Preserve raw session-index objects and raw session metadata before analysis.
5. Keep auth material and cook dumps outside the repo.

## Non-goals

- BLE pairing.
- Probe firmware control.
- Adding fields that the API did not return.
- Storing plaintext Firebase tokens, Apple handler URLs, or real cook dumps.

## Acceptance

- Given a valid local auth bundle, when `cpt-crawl dump` runs, then each selected probe gets raw JSON, CSV, and a summary with missing row counts.
- Given an active cook, when `cpt-crawl window --minutes N` runs, then stdout contains recent rows.
- Given an active cook, when `cpt-crawl watch` runs, then stdout receives JSON Lines for new samples without reprinting the whole recent window.
- Given older cooks exist in the discovered cloud index, when `cpt-crawl dump-history` runs, then each unique retrievable indexed session gets JSON and CSV output, and session index entries preserve their raw source object.
- Given any expected row is missing, when a dump command completes, then it exits nonzero.
