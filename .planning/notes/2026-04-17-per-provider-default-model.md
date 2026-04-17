---
date: "2026-04-17 18:09"
promoted: false
---

Per-provider default_model (ProviderConfig.DefaultModel in toml.go) is now wired through ProviderRegistry: SetDefaultModel/DefaultModelFor + used in config.ResolveModel. Previously this TOML field existed but was unread. Ported from the abandoned e975f4d branch (v1.1 retrospectively-duplicated commits) after discarding via rebase abort + reset to b6cd4a4. Resolver extracted to internal/config/resolve.go with full unit test coverage.
