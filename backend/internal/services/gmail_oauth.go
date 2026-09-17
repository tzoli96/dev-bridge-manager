// backend/internal/services/gmail_oauth.go
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var gmailScopes = []string{gmail.GmailReadonlyScope, gmail.GmailSendScope}

func gmailOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		Scopes:       gmailScopes,
		Endpoint:     google.Endpoint,
	}
}

// The OAuth "state" parameter is this flow's CSRF protection: auth-url
// (called with the app's normal JWT auth) mints a one-time random state
// tied to the requesting user; the callback (hit by Google's plain browser
// redirect, which carries no app auth header) trades it back for that
// user id. Since this app runs as a single process with no horizontal
// scaling, an in-memory map is enough — a server restart between auth-url
// and callback just means the user clicks "Connect Gmail" again.
type oauthStateEntry struct {
	userID    uint
	expiresAt time.Time
}

var (
	oauthStateMu      sync.Mutex
	oauthStatePending = map[string]oauthStateEntry{}
)

func newOAuthState(userID uint) string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	state := hex.EncodeToString(buf)

	oauthStateMu.Lock()
	oauthStatePending[state] = oauthStateEntry{userID: userID, expiresAt: time.Now().Add(10 * time.Minute)}
	oauthStateMu.Unlock()
	return state
}

// consumeOAuthState validates and deletes a one-time state value, returning
// the userID it was issued for. Returns an error for an unknown or expired
// state (forged or stale callback).
func consumeOAuthState(state string) (uint, error) {
	oauthStateMu.Lock()
	defer oauthStateMu.Unlock()

	entry, ok := oauthStatePending[state]
	delete(oauthStatePending, state)
	if !ok {
		return 0, fmt.Errorf("unknown or already-used oauth state")
	}
	if time.Now().After(entry.expiresAt) {
		return 0, fmt.Errorf("oauth state expired")
	}
	return entry.userID, nil
}

// GmailAuthURL builds the Google consent screen URL for userID to connect
// their Gmail account. AccessTypeOffline + prompt=consent ensure Google
// always returns a refresh_token, even on a repeat connect.
func GmailAuthURL(userID uint) string {
	state := newOAuthState(userID)
	return gmailOAuthConfig().AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
}

// ExchangeAndSaveGmailAccount trades an OAuth code for tokens, fetches the
// connected address, and upserts the gmail_accounts row for state's user.
func ExchangeAndSaveGmailAccount(ctx context.Context, code, state string) error {
	userID, err := consumeOAuthState(state)
	if err != nil {
		return err
	}

	token, err := gmailOAuthConfig().Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange oauth code: %w", err)
	}

	svc, err := gmail.NewService(ctx, option.WithTokenSource(gmailOAuthConfig().TokenSource(ctx, token)))
	if err != nil {
		return fmt.Errorf("failed to create gmail service: %w", err)
	}
	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return fmt.Errorf("failed to fetch gmail profile: %w", err)
	}

	account := models.GmailAccount{
		UserID:       userID,
		EmailAddress: profile.EmailAddress,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry,
		NeedsReauth:  false,
	}

	return database.GetDB().
		Where("user_id = ?", userID).
		Assign(account).
		FirstOrCreate(&account).Error
}

// validTokenForAccount returns a non-expired oauth2.Token for account,
// refreshing and persisting it first if the stored access token has
// expired. Every RealGmailAPI call goes through this so a refreshed token
// is never silently dropped after a single request.
func validTokenForAccount(ctx context.Context, account *models.GmailAccount) (*oauth2.Token, error) {
	existing := &oauth2.Token{
		AccessToken:  account.AccessToken,
		RefreshToken: account.RefreshToken,
		Expiry:       account.TokenExpiry,
	}

	fresh, err := gmailOAuthConfig().TokenSource(ctx, existing).Token()
	if err != nil {
		return nil, err
	}

	if fresh.AccessToken != existing.AccessToken {
		database.GetDB().Model(account).Updates(map[string]interface{}{
			"access_token": fresh.AccessToken,
			"token_expiry": fresh.Expiry,
		})
		account.AccessToken = fresh.AccessToken
		account.TokenExpiry = fresh.Expiry
	}
	return fresh, nil
}
