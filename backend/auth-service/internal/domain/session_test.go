package domain

import (
	"testing"
	"time"
)

func TestSession_Active(t *testing.T) {
	now := time.Now()
	revoked := now.Add(-time.Minute)

	for name, tc := range map[string]struct {
		session Session
		want    bool
	}{
		"active":          {Session{ExpiresAt: now.Add(time.Hour)}, true},
		"expired":         {Session{ExpiresAt: now.Add(-time.Second)}, false},
		"expires exactly": {Session{ExpiresAt: now}, false},
		"revoked":         {Session{ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}, false},
	} {
		if got := tc.session.Active(now); got != tc.want {
			t.Errorf("%s: Active() = %v, want %v", name, got, tc.want)
		}
	}
}
