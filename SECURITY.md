# Security policy

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository (Security → Report a vulnerability). Do not open a public issue for security problems.

Include the affected commit, reproduction steps, impact, and any suggested fix. Expect an initial response within a few days.

## Scope

This project generates FPGA firmware and accesses donor PCIe hardware. Reports about unsafe hardware access paths — unexpected MMIO reads or writes, missing safety gates, bypassable refusals — are treated as security issues.

Only the current `main` is supported; fixes land on `main` and in the next release.
