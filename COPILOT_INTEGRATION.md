# Integracja GitHub Copilot w Pi Agent

Instrukcja pokazuje jak zamontować GitHub Copilot providera w swoim agencie opartym na Pi.

## Co to jest GitHub Copilot Provider?

GitHub Copilot Provider to custom provider dla Pi, który łączy się z GitHub Models API (`https://models.github.ai`). Daje dostęp do:
- **GPT-4o** i **GPT-4o-mini** (OpenAI)
- **Llama 3.3 70B** (Meta)
- **Mistral** i innych modeli
- **Tool calling** - obsługa funkcji i narzędzi
- **Streaming** - odpowiedzi w czasie rzeczywistym

Wszystko zintegrowane z Twoją istniejącą autentykacją GitHub CLI (`gh`).

## 1. Wymagania wstępne

1. **GitHub CLI (`gh`)** zainstalowany i zalogowany:
   ```bash
   brew install gh  # macOS
   # lub: apt install gh (Linux), choco install gh (Windows)
   gh auth login
   ```

2. **Pi agent** zainstalowany:
   ```bash
   npm install -g @mariozechner/pi-coding-agent
   ```

3. **Node.js** w wersji 18+

## 2. Utworzenie extensji z GitHub Copilot providerem

### Krok 1: Przygotuj katalog extensji

```bash
mkdir -p ~/.pi/extensions/github-copilot-provider
cd ~/.pi/extensions/github-copilot-provider
npm init -y
npm install axios  # do komunikacji HTTP
```

### Krok 2: Utwórz plik `index.ts` extensji

```typescript
import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";
import axios from "axios";
import { execSync } from "child_process";
import * as fs from "fs";
import * as path from "path";

// Pobierz token GitHub z tych źródeł w kolejności:
// 1. GH_TOKEN env var
// 2. GITHUB_TOKEN env var
// 3. gh auth token (z GitHub CLI)
async function resolveGitHubToken(): Promise<string> {
  // Priorytet 1: GH_TOKEN
  if (process.env.GH_TOKEN) {
    return process.env.GH_TOKEN;
  }

  // Priorytet 2: GITHUB_TOKEN (GitHub Actions itp.)
  if (process.env.GITHUB_TOKEN) {
    return process.env.GITHUB_TOKEN;
  }

  // Priorytet 3: gh CLI keyring
  try {
    const output = execSync("gh auth token --hostname github.com", {
      encoding: "utf-8",
      stdio: ["pipe", "pipe", "pipe"],
    });
    const token = output.trim();
    if (!token) {
      throw new Error("Token był pusty");
    }
    return token;
  } catch (error) {
    throw new Error(
      `Nie można pobrać tokenu GitHub. Upewnij się, że GitHub CLI (gh) jest zainstalowany i zalogowany:\n` +
      `  gh auth login\n\n` +
      `Alternatywnie ustaw zmienną środowiska GH_TOKEN lub GITHUB_TOKEN.`
    );
  }
}

// Pobierz listę dostępnych modeli z GitHub Models API
async function fetchGitHubModels(token: string): Promise<Array<{
  id: string;
  name: string;
  limits: { max_input_tokens: number; max_output_tokens: number };
}>> {
  const response = await axios.get("https://models.github.ai/v1/models", {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  return response.data.data;
}

export default async function (pi: ExtensionAPI) {
  try {
    // Pobierz token GitHub
    const token = await resolveGitHubToken();

    // Pobierz katalog modeli
    const models = await fetchGitHubModels(token);

    // Zamapuj modele do formatu Pi
    const piModels = models.map((model) => ({
      id: model.id,
      name: model.name || model.id,
      reasoning: false,
      input: ["text", "image"],
      cost: {
        input: 0,
        output: 0,
        cacheRead: 0,
        cacheWrite: 0,
      },
      contextWindow: model.limits.max_input_tokens || 131072,
      maxTokens: model.limits.max_output_tokens || 4096,
    }));

    console.log(`📦 Zarejestrowano ${piModels.length} modeli z GitHub Models API`);

    // Zarejestruj providera w Pi
    pi.registerProvider("github-copilot", {
      baseUrl: "https://models.github.ai/inference",
      apiKey: token,
      api: "openai-completions",
      models: piModels,
    });

    console.log(
      `✅ GitHub Copilot provider zarejestowany! Dostępne modele:\n` +
      piModels.map((m) => `   - ${m.id} (${m.contextWindow} tokens)`).join("\n")
    );
  } catch (error) {
    const message =
      error instanceof Error ? error.message : String(error);
    console.error(`❌ Błąd podczas rejestracji GitHub Copilot providera:\n${message}`);
    throw error;
  }
}
```

### Krok 3: Skonfiguruj `package.json`

```json
{
  "name": "github-copilot-provider",
  "version": "1.0.0",
  "description": "GitHub Copilot provider dla Pi agent",
  "main": "index.ts",
  "type": "module",
  "dependencies": {
    "axios": "^1.6.0"
  }
}
```

## 3. Dodaj extensję do konfiguracji Pi

Edytuj plik `~/.pi/config.json` (lub utwórz jeśli nie istnieje):

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

