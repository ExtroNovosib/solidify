# Security Policy

## Supported versions

The latest published v0.x release and the current default branch receive security fixes. Older v0.x releases should upgrade to the latest patch before reporting version-specific behavior.

## Reporting a vulnerability

Use GitHub private vulnerability reporting for this repository. Do not open a public issue containing exploit details, sensitive source, or credentials. Include affected versions, reproduction steps, impact, and a suggested mitigation when possible.

Maintainers will acknowledge a report, coordinate validation and remediation privately, and agree on disclosure timing with the reporter. A release is blocked when `govulncheck ./...` identifies a reachable vulnerability. ABI-overlapping GolangCI dependencies are upgraded together with the pinned GolangCI target.
