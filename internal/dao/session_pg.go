package dao

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
)

const sessionsTable = "identity_sessions"

func (p *PG) CreateSession(ctx context.Context, s model.Session, ttl time.Duration) error {
	if ttl > 0 && s.ExpiresAt.IsZero() {
		base := s.CreatedAt
		if base.IsZero() {
			base = time.Now().UTC()
		}
		s.ExpiresAt = base.Add(ttl)
	}
	s.Authentication = authentication.NormalizeLegacy(s.Authentication, s.CreatedAt)
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		return insertAuthenticationSessionTX(ctx, tx, s)
	})
}

func insertAuthenticationSessionTX(ctx context.Context, tx gdb.TX, s model.Session) error {
	credentialRefs := s.Authentication.CredentialRefs
	if credentialRefs == nil {
		credentialRefs = []string{}
	}
	if _, err := tx.Model("authentication_events").Ctx(ctx).Data(g.Map{
		"id":                 s.Authentication.EventID,
		"identity_id":        s.IdentityID,
		"session_id":         s.ID,
		"authenticated_at":   s.Authentication.AuthenticatedAt,
		"methods":            authentication.MethodStrings(s.Authentication.Methods),
		"factor_classes":     authentication.FactorStrings(s.Authentication.FactorClasses),
		"assurance_level":    s.Authentication.Level,
		"assurance_profile":  s.Authentication.Profile,
		"user_verified":      s.Authentication.UserVerified,
		"phishing_resistant": s.Authentication.PhishingResistant,
		"recovery":           s.Authentication.Recovery,
		"credential_refs":    credentialRefs,
		"policy_version":     s.Authentication.PolicyVersion,
	}).Insert(); err != nil {
		return err
	}
	_, err := tx.Model(sessionsTable).Ctx(ctx).Data(g.Map{
		"id":                      s.ID,
		"identity_id":             s.IdentityID,
		"created_at":              s.CreatedAt,
		"last_seen":               s.LastSeen,
		"user_agent":              s.UserAgent,
		"ip":                      s.IP,
		"device":                  s.Device,
		"expires_at":              s.ExpiresAt,
		"authentication_event_id": s.Authentication.EventID,
	}).Insert()
	return err
}

func (p *PG) GetSession(ctx context.Context, id string) (model.Session, error) {
	var row sessionRow
	if err := p.db.Model(sessionsTable+" AS sessions").Ctx(ctx).
		LeftJoin("authentication_events AS events", "events.id = sessions.authentication_event_id").
		Fields(sessionFields).
		Where("sessions.id", id).
		Where("sessions.expires_at IS NULL OR sessions.expires_at > ?", time.Now().UTC()).
		Scan(&row); err != nil {
		return model.Session{}, err
	}
	if row.ID == "" {
		return model.Session{}, repo.ErrSessionNotFound
	}
	return row.session(), nil
}

func (p *PG) DeleteSession(ctx context.Context, id string) error {
	_, err := p.db.Model(sessionsTable).Ctx(ctx).Where("id", id).Delete()
	return err
}

func (p *PG) ListSessionsByIdentity(ctx context.Context, identityID string) ([]model.Session, error) {
	var rows []sessionRow
	if err := p.db.Model(sessionsTable+" AS sessions").Ctx(ctx).
		LeftJoin("authentication_events AS events", "events.id = sessions.authentication_event_id").
		Fields(sessionFields).
		Where("sessions.identity_id", identityID).
		Where("sessions.expires_at IS NULL OR sessions.expires_at > ?", time.Now().UTC()).
		OrderDesc("sessions.created_at").
		Scan(&rows); err != nil {
		return nil, err
	}
	out := make([]model.Session, len(rows))
	for i := range rows {
		out[i] = rows[i].session()
	}
	return out, nil
}

func (p *PG) DeleteSessionsByIdentity(ctx context.Context, identityID string) error {
	_, err := p.db.Model(sessionsTable).Ctx(ctx).Where("identity_id", identityID).Delete()
	return err
}

const sessionFields = `
sessions.id,
sessions.identity_id,
sessions.created_at,
sessions.last_seen,
sessions.user_agent,
sessions.ip,
sessions.device,
sessions.expires_at,
events.id AS authentication_event_id,
events.authenticated_at,
events.methods AS authentication_methods,
events.factor_classes AS authentication_factor_classes,
events.assurance_level,
events.assurance_profile,
events.user_verified,
events.phishing_resistant,
events.recovery,
events.credential_refs,
events.policy_version`

type sessionRow struct {
	ID                          string
	IdentityID                  string
	CreatedAt                   time.Time
	LastSeen                    time.Time
	UserAgent                   string
	IP                          string
	Device                      string
	ExpiresAt                   time.Time
	AuthenticationEventID       string
	AuthenticatedAt             time.Time
	AuthenticationMethods       []string
	AuthenticationFactorClasses []string
	AssuranceLevel              string
	AssuranceProfile            string
	UserVerified                bool
	PhishingResistant           bool
	Recovery                    bool
	CredentialRefs              []string
	PolicyVersion               int
}

func (row sessionRow) session() model.Session {
	return model.Session{
		ID: row.ID, IdentityID: row.IdentityID, CreatedAt: row.CreatedAt,
		LastSeen: row.LastSeen, UserAgent: row.UserAgent, IP: row.IP,
		Device: row.Device, ExpiresAt: row.ExpiresAt,
		Authentication: authentication.Context{
			EventID: row.AuthenticationEventID, AuthenticatedAt: row.AuthenticatedAt,
			Methods:       authentication.Methods(row.AuthenticationMethods),
			FactorClasses: authentication.Factors(row.AuthenticationFactorClasses),
			Level:         authentication.Level(row.AssuranceLevel),
			Profile:       authentication.Profile(row.AssuranceProfile),
			UserVerified:  row.UserVerified, PhishingResistant: row.PhishingResistant,
			Recovery: row.Recovery, CredentialRefs: row.CredentialRefs,
			PolicyVersion: row.PolicyVersion,
		},
	}
}
