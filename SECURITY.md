# Security Policy

## Supported versions

Until v0.1.0 is published, only the current default branch receives security fixes. After release, the latest v0.x release is supported.

## Reporting a vulnerability

Use GitHub private vulnerability reporting for this repository. Do not open a public issue containing exploit details, sensitive source, or credentials. Include affected versions, reproduction steps, impact, and a suggested mitigation when possible.

Maintainers will acknowledge a report, coordinate validation and remediation privately, and agree on disclosure timing with the reporter. A release is blocked when `govulncheck ./...` identifies a reachable vulnerability. ABI-overlapping GolangCI dependencies are upgraded together with the pinned GolangCI target.
