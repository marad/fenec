import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";
import axios from "axios";
import { execSync } from "child_process";

/**
 * GitHub Copilot Provider for Pi
 *
 * Integrates GitHub Models API (https://models.github.ai) into Pi agent.
 * Automatically resolves GitHub token from:
 * 1. GH_TOKEN env var
 * 2. GITHUB_TOKEN env var
 * 3. gh CLI keyring (via `gh auth token`)
 */

interface GitHubModel {
  id: string;
  name: string;
  capabilities: string[];
  limits: {
    max_input_tokens: number;
    max_output_tokens: number;
  };
  rate_limit_tier: string;
}

interface ModelsResponse {
  data: GitHubModel[];
}

/**
 * Resolves GitHub authentication token.
 * Priority: GH_TOKEN > GITHUB_TOKEN > gh CLI
 */
async function resolveGitHubToken(): Promise<string> {
  // Priority 1: GH_TOKEN (matches gh CLI behavior)
  if (process.env.GH_TOKEN) {
    console.log("📌 Używam token z GH_TOKEN");
    return process.env.GH_TOKEN;
  }

  // Priority 2: GITHUB_TOKEN (CI/CD environments)
  if (process.env.GITHUB_TOKEN) {
    console.log("📌 Używam token z GITHUB_TOKEN");
    return process.env.GITHUB_TOKEN;
  }

  // Priority 3: gh CLI keyring
  console.log("📌 Pobieranie tokenu z GitHub CLI...");
  try {
    const output = execSync("gh auth token --hostname github.com", {
      encoding: "utf-8",
      stdio: ["pipe", "pipe", "pipe"],
    });
    const token = output.trim();
    if (!token) {
      throw new Error("Token był pusty");
    }
    console.log("✅ Token pobrano z GitHub CLI");
    return token;
  } catch (error) {
    const message =
      error instanceof Error ? error.message : String(error);
    throw new Error(
      `Nie można pobrać tokenu GitHub.\n\n` +
      `Upewnij się, że GitHub CLI (gh) jest zainstalowany i zalogowany:\n` +
      `  1. brew install gh        (macOS)\n` +
      `  2. gh auth login          (zaloguj się)\n` +
      `  3. gh auth status         (sprawdź status)\n\n` +
      `Alternatywnie ustaw zmienną środowiska:\n` +
      `  export GH_TOKEN="ghp_..."\n\n` +
      `Szczegóły błędu: ${message}`
    );
  }
}

/**
 * Fetches available models from GitHub Models API catalog.
 */
async function fetchGitHubModels(token: string): Promise<GitHubModel[]> {
  try {
    console.log("📚 Pobieranie katalogu modeli z GitHub Models API...");
    const response = await axios.get<ModelsResponse>(
      "https://models.github.ai/v1/models",
      {
        headers: {
          Authorization: `Bearer ${token}`,
        },
        timeout: 10000,
      }
    );

    console.log(`✅ Pobrano ${response.data.data.length} modeli`);
    return response.data.data;
  } catch (error) {
    if (axios.isAxiosError(error)) {
      if (error.response?.status === 401) {
        throw new Error(
          "Autoryzacja nie powiodła się (401). Sprawdź czy Twój token GitHub jest prawidłowy."
        );
      }
      if (error.response?.status === 404) {
        throw new Error(
          "GitHub Models API niedostępne (404). Sprawdź czy używasz prawidłowego hosta."
        );
      }
    }
    const message =
      error instanceof Error ? error.message : String(error);
    throw new Error(`Błąd pobierania modeli: ${message}`);
  }
}

/**
 * Extension factory - registers GitHub Copilot provider with Pi
 */
export default async function (pi: ExtensionAPI) {
  try {
    console.log("\n🔧 Inicjalizacja GitHub Copilot Provider...\n");

    // Step 1: Resolve GitHub token
    const token = await resolveGitHubToken();

    // Step 2: Fetch available models
    const models = await fetchGitHubModels(token);

    if (models.length === 0) {
      throw new Error(
        "GitHub Models API zwrócił pustą listę. Sprawdź dostęp do API."
      );
    }

    // Step 3: Map models to Pi format
    const piModels = models.map((model) => ({
      id: model.id,
      name: model.name || model.id,
      reasoning: false,
      input: ["text", "image"] as const,
      cost: {
        input: 0,
        output: 0,
        cacheRead: 0,
        cacheWrite: 0,
      },
      contextWindow: model.limits.max_input_tokens || 131072,
      maxTokens: model.limits.max_output_tokens || 4096,
    }));

    // Step 4: Register provider
    pi.registerProvider("github-copilot", {
      baseUrl: "https://models.github.ai/inference",
      apiKey: token,
      api: "openai-completions",
      models: piModels,
    });

    // Summary
    console.log("\n✅ GitHub Copilot Provider zarejestrowany!\n");
    console.log("📦 Dostępne modele:\n");

    // Group models by publisher
    const byPublisher = new Map<string, (typeof piModels)[0][]>();
    piModels.forEach((m) => {
      const publisher = m.id.split("/")[0];
      if (!byPublisher.has(publisher)) {
        byPublisher.set(publisher, []);
      }
      byPublisher.get(publisher)!.push(m);
    });

    // Display in organized format
    byPublisher.forEach((models, publisher) => {
      console.log(`   ${publisher.toUpperCase()}`);
      models.forEach((m) => {
        const contextK = Math.round(m.contextWindow / 1000);
        console.log(`     - ${m.id.split("/")[1]} (${contextK}K tokens)`);
      });
    });

    console.log(
      `\n💡 Wskazówka: Ustaw domyślny model za pomocą '/use-model openai/gpt-4o-mini'\n`
    );
  } catch (error) {
    const message =
      error instanceof Error ? error.message : String(error);
    console.error(`\n❌ Błąd podczas inicjalizacji:\n${message}\n`);
    throw error;
  }
}
