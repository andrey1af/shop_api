package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/jwt"
)

type memorySessionStore struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]domain.Session
	tokens   []domain.RefreshToken

	deletedNow, deletedUsedBefore time.Time
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{sessions: map[uuid.UUID]domain.Session{}}
}

func (m *memorySessionStore) CreateSession(_ context.Context, session domain.Session, token domain.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
	m.tokens = append(m.tokens, token)
	return nil
}

func (m *memorySessionStore) SessionByID(_ context.Context, id uuid.UUID) (domain.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return domain.Session{}, apperrors.ErrSessionNotFound
	}
	return s, nil
}

func (m *memorySessionStore) RefreshTokenByHash(_ context.Context, hash []byte) (domain.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tokens {
		if bytes.Equal(t.TokenHash, hash) {
			return t, nil
		}
	}
	return domain.RefreshToken{}, apperrors.ErrRefreshTokenNotFound
}

func (m *memorySessionStore) RotateRefreshToken(
	_ context.Context,
	usedTokenID uuid.UUID,
	usedAt time.Time,
	next domain.RefreshToken,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, t := range m.tokens {
		if t.ID == usedTokenID {
			if t.UsedAt != nil {
				return apperrors.ErrRefreshTokenReused
			}
			m.tokens[i].UsedAt = &usedAt
		}
	}
	m.tokens = append(m.tokens, next)
	s := m.sessions[next.SessionID]
	s.ExpiresAt = next.ExpiresAt
	m.sessions[next.SessionID] = s
	return nil
}

func (m *memorySessionStore) RevokeSession(_ context.Context, id uuid.UUID, revokedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok && s.RevokedAt == nil {
		s.RevokedAt = &revokedAt
		m.sessions[id] = s
	}
	return nil
}

func (m *memorySessionStore) RevokeUserSessions(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	return m.RevokeUserSessionsExcept(ctx, userID, uuid.Nil(), revokedAt)
}

func (m *memorySessionStore) RevokeUserSessionsExcept(
	_ context.Context,
	userID, keepSessionID uuid.UUID,
	revokedAt time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.UserID == userID && id != keepSessionID && s.RevokedAt == nil {
			s.RevokedAt = &revokedAt
			m.sessions[id] = s
		}
	}
	return nil
}

func (m *memorySessionStore) DeleteExpired(_ context.Context, now, usedBefore time.Time) (int64, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedNow, m.deletedUsedBefore = now, usedBefore
	return 1, 2, nil
}

type stubUserProvider struct {
	user domain.User
}

func (s stubUserProvider) UserByID(context.Context, uuid.UUID) (domain.User, error) {
	return s.user, nil
}

func (s stubUserProvider) UserByEmail(context.Context, string) (domain.User, error) {
	return s.user, nil
}

type sessionsFixture struct {
	sessions *Sessions
	store    *memorySessionStore
	user     domain.User
	now      time.Time
}

func newSessionsFixture(t *testing.T) *sessionsFixture {
	t.Helper()

	f := &sessionsFixture{
		store: newMemorySessionStore(),
		user:  domain.User{ID: uuid.New(), Email: "user@example.com"},
		now:   time.Now(),
	}
	f.sessions = NewSessions(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		f.store,
		stubUserProvider{user: f.user},
		jwt.NewManager("test-secret", 15*time.Minute),
		30*24*time.Hour,
	)
	f.sessions.now = func() time.Time { return f.now }

	return f
}

func TestSessions_StartAndValidate(t *testing.T) {
	f := newSessionsFixture(t)

	token, err := f.sessions.Start(context.Background(), f.user)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		t.Fatalf("Start() token = %+v, want both access and refresh", token)
	}

	claims, err := f.sessions.Validate(context.Background(), token.AccessToken)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if claims.UserID != f.user.ID || claims.SessionID == uuid.Nil() {
		t.Errorf("Validate() claims = %+v", claims)
	}

	if bytes.Equal(f.store.tokens[0].TokenHash, []byte(token.RefreshToken)) {
		t.Error("refresh token is stored in plain text")
	}
}

func TestSessions_RefreshRotatesToken(t *testing.T) {
	f := newSessionsFixture(t)
	ctx := context.Background()

	first, _ := f.sessions.Start(ctx, f.user)

	f.now = f.now.Add(time.Hour)
	second, err := f.sessions.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("Refresh() returned the same refresh token")
	}
	if _, err := f.sessions.Validate(ctx, second.AccessToken); err != nil {
		t.Fatalf("Validate(new access) error = %v", err)
	}

	claims, _ := f.sessions.Validate(ctx, second.AccessToken)
	if got := f.store.sessions[claims.SessionID].ExpiresAt; !got.Equal(second.RefreshExpiresAt) {
		t.Errorf("session expires at %v, want %v", got, second.RefreshExpiresAt)
	}
}

