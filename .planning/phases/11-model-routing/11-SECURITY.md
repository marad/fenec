---
phase: 11
slug: model-routing
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-14
---

# Phase 11 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CLI → ProviderRegistry | User-supplied `--model` and `/model` args resolve against registry | Provider name, model name (strings) |
| REPL → Provider API | GetContextLength and ListModels calls to local daemon or remote API | Model metadata (read-only) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-11-01 | I — Information Disclosure | `main.go` stderr, `handleModelCommand` stdout | Accept | Provider names are user-configured, not sensitive; personal tool | closed |
| T-11-02 | T — Tampering | `handleModelCommand` (`strings.SplitN`) | Accept | APIs validate model names server-side; no shell execution | closed |
| T-11-03 | D — DoS (local) | `handleModelCommand` `GetContextLength` calls | Mitigate | Added 5s `context.WithTimeout` to both branches (mirrors `listModels` convention) | closed |
| T-11-04 | D — DoS (goroutines) | `listModels()` | Mitigate | 5-second `context.WithTimeout` + `sync.WaitGroup` in place from implementation | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-11-01 | T-11-01 | Provider names are user-configured strings in `~/.config/fenec/config.toml`. The user owns this file. Exposing them in error messages is intentional for UX. | audit | 2026-04-14 |
| AR-11-02 | T-11-02 | Model names are CLI args passed as JSON strings to provider APIs. No shell execution; providers handle validation. `strings.SplitN` with n=2 is safe. | audit | 2026-04-14 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-14 | 4 | 4 | 0 | gsd-security-auditor (automated) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-14
