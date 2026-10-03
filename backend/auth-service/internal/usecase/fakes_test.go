package usecase

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/jwt"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type memoryUserStore struct {
	mu         sync.Mutex
	users      map[uuid.UUID]domain.User
	identities []domain.UserIdentity
}

func newMemoryUserStore() *memoryUserStore {
	return &memoryUserStore{users: map[uuid.UUID]domain.User{}}
}

func (m *memoryUserStore) UserByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return domain.User{}, apperrors.ErrUserNotFound
	}
	return u, nil
}

func (m *memoryUserStore) UserByEmail(_ context.Context, email string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return domain.User{}, apperrors.ErrUserNotFound
}

func (m *memoryUserStore) SaveUser(_ context.Context, user domain.User) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if strings.EqualFold(u.Email, user.Email) {
			return domain.User{}, apperrors.ErrUserAlreadyExists
		}
	}
	m.users[user.ID] = user
	return user, nil
}

func (m *memoryUserStore) UpdatePassword(_ context.Context, userID uuid.UUID, passwordHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return apperrors.ErrUserNotFound
	}
	u.PasswordHash = passwordHash
	m.users[userID] = u
	return nil
}

func (m *memoryUserStore) UserByIdentity(_ context.Context, provider, providerUserID string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, i := range m.identities {
		if i.Provider == provider && i.ProviderUserID == providerUserID {
			return m.users[i.UserID], nil
		}
	}
	return domain.User{}, apperrors.ErrUserNotFound
}

func (m *memoryUserStore) SaveUserWithIdentity(
	ctx context.Context,
	user domain.User,
	identity domain.UserIdentity,
) (domain.User, error) {
	saved, err := m.SaveUser(ctx, user)
	if err != nil {
		return domain.User{}, err
	}
	identity.UserID = saved.ID
	return saved, m.SaveIdentity(ctx, identity)
}

func (m *memoryUserStore) SaveIdentity(_ context.Context, identity domain.UserIdentity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.identities = append(m.identities, identity)
	return nil
}

type authFixture struct {
	auth     *AuthUseCase
	sessions *Sessions
	users    *memoryUserStore
	store    *memorySessionStore
	now      time.Time
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()

	f := &authFixture{
		users: newMemoryUserStore(),
		store: newMemorySessionStore(),
		now:   time.Now(),
	}
	f.sessions = NewSessions(
		newTestLogger(),
		f.store,
		f.users,
		jwt.NewManager("test-secret", 15*time.Minute),
		30*24*time.Hour,
	)
	f.sessions.now = func() time.Time { return f.now }
	f.auth = NewAuthUseCase(newTestLogger(), f.users, f.users, f.sessions)

	return f
}

func (f *authFixture) register(t *testing.T, email, password string) RegisterUserOutput {
	t.Helper()

	out, err := f.auth.Register(context.Background(), RegisterUserInput{
		Name: "Ivan", Surname: "Petrov", Email: email, PhoneNumber: "+79991234567", Password: password,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return out
}

func (f *authFixture) login(t *testing.T, email, password string) Token {
	t.Helper()

	out, err := f.auth.Login(context.Background(), LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return out.Token
}

func domainUser(email string, passwordHash []byte) domain.User {
	return domain.User{
		ID:           uuid.New(),
		Name:         "Ivan",
		Surname:      "Petrov",
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
}

type memoryStateStore struct {
	mu            sync.Mutex
	states        map[string]domain.OAuthState
	deleteErr     error
	deleteCalls   int
	deletedBefore time.Time
}

func newMemoryStateStore() *memoryStateStore {
	return &memoryStateStore{states: map[string]domain.OAuthState{}}
}

func (m *memoryStateStore) SaveState(_ context.Context, state domain.OAuthState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[state.State] = state
	return nil
}

func (m *memoryStateStore) ConsumeState(_ context.Context, state string) (domain.OAuthState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.states[state]
	if !ok {
		return domain.OAuthState{}, apperrors.ErrInvalidOAuthState
	}
	delete(m.states, state)
	return s, nil
}

func (m *memoryStateStore) DeleteExpiredStates(_ context.Context, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls++
	m.deletedBefore = now
	return m.deleteErr
}

type fakeOAuthProvider struct {
	profile     domain.OAuthProfile
	exchangeErr error

	gotState    string
	gotVerifier string
	gotCode     string
}

func (p *fakeOAuthProvider) AuthCodeURL(state, codeVerifier string) string {
	p.gotState, p.gotVerifier = state, codeVerifier
	return "https://provider.example/authorize?state=" + state
}

func (p *fakeOAuthProvider) Exchange(_ context.Context, code, codeVerifier string) (domain.OAuthProfile, error) {
	p.gotCode, p.gotVerifier = code, codeVerifier
	return p.profile, p.exchangeErr
}