func TestSessions_RefreshReuseRevokesSession(t *testing.T) {
	f := newSessionsFixture(t)
	ctx := context.Background()

	first, _ := f.sessions.Start(ctx, f.user)
	second, err := f.sessions.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if _, err := f.sessions.Refresh(ctx, first.RefreshToken); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Fatalf("Refresh(reused) error = %v, want ErrInvalidToken", err)
	}

	if _, err := f.sessions.Validate(ctx, second.AccessToken); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Errorf("Validate(after reuse) error = %v, want ErrInvalidToken", err)
	}
	if _, err := f.sessions.Refresh(ctx, second.RefreshToken); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Errorf("Refresh(after reuse) error = %v, want ErrInvalidToken", err)
	}
}

func TestSessions_RefreshRejectsExpiredAndUnknownTokens(t *testing.T) {
	f := newSessionsFixture(t)
	ctx := context.Background()

	token, _ := f.sessions.Start(ctx, f.user)

	if _, err := f.sessions.Refresh(ctx, "unknown"); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Errorf("Refresh(unknown) error = %v, want ErrInvalidToken", err)
	}

	f.now = f.now.Add(31 * 24 * time.Hour)
	if _, err := f.sessions.Refresh(ctx, token.RefreshToken); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Errorf("Refresh(expired) error = %v, want ErrInvalidToken", err)
	}
}

func TestSessions_Revoke(t *testing.T) {
	f := newSessionsFixture(t)
	ctx := context.Background()

	byRefresh, _ := f.sessions.Start(ctx, f.user)
	byAccess, _ := f.sessions.Start(ctx, f.user)
	kept, _ := f.sessions.Start(ctx, f.user)
	other, _ := f.sessions.Start(ctx, f.user)

	if err := f.sessions.RevokeByRefreshToken(ctx, byRefresh.RefreshToken); err != nil {
		t.Fatalf("RevokeByRefreshToken() error = %v", err)
	}
	if err := f.sessions.RevokeByAccessToken(ctx, byAccess.AccessToken); err != nil {
		t.Fatalf("RevokeByAccessToken() error = %v", err)
	}

	keptClaims, _ := f.sessions.Validate(ctx, kept.AccessToken)
	if err := f.sessions.RevokeAllExcept(ctx, f.user.ID, keptClaims.SessionID); err != nil {
		t.Fatalf("RevokeAllExcept() error = %v", err)
	}

	for name, token := range map[string]Token{"by refresh": byRefresh, "by access": byAccess, "other": other} {
		if _, err := f.sessions.Validate(ctx, token.AccessToken); !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("%s: Validate() error = %v, want ErrInvalidToken", name, err)
		}
	}
	if _, err := f.sessions.Validate(ctx, kept.AccessToken); err != nil {
		t.Errorf("kept session: Validate() error = %v", err)
	}

	if err := f.sessions.RevokeAll(ctx, f.user.ID); err != nil {
		t.Fatalf("RevokeAll() error = %v", err)
	}
	if _, err := f.sessions.Validate(ctx, kept.AccessToken); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Errorf("after RevokeAll: Validate() error = %v, want ErrInvalidToken", err)
	}
}

func TestSessions_ExpiredSessionIsInvalid(t *testing.T) {
	f := newSessionsFixture(t)
	ctx := context.Background()

	token, _ := f.sessions.Start(ctx, f.user)
	claims, _ := f.sessions.Validate(ctx, token.AccessToken)

	s := f.store.sessions[claims.SessionID]
	s.ExpiresAt = f.now.Add(-time.Second)
	f.store.sessions[claims.SessionID] = s

	if _, err := f.sessions.Validate(ctx, token.AccessToken); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Fatalf("Validate() error = %v, want ErrInvalidToken", err)
	}
}

func TestSessions_DeleteExpiredKeepsUsedTokensForReuseDetection(t *testing.T) {
	f := newSessionsFixture(t)

	if err := f.sessions.DeleteExpired(context.Background()); err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}
	if !f.store.deletedNow.Equal(f.now) {
		t.Errorf("deleted sessions expired before %v, want now", f.store.deletedNow)
	}
	if want := f.now.Add(-usedRefreshTokenRetention); !f.store.deletedUsedBefore.Equal(want) {
		t.Errorf("deleted used tokens before %v, want %v", f.store.deletedUsedBefore, want)
	}
}

func TestSessions_RevokeByUnknownRefreshToken(t *testing.T) {
	f := newSessionsFixture(t)
	err := f.sessions.RevokeByRefreshToken(context.Background(), "unknown")
	if !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Fatalf("RevokeByRefreshToken() error = %v, want ErrInvalidToken", err)
	}
}