## 4. Uruchomienie

Po konfiguracji extensji:

```bash
# Wyświetl dostępne modele
pi --list-models

# Sprawdź czy GitHub Copilot provider jest dostępny
pi --list-providers

# Zacznij używać
pi
```

## 5. Przykłady użycia

### W interaktywnym czacie Pi:

```bash
pi

> /use-model openai/gpt-4o
> Jakie są najlepsze praktyki w TypeScript?
```

### Zmiana modelu w runtime:

```
/use-model openai/gpt-4o
/use-model meta/llama-3.3-70b-instruct
```

### Listowanie dostępnych modeli:

```
pi --list-models
```

### Automatyzacja z zmienną środowiska:

```bash
GH_TOKEN="ghp_..." pi
```

## 6. Dostępne modele (stan na 2026-04-24)

| Model | Producent | Kontekst | Tool Calling | Status |
|-------|-----------|----------|--------------|--------|
| `openai/gpt-4o` | OpenAI | 131K tokens | ✅ | High tier |
| `openai/gpt-4o-mini` | OpenAI | 131K tokens | ✅ | Low tier (darmowy) |
| `openai/gpt-4.1` | OpenAI | 1M tokens | ✅ | High tier |
| `meta/llama-3.3-70b-instruct` | Meta | 131K tokens | ❌ | High tier |
| `mistral-ai/mistral-small-2503` | Mistral | 128K tokens | ✅ | Low tier |
| `deepseek/deepseek-r1` | DeepSeek | 128K tokens | ✅ | Custom tier |

**Polecam zacząć z:** `openai/gpt-4o-mini` - najlepsze ratio jakość/szybkość na darmowym tierze.

## 7. Rozwiązywanie problemów

### Błąd: "GitHub CLI is not installed"
```bash
# Zainstaluj GitHub CLI
brew install gh

# Zaloguj się
gh auth login
```

### Błąd: "Unauthorized" (401)
- Sprawdź czy `gh auth login` się powiódł: `gh auth status`
- Jeśli ustawiasz `GH_TOKEN`, sprawdź czy token jest prawidłowy
- Token musi zaczynać się od `ghp_`

### Błąd: "Unknown model"
- Uruchom `pi --list-models` by zobaczyć dostępne modele
- Upewnij się że Twoje konto GitHub ma dostęp do GitHub Models

### Extensja się nie ładuje
1. Sprawdź składnię TypeScript: `npx tsc --noEmit`
2. Sprawdź ścieżkę w `~/.pi/config.json` - powinna być bezwzględna lub z `~`
3. Przeładuj agenta: `pi /reload`

## 8. Linki do dokumentacji

### Repozytorium Fenec (implementacja Go)
- [GitHub Copilot Provider - implementacja](https://github.com/marad/fenec/tree/main/internal/provider/copilot)
- [Architektura autentykacji](https://github.com/marad/fenec/blob/main/.planning/research/ARCHITECTURE-copilot-auth.md)
- [Plik konfiguracyjny](https://github.com/marad/fenec/blob/main/main.go)

### Dokumentacja Pi
- [Custom Providers - przewodnik](https://github.com/badlogic/pi-mono/blob/main/docs/custom-provider.md)
- [Przykład: Anthropic Provider](https://github.com/badlogic/pi-mono/tree/main/examples/extensions/custom-provider-anthropic)
- [Przykład: GitLab Duo Provider](https://github.com/badlogic/pi-mono/tree/main/examples/extensions/custom-provider-gitlab-duo)
- [SDK Extensions API](https://github.com/badlogic/pi-mono/blob/main/docs/extensions.md)

### GitHub Models API
- [GitHub Models - strona startowa](https://github.com/marketplace/models)
- [Rate limits i tiers](https://docs.github.com/en/github-models/prototyping-with-ai-models)

## 9. Zaawansowana konfiguracja

### Użyj niestandardowego tokenu zamiast gh CLI

Jeśli nie chcesz używać GitHub CLI, możesz bezpośrednio przekazać token:

```bash
export GH_TOKEN="ghp_your_token_here"
pi
```

### Cachowanie modeli

Extensja cachuje listę modeli w pamięci. Aby wymusić odświeżenie, przeładuj agenta:

```
pi /reload
```

### Niestandardowa konfiguracja API

Jeśli prowadzisz enterprise GitHub, możesz zmienić URL:

```typescript
pi.registerProvider("github-copilot", {
  baseUrl: "https://your-enterprise.github.ai/inference",
  apiKey: token,
  api: "openai-completions",
  models: piModels,
  headers: {
    "X-Enterprise-Auth": "your-header"
  }
});
```

## 10. Wsparcie i feedback

- 🐛 Błędy w implementacji Go: [Fenec Issues](https://github.com/marad/fenec/issues)
- 💬 Pytania o Pi SDK: [Pi Documentation](https://github.com/badlogic/pi-mono/discussions)
- 📚 GitHub Models: [Models Feedback](https://github.com/orgs/github-community/discussions)

---

**Autor:** Instrukcja dla implementacji GitHub Copilot providera w Pi
**Data:** 2026-04-24
**Status:** Gotowe do użytku
