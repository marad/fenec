# GitHub Copilot Provider dla Pi Agent

Extensja integruje GitHub Models API z agentem Pi. Zapewnia dostęp do ponad 40 modeli AI, w tym:
- **OpenAI**: GPT-4o, GPT-4o-mini, GPT-4.1
- **Meta**: Llama 3.3 70B
- **Mistral**, **DeepSeek**, **Qwen** i inne

## ⚡ Quick Start

### 1. Instalacja GitHub CLI (jeśli jeszcze nie masz)

```bash
# macOS
brew install gh

# Linux (Debian/Ubuntu)
sudo apt-get install gh

# Windows
choco install gh

# Zaloguj się
gh auth login
```

Sprawdź status:
```bash
gh auth status
```

### 2. Skopiuj extensję do Pi

```bash
# Utwórz katalog extensji
mkdir -p ~/.pi/extensions

# Skopiuj tę extensję
cp -r . ~/.pi/extensions/github-copilot-provider

# Zainstaluj zależności
cd ~/.pi/extensions/github-copilot-provider
npm install
```

### 3. Dodaj do konfiguracji Pi

Edytuj `~/.pi/config.json` i dodaj:

```json
{
  "extensions": [
    {
      "name": "github-copilot-provider",
      "path": "~/.pi/extensions/github-copilot-provider"
    }
  ],
  "defaultProvider": "github-copilot",
  "defaultModel": "openai/gpt-4o-mini"
}
```

### 4. Uruchom Pi

```bash
pi

# Powinnaś zobaczyć:
# ✅ GitHub Copilot Provider zarejestrowany!
# 📦 Dostępne modele:
#    OPENAI
#      - gpt-4o (131K tokens)
#      - gpt-4o-mini (131K tokens)
#    META
#      - llama-3.3-70b-instruct (131K tokens)
```

## 🎯 Przykłady użycia

### W interaktywnym czacie:

```bash
pi

> /use-model openai/gpt-4o-mini
> Wyjaśnij jak działa JavaScript async/await
```

### Zmiana modelu w runtime:

```
/use-model openai/gpt-4o          # Bardziej zdolny, wolniejszy
/use-model openai/gpt-4o-mini     # Szybszy, darmowy
/use-model meta/llama-3.3-70b-instruct  # Open source
```

### Lista dostępnych modeli:

```
pi --list-models
```

### Ze zmienną środowiska (zamiast gh CLI):

```bash
export GH_TOKEN="ghp_your_token_here"
pi
```

## 🔑 Autentykacja

Extensja automatycznie pobiera token GitHub w tej kolejności:

1. **`GH_TOKEN`** - zmienna środowiska
2. **`GITHUB_TOKEN`** - zmienna środowiska (GitHub Actions itp.)
3. **`gh auth token`** - z GitHub CLI (system keyring)

### Którą metodę wybrać?

| Metoda | Przypadek użycia | Bezpieczeństwo |
|--------|------------------|----------------|
| `gh auth token` | Rozwój lokalny | ✅ Excellent (keyring) |
| `GH_TOKEN` | CI/CD, Docker | ⚠️ Okej (env var) |
| `GITHUB_TOKEN` | GitHub Actions | ✅ Excellent (CI secret) |

## 📚 Dostępne modele

Pełna lista modeli na: https://github.com/marketplace/models

### Polecane dla różnych zadań:

| Zadanie | Model | Dlaczego |
|---------|-------|---------|
| **Szybkie odpowiedzi** | `openai/gpt-4o-mini` | Szybko + darmowy tier |
| **Skomplikowana analiza** | `openai/gpt-4o` | Najzdolniejszy |
| **Duże kontekst (1M tokens)** | `openai/gpt-4.1` | Ogromne okno kontekstu |
| **Open Source** | `meta/llama-3.3-70b-instruct` | Brak kosztów, pełna prywatność |
| **Code generation** | `openai/gpt-4o` | Best code quality |

## 🚨 Rozwiązywanie problemów

### Błąd: "GitHub CLI is not installed"

```bash
# Zainstaluj
brew install gh  # macOS
apt-get install gh  # Linux

# Zaloguj się
gh auth login
gh auth status  # Sprawdź
```

### Błąd: "401 Unauthorized"

Sprawdzenie:
```bash
# Czy token jest prawidłowy?
gh auth status

# Czy token się nie zmienił?
gh auth refresh

# Czy konto ma dostęp do Models?
# Sprawdź na https://github.com/marketplace/models
```

Rozwiązanie:
```bash
# Odśwież autentykację
gh auth logout
gh auth login
```

### Błąd: "Unknown model"

```bash
# Lista dostępnych modeli
pi --list-models

# Lub sprawdź online
# https://github.com/marketplace/models
```

### Extensja się nie ładuje

```bash
# 1. Sprawdź składnię TypeScript
cd ~/.pi/extensions/github-copilot-provider
npm run check

# 2. Sprawdź ścieżkę w config.json
# Powinna być pełna: /Users/username/.pi/extensions/github-copilot-provider
# Lub z ~: ~/.pi/extensions/github-copilot-provider

# 3. Przeładuj agenta
pi /reload
```

### Timeout pobierania modeli

Jeśli pobieranie katalogu trwa zbyt długo:

```bash
# Sprawdź połączenie sieciowe
curl -I https://models.github.ai/v1/models

# Lub użyj z timeoutem
export GH_TOKEN="..." && timeout 30 pi
```

## 🔧 Konfiguracja zaawansowana

### Niestandardowy endpoint (dla enterprise)

Edytuj `index.ts`:

```typescript
pi.registerProvider("github-copilot", {
  baseUrl: "https://your-enterprise.github.ai/inference",
  apiKey: token,
  api: "openai-completions",
  models: piModels,
  headers: {
    "X-Enterprise-Auth": "your-custom-header"
  }
});
```

### Cachowanie modeli (offline)

Domyślnie modele są cachowane w pamięci. Aby wymusić odświeżenie:

```
pi /reload
```

## 📖 Dokumentacja

### W tym repozytorium (Fenec)
- [GitHub Copilot Provider - implementacja Go](https://github.com/marad/fenec/tree/main/internal/provider/copilot)
- [Architektura autentykacji](https://github.com/marad/fenec/blob/main/.planning/research/ARCHITECTURE-copilot-auth.md)

### Pi Agent
- [Custom Providers - docs](https://github.com/badlogic/pi-mono/blob/main/docs/custom-provider.md)
- [Extensions SDK](https://github.com/badlogic/pi-mono/blob/main/docs/extensions.md)
- [Config Reference](https://github.com/badlogic/pi-mono/blob/main/docs)

### GitHub Models
- [GitHub Models Marketplace](https://github.com/marketplace/models)
- [GitHub Models Docs](https://docs.github.com/en/github-models)

## 🤝 Wkład i wsparcie

- 🐛 Problemy z extensją: [Fenec Issues](https://github.com/marad/fenec/issues)
- 💬 Pytania o Pi: [Pi Discussions](https://github.com/badlogic/pi-mono/discussions)
- 📚 Problemy z Models: [GitHub Models Feedback](https://github.com/orgs/github-community/discussions)

## 📄 Licencja

MIT - patrz LICENSE

---

**Utworzono:** 2026-04-24  
**Status:** Production ready  
**Testowane z:** Pi Agent v1.x, Node.js 18+, macOS/Linux/Windows
