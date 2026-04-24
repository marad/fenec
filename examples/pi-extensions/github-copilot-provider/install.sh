#!/bin/bash

# GitHub Copilot Provider dla Pi - Simple Install Script
# Automatycznie instaluje extensję i konfiguruje Pi

set -e  # Exit on error

echo "🚀 GitHub Copilot Provider - Instalacja"
echo "========================================"
echo ""

# Step 1: Check GitHub CLI
echo "📋 Sprawdzanie wymagań..."
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) nie zainstalowany!"
    echo ""
    echo "   Zainstaluj go:"
    echo "   • macOS:   brew install gh"
    echo "   • Linux:   apt-get install gh  (lub apt install gh)"
    echo "   • Windows: choco install gh"
    echo ""
    exit 1
fi

if ! gh auth status &> /dev/null; then
    echo "❌ GitHub CLI nie zalogowany!"
    echo ""
    echo "   Zaloguj się:"
    echo "   gh auth login"
    echo ""
    exit 1
fi

echo "✅ GitHub CLI zainstalowany i zalogowany"
echo ""

# Step 2: Create extension directory
echo "📁 Tworzenie katalogów..."
EXT_DIR="$HOME/.pi/extensions/github-copilot-provider"
mkdir -p "$EXT_DIR"
echo "✅ Katalog: $EXT_DIR"
echo ""

# Step 3: Copy files
echo "📦 Kopiowanie plików extensji..."
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

cp "$SCRIPT_DIR/index.ts" "$EXT_DIR/"
cp "$SCRIPT_DIR/package.json" "$EXT_DIR/"
cp "$SCRIPT_DIR/tsconfig.json" "$EXT_DIR/"
cp "$SCRIPT_DIR/.gitignore" "$EXT_DIR/"

echo "✅ Pliki skopiowane"
echo ""

# Step 4: Install dependencies
echo "📥 Instalowanie zależności npm (może potrwać chwilę)..."
cd "$EXT_DIR"
npm install --legacy-peer-deps > /dev/null 2>&1 || npm install > /dev/null 2>&1
echo "✅ Zależności zainstalowane"
echo ""

# Step 5: Create/update Pi config
echo "⚙️  Aktualizowanie konfiguracji Pi..."
CONFIG_DIR="$HOME/.pi"
CONFIG_FILE="$CONFIG_DIR/config.json"

mkdir -p "$CONFIG_DIR"

if [ ! -f "$CONFIG_FILE" ]; then
    # Create new config
    cat > "$CONFIG_FILE" << 'EOF'
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
EOF
    echo "✅ Nowa konfiguracja utworzona: $CONFIG_FILE"
else
    # Check if extension is already configured
    if grep -q "github-copilot-provider" "$CONFIG_FILE"; then
        echo "✅ Extensja już skonfigurowana w $CONFIG_FILE"
    else
        echo "⚠️  $CONFIG_FILE już istnieje"
        echo ""
        echo "   Proszę ręcznie dodać do sekcji 'extensions':"
        echo "   {"
        echo "     \"name\": \"github-copilot-provider\","
        echo "     \"path\": \"~/.pi/extensions/github-copilot-provider\""
        echo "   }"
        echo ""
    fi
fi
echo ""

# Step 6: Verify installation
echo "🔍 Weryfikacja instalacji..."
cd "$EXT_DIR"
if npm run check > /dev/null 2>&1; then
    echo "✅ TypeScript - OK"
else
    echo "⚠️  TypeScript - błąd składni (spróbuj: npm run check)"
fi
echo ""

# Success message
echo "✅ INSTALACJA ZAKOŃCZONA!"
echo "========================================"
echo ""
echo "🎉 Następne kroki:"
echo ""
echo "1. Uruchom Pi:"
echo "   pi"
echo ""
echo "2. Powinieneś zobaczyć:"
echo "   ✅ GitHub Copilot Provider zarejestrowany!"
echo "   📦 Dostępne modele:"
echo ""
echo "3. Zacznij používać:"
echo "   pi"
echo "   > Cześć! Jaki masz model?"
echo "   /use-model openai/gpt-4o-mini"
echo "   > Wyjaśnij jak działa JavaScript async/await"
echo ""
echo "📚 Dokumentacja:"
echo "   - README:          ~/.pi/extensions/github-copilot-provider/README.md"
echo "   - Pi Extensions:   https://github.com/badlogic/pi-mono/blob/main/docs/extensions.md"
echo "   - GitHub Models:   https://github.com/marketplace/models"
echo ""
