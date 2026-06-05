# Security Policy

SELECT is a database client and proxy: it holds connection strings, database
credentials, and SSH keys, and the backend dials user-supplied databases.
Responsible disclosure is appreciated.

## Reporting a vulnerability

**Do not open a public issue for vulnerabilities.** Report privately via
[**GitHub Security Advisories**](https://github.com/select-db/select/security/advisories/new)
("Report a vulnerability"); this stays private until a fix ships. If you can't
use advisories, contact the maintainer
([@Arthur-Mesplede](https://github.com/Arthur-Mesplede)) on GitHub and we'll open
a private channel.

Include: impact, reproduction (a minimal PoC if possible), affected component
(`app/` client, `backend/` proxy, `dialect/` engine) and commit, and any fix idea.

## Process

- Acknowledgement within 3 business days; initial severity assessment within 7.
- We ship a fix before public disclosure and credit you (if you wish) in the
  advisory and release notes.

## Scope

**In scope**: `app/` (desktop client), `backend/` (proxy), `dialect/` (engine),
and the release/update pipeline. Classes of interest: authn/authz bypass, tenant
isolation breaks, SSRF, credential disclosure, update/signature-integrity bypass,
injection, and path traversal.

**Out of scope**: third-party dependency CVEs (report upstream; tell us if we're
affected), issues requiring a compromised host or physical access, social
engineering of maintainers, and scanner output without demonstrated impact.

## Supported versions

Pre-1.0, rolling release. Fixes land on the latest release; reproduce against it
before reporting.
