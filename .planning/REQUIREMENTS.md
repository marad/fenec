# Requirements: Fenec

**Defined:** 2026-04-12
**Core Value:** An extensible AI agent platform that can grow its own capabilities through self-authored Lua tools.

## v1.1 Requirements

Requirements for multi-provider support. Each maps to roadmap phases.

### Provider Abstraction

- [ ] **PROV-01**: User can chat with Fenec using any configured provider without knowing the underlying protocol
- [ ] **PROV-02**: User experiences identical tool calling behavior regardless of which provider is active
- [ ] **PROV-03**: User's existing Ollama workflow works exactly as before with zero configuration changes

### OpenAI-Compatible Support

- [ ] **OAIC-01**: User can chat with models served by LM Studio via the OpenAI-compatible protocol
- [ ] **OAIC-02**: User can chat with OpenAI cloud models (GPT-4o, etc.) via the OpenAI API
- [ ] **OAIC-03**: User can use tool calling with OpenAI-compatible providers (non-streaming when tools present)
- [ ] **OAIC-04**: User can switch providers mid-session and continue the conversation

### Configuration

- [ ] **CONF-01**: User can define providers in a TOML config file with name, type, URL, and API key
- [ ] **CONF-02**: User can reference environment variables for API keys in config (e.g., `$OPENAI_API_KEY`)
- [ ] **CONF-03**: User can run Fenec with no config file and get the default Ollama provider automatically
- [ ] **CONF-04**: User can modify provider config and have changes take effect without restarting Fenec

### Model Routing

- [ ] **ROUT-01**: User can select a model with `--model provider/model` to target a specific provider
- [ ] **ROUT-02**: User can use `--model modelname` (no prefix) to use the default provider
- [ ] **ROUT-03**: User can list available models grouped by provider via `/model`
- [ ] **ROUT-04**: User can discover models from each provider automatically (fetched from provider APIs)

## Future Requirements

Deferred to future milestone. Tracked but not in current roadmap.

### Provider Ecosystem

- **PECO-01**: Provider-specific feature negotiation (auto-detect thinking, streaming+tools support)
- **PECO-02**: Provider health dashboard in REPL

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Anthropic/Google-specific adapters | OpenAI-compatible format covers 90% of providers; per-provider adapters add maintenance burden |
| LangChain-style provider chaining/fallback | Infrastructure for production services, not a personal CLI agent |
| API key management UI / keychain integration | Over-engineered for personal tool; env var references are standard |
| Provider-specific parameter tuning | Normalizing temperature/top_p across providers is a rabbit hole; pass through as-is |
| OpenAI Responses API | OpenAI-specific, not an interop standard; Chat Completions is the universal target |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PROV-01 | — | Pending |
| PROV-02 | — | Pending |
| PROV-03 | — | Pending |
| OAIC-01 | — | Pending |
| OAIC-02 | — | Pending |
| OAIC-03 | — | Pending |
| OAIC-04 | — | Pending |
| CONF-01 | — | Pending |
| CONF-02 | — | Pending |
| CONF-03 | — | Pending |
| CONF-04 | — | Pending |
| ROUT-01 | — | Pending |
| ROUT-02 | — | Pending |
| ROUT-03 | — | Pending |
| ROUT-04 | — | Pending |

**Coverage:**
- v1.1 requirements: 14 total
- Mapped to phases: 0
- Unmapped: 14

---
*Requirements defined: 2026-04-12*
*Last updated: 2026-04-12 after initial definition*
