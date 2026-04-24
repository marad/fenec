package copilot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/marad/fenec/internal/model"
	"github.com/marad/fenec/internal/provider"
	openaiProvider "github.com/marad/fenec/internal/provider/openai"
	"github.com/openai/openai-go/v3/option"
)

const (
	// copilotBaseURL is the GitHub Copilot Premium API base URL.
	copilotBaseURL = "https://api.githubcopilot.com"

	// defaultModel is the recommended default model for the copilot provider.
	defaultModel = "gpt-4o"

	// tokenRefreshBuffer is the number of seconds before expiry at which
	// the session token is proactively refreshed.
	tokenRefreshBuffer int64 = 5 * 60
)

// Compile-time check: Provider satisfies provider.Provider.
var _ provider.Provider = (*Provider)(nil)

// Provider wraps openai.Provider with GitHub Copilot Premium authentication.
// It manages a short-lived Copilot session token, automatically refreshing it
// before expiry by exchanging the long-lived GitHub token.
type Provider struct {
	githubToken string
	session     *copilotSession
	inner       *openaiProvider.Provider
	mu          sync.Mutex

	// sessionURL can be overridden for testing. Defaults to sessionTokenURL.
	sessionURL string
}

// New creates a Provider using GitHub authentication from the environment or gh CLI.
// Token resolution order: GH_TOKEN env var > GITHUB_TOKEN env var > Copilot config files > gh auth token.
// The Copilot session token is fetched lazily on first API call.
func New() (*Provider, error) {
	token, err := resolveToken()
	if err != nil {
		return nil, fmt.Errorf("copilot: %w", err)
	}
	return &Provider{
		githubToken: token,
		sessionURL:  sessionTokenURL,
	}, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "copilot"
}

// DefaultModel returns the recommended default model for the copilot provider.
func (p *Provider) DefaultModel() string {
	return defaultModel
}

// ensureSession ensures a valid Copilot session token exists, refreshing if
// needed. Returns the inner openai.Provider configured with the current token.
//
// If the session token exchange returns 404 (token lacks copilot scope),
// automatically triggers the GitHub OAuth device flow to obtain a properly
// scoped token, stores it for future use, and retries.
func (p *Provider) ensureSession(ctx context.Context) (*openaiProvider.Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now().Unix()
	if p.session != nil && now < p.session.ExpiresAt-tokenRefreshBuffer && p.inner != nil {
		return p.inner, nil
	}

	session, err := fetchSessionTokenFrom(ctx, p.sessionURL, p.githubToken)
	if err != nil {
		// If 404, the token lacks copilot scope → try device flow.
		var notFound *errSessionNotFound
		if errors.As(err, &notFound) {
			slog.Info("copilot: token lacks copilot scope, starting device flow authentication")
			newToken, authErr := p.runDeviceFlow(ctx)
			if authErr != nil {
				return nil, fmt.Errorf("copilot: authentication failed: %w", authErr)
			}
			p.githubToken = newToken
			session, err = fetchSessionTokenFrom(ctx, p.sessionURL, p.githubToken)
			if err != nil {
				return nil, fmt.Errorf("copilot: session token exchange failed after device flow: %w", err)
			}
		} else {
			return nil, err
		}
	}

	inner, err := openaiProvider.New(copilotBaseURL, session.Token,
		option.WithHeader("Editor-Version", "vscode/1.107.0"),
		option.WithHeader("Editor-Plugin-Version", "copilot-chat/0.35.0"),
		option.WithHeader("Copilot-Integration-Id", "vscode-chat"),
	)
	if err != nil {
		return nil, fmt.Errorf("copilot: creating openai client: %w", err)
	}

	p.session = session
	p.inner = inner
	return inner, nil
}

// runDeviceFlow performs the GitHub OAuth device flow, stores the resulting token,
// and returns it. Must be called with p.mu held.
func (p *Provider) runDeviceFlow(ctx context.Context) (string, error) {
	token, err := DeviceFlowAuth(ctx, func(userCode, verificationURI string) {
		fmt.Fprintf(os.Stderr, "\n"+
			"╭─────────────────────────────────────────────────╮\n"+
			"│  GitHub Copilot Authentication Required         │\n"+
			"│                                                 │\n"+
			"│  1. Open: %-37s  │\n"+
			"│  2. Enter code: %-30s  │\n"+
			"│                                                 │\n"+
			"│  Waiting for authorization...                   │\n"+
			"╰─────────────────────────────────────────────────╯\n\n",
			verificationURI, userCode)
	})
	if err != nil {
		return "", err
	}

	// Store for future sessions.
	if storeErr := storeCopilotToken(token); storeErr != nil {
		slog.Warn("copilot: failed to store token (will need to re-authenticate)", "error", storeErr)
	} else {
		slog.Info("copilot: token stored in ~/.config/github-copilot/hosts.json")
	}

	fmt.Fprintf(os.Stderr, "✓ GitHub Copilot authenticated successfully!\n\n")
	return token, nil
}

// StreamChat delegates to the inner openai.Provider after ensuring a valid session.
func (p *Provider) StreamChat(ctx context.Context, req *provider.ChatRequest, onToken func(string), onThinking func(string)) (*model.Message, *model.StreamMetrics, error) {
	inner, err := p.ensureSession(ctx)
	if err != nil {
		return nil, nil, err
	}
	return inner.StreamChat(ctx, req, onToken, onThinking)
}

// ListModels returns available models from the GitHub Copilot API.
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	inner, err := p.ensureSession(ctx)
	if err != nil {
		return nil, err
	}
	return inner.ListModels(ctx)
}

// Ping verifies the provider is reachable and authenticated.
func (p *Provider) Ping(ctx context.Context) error {
	inner, err := p.ensureSession(ctx)
	if err != nil {
		return fmt.Errorf("cannot connect to GitHub Copilot: %w", err)
	}
	return inner.Ping(ctx)
}

// GetContextLength returns the context window size for a model.
// The Copilot API does not expose context length metadata, so we return 0
// to signal "unknown / use model default" — consistent with the openai provider.
func (p *Provider) GetContextLength(ctx context.Context, modelName string) (int, error) {
	return 0, nil
}
