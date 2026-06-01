# Security

> Authorship: agent-authored security guidance.

Do not open public issues or pull requests containing:

- Firebase refresh tokens or id tokens
- Apple/Firebase callback URLs containing `code`/`state`, or auth codes
- plaintext auth bundles
- real cook dumps
- probe serial numbers tied to a person
- account-linked IDs from exported JSON

Use GitHub private vulnerability reporting if available, or contact the repository owner privately.

The Firebase Web API key used by the official app is app-public configuration, not a user secret. This repository still avoids storing the literal value because automated scanners treat it as a possible Google API key.
