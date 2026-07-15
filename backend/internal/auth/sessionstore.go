package auth

import (
	"context"
	"sort"
	"strconv"
	"time"
)

// Redis key prefixes for device-session state.
const (
	sessionPrefix     = "auth:session:"   // hash of one session's device metadata, keyed by sid
	userSessionPrefix = "auth:usessions:" // SET of a user's live sids (enumeration index)
)

func sessionKey(sid string) string        { return sessionPrefix + sid }
func userSessionKey(userID string) string { return userSessionPrefix + userID }

// SessionMeta is the device metadata recorded when a session is created.
type SessionMeta struct {
	UserID     string
	UserAgent  string
	IP         string
	Location   string // resolved city/country (GeoIP); empty when unknown
	RefreshJTI string
}

// Session is a hydrated device-session record.
type Session struct {
	ID         string // the sid
	UserID     string
	UserAgent  string
	IP         string
	Location   string
	RefreshJTI string
	CreatedAt  time.Time
	LastSeenAt time.Time
}

// SaveSession records a new session (login): the metadata hash plus an entry in
// the per-user index, both expiring with the refresh token.
func (s *TokenStore) SaveSession(ctx context.Context, sid string, m SessionMeta, ttl time.Duration) error {
	now := time.Now().Unix()
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, sessionKey(sid), map[string]any{
		"user_id":     m.UserID,
		"ua":          m.UserAgent,
		"ip":          m.IP,
		"location":    m.Location,
		"refresh_jti": m.RefreshJTI,
		"created_at":  now,
		"last_seen":   now,
	})
	pipe.Expire(ctx, sessionKey(sid), ttl)
	pipe.SAdd(ctx, userSessionKey(m.UserID), sid)
	pipe.Expire(ctx, userSessionKey(m.UserID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// TouchSession updates a session on refresh rotation: new last_seen + refresh
// JTI and a bumped TTL. It re-adds the sid to the user index (self-heal) so a
// long-lived session whose index entry lapsed is not lost.
func (s *TokenStore) TouchSession(ctx context.Context, sid, userID, refreshJTI string, seenAt time.Time, ttl time.Duration) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, sessionKey(sid), map[string]any{
		"last_seen":   seenAt.Unix(),
		"refresh_jti": refreshJTI,
	})
	pipe.Expire(ctx, sessionKey(sid), ttl)
	pipe.SAdd(ctx, userSessionKey(userID), sid)
	pipe.Expire(ctx, userSessionKey(userID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// SessionAlive reports whether the session record still exists. A revoked or
// expired session returns false, which RequireAuth uses to reject a still-valid
// access token whose session was killed.
func (s *TokenStore) SessionAlive(ctx context.Context, sid string) (bool, error) {
	n, err := s.rdb.Exists(ctx, sessionKey(sid)).Result()
	return n > 0, err
}

// SessionOwnedBy reports whether sid is one of the user's own sessions. It is
// the segregation-of-duties gate for revocation: a caller may only ever revoke
// a session listed in their own index, never another user's (the session hash
// itself is keyed by a global sid, so deletion must be authorized here first).
func (s *TokenStore) SessionOwnedBy(ctx context.Context, userID, sid string) (bool, error) {
	return s.rdb.SIsMember(ctx, userSessionKey(userID), sid).Result()
}

// ListSessions returns the user's live sessions, newest activity first. Index
// entries whose metadata hash has expired are pruned lazily (SREM).
func (s *TokenStore) ListSessions(ctx context.Context, userID string) ([]Session, error) {
	sids, err := s.rdb.SMembers(ctx, userSessionKey(userID)).Result()
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, 0, len(sids))
	for _, sid := range sids {
		h, err := s.rdb.HGetAll(ctx, sessionKey(sid)).Result()
		if err != nil {
			return nil, err
		}
		if len(h) == 0 {
			// Metadata expired but the index still references it: prune.
			_ = s.rdb.SRem(ctx, userSessionKey(userID), sid).Err()
			continue
		}
		sessions = append(sessions, Session{
			ID:         sid,
			UserID:     h["user_id"],
			UserAgent:  h["ua"],
			IP:         h["ip"],
			Location:   h["location"],
			RefreshJTI: h["refresh_jti"],
			CreatedAt:  unixToTime(h["created_at"]),
			LastSeenAt: unixToTime(h["last_seen"]),
		})
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastSeenAt.After(sessions[j].LastSeenAt)
	})
	return sessions, nil
}

// DeleteSession removes one session (metadata + index entry) and returns its
// current refresh JTI so the caller can also drop it from the refresh whitelist.
// A missing session yields an empty JTI and no error (idempotent).
func (s *TokenStore) DeleteSession(ctx context.Context, userID, sid string) (string, error) {
	refreshJTI, _ := s.rdb.HGet(ctx, sessionKey(sid), "refresh_jti").Result()
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, sessionKey(sid))
	pipe.SRem(ctx, userSessionKey(userID), sid)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return refreshJTI, nil
}

// DeleteAllSessions removes every session for a user (used on password
// change/reset for a uniform "logout everywhere"). It returns the refresh JTIs
// it removed so the caller can clear them from the whitelist too.
func (s *TokenStore) DeleteAllSessions(ctx context.Context, userID string) ([]string, error) {
	sids, err := s.rdb.SMembers(ctx, userSessionKey(userID)).Result()
	if err != nil {
		return nil, err
	}
	refreshJTIs := make([]string, 0, len(sids))
	pipe := s.rdb.TxPipeline()
	for _, sid := range sids {
		if jti, err := s.rdb.HGet(ctx, sessionKey(sid), "refresh_jti").Result(); err == nil && jti != "" {
			refreshJTIs = append(refreshJTIs, jti)
		}
		pipe.Del(ctx, sessionKey(sid))
	}
	pipe.Del(ctx, userSessionKey(userID))
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	return refreshJTIs, nil
}

func unixToTime(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(n, 0)
}
