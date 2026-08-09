// Package dao implements the PostgreSQL-backed IdentityRepo (gdb). The atomic
// create runs in a gdb transaction (identity + profile + password credential).
package dao

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/yueli-official/foundation/go/identifier"

	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
	"github.com/yueli-official/identity/internal/user"
)

// PG is a PostgreSQL-backed implementation of repo.IdentityRepo using gdb.
type PG struct {
	db gdb.DB
}

func (p *PG) SetProfileSocialLinks(ctx context.Context, identityID string, links []model.SocialLink) error {
	document, err := json.Marshal(links)
	if err != nil {
		return err
	}
	result, err := p.db.Model("user_profiles").Ctx(ctx).Where("identity_id", identityID).
		Data(g.Map{"social_links": string(document)}).Update()
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return repo.ErrIdentityMissing
	}
	return nil
}

// NewPG creates a PG repo backed by the given gdb.DB connection.
func NewPG(db gdb.DB) *PG { return &PG{db: db} }

// CreateIdentityWithProfile atomically inserts an identity row, a user_profiles
// row, and a credentials_password row inside a single transaction.
// It returns repo.ErrEmailTaken when a UNIQUE constraint on identities.email fires.
func (p *PG) CreateIdentityWithProfile(ctx context.Context, in repo.NewIdentityInput) (model.Identity, error) {
	if strings.TrimSpace(in.ID) == "" {
		in.ID = identifier.MustNew().String()
	} else if parsed, err := identifier.Parse(in.ID); err != nil || parsed.Version() != 7 {
		return model.Identity{}, errors.New("identity ID must be a canonical UUIDv7")
	}
	if strings.TrimSpace(in.UserKey) != "" {
		if _, err := user.ParsePublicKey(in.UserKey); err != nil {
			return model.Identity{}, err
		}
		return p.createIdentityWithProfile(ctx, in, in.UserKey)
	}
	var out model.Identity
	_, err := identifier.Allocate(ctx, identifier.CompactURLV1,
		func(ctx context.Context, candidate identifier.Key) (identifier.ClaimResult, error) {
			created, err := p.createIdentityWithProfile(ctx, in, candidate.String())
			if errors.Is(err, errPublicKeyCollision) {
				return identifier.Collision, nil
			}
			if err != nil {
				return 0, err
			}
			out = created
			return identifier.Claimed, nil
		})
	return out, err
}

func (p *PG) createIdentityWithProfile(ctx context.Context, in repo.NewIdentityInput, userKey string) (model.Identity, error) {
	var out model.Identity
	err := p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Set context on the transaction so that subsequent Model calls inherit it.
		tx = tx.Ctx(ctx)

		if err := validateRoles(ctx, tx, in.Roles); err != nil {
			return err
		}
		val, err := tx.GetValue(
			"INSERT INTO identities (id, user_key, email) VALUES (?, ?, ?) RETURNING id",
			in.ID, userKey, in.Email,
		)
		if err != nil {
			if isPublicKeyCollision(err) {
				return errPublicKeyCollision
			}
			if isUniqueViolation(err) {
				return repo.ErrEmailTaken
			}
			return err
		}
		id := val.String()
		if _, err := tx.Model("oidc_subjects").Ctx(ctx).Data(g.Map{
			"identity_id": id, "sector_key": "public", "subject": userKey, "subject_type": "public",
		}).Insert(); err != nil {
			return err
		}

		if _, err := tx.Model("user_profiles").Ctx(ctx).Data(g.Map{
			"identity_id":  id,
			"display_name": in.DisplayName,
			"locale":       orDefault(in.Locale, "zh-CN"),
		}).Insert(); err != nil {
			return err
		}
		if _, err := tx.Model("credentials_password").Ctx(ctx).Data(g.Map{
			"identity_id":   id,
			"password_hash": in.PasswordHash,
		}).Insert(); err != nil {
			return err
		}
		if err := insertRoles(ctx, tx, id, in.Roles); err != nil {
			return err
		}

		// Reload the full identity row (including DB defaults: status, timestamps).
		return tx.Model("identities").Ctx(ctx).Where("id", id).Scan(&out)
	})
	return out, err
}

// GetByEmail returns the non-deleted identity with the given email address.
// Returns repo.ErrIdentityMissing when absent.
func (p *PG) GetByEmail(ctx context.Context, email string) (model.Identity, error) {
	var out model.Identity
	err := p.db.Model("identities").Ctx(ctx).
		Where("email", email).Where("status <>", string(model.StatusDeleted)).Scan(&out)
	if err != nil {
		return model.Identity{}, err
	}
	if out.ID == "" {
		return model.Identity{}, repo.ErrIdentityMissing
	}
	return out, nil
}

// GetByID returns the identity with the given UUID.
// Returns repo.ErrIdentityMissing when absent.
func (p *PG) GetByID(ctx context.Context, id string) (model.Identity, error) {
	var out model.Identity
	if err := p.db.Model("identities").Ctx(ctx).Where("id", id).Scan(&out); err != nil {
		return model.Identity{}, err
	}
	if out.ID == "" {
		return model.Identity{}, repo.ErrIdentityMissing
	}
	return out, nil
}

func (p *PG) GetByUserKey(ctx context.Context, userKey string) (model.Identity, error) {
	var out model.Identity
	if err := p.db.Model("identities").Ctx(ctx).Where("user_key", userKey).Scan(&out); err != nil {
		return model.Identity{}, err
	}
	if out.ID == "" {
		return model.Identity{}, repo.ErrIdentityMissing
	}
	return out, nil
}

func (p *PG) GetUserKeysByIDs(ctx context.Context, identityIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(identityIDs))
	if len(identityIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID      string `orm:"id"`
		UserKey string `orm:"user_key"`
	}
	if err := p.db.Model("identities").Ctx(ctx).
		Fields("id, user_key").WhereIn("id", identityIDs).Scan(&rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.UserKey
	}
	return out, nil
}

func (p *PG) GetByOIDCSubject(ctx context.Context, subject string) (model.Identity, error) {
	var out model.Identity
	err := p.db.Model("oidc_subjects s").Ctx(ctx).
		InnerJoin("identities i", "i.id = s.identity_id").
		Fields("i.*").Where("s.subject", subject).Scan(&out)
	if err != nil {
		return model.Identity{}, err
	}
	if out.ID == "" {
		return model.Identity{}, repo.ErrIdentityMissing
	}
	return out, nil
}

func (p *PG) ResolveOIDCSubject(ctx context.Context, identityID, subjectType, sector string) (string, error) {
	identity, err := p.GetByID(ctx, identityID)
	if err != nil {
		return "", err
	}
	if subjectType == "" || subjectType == "public" {
		_, err := p.db.Exec(ctx, `
INSERT INTO oidc_subjects (identity_id, sector_key, subject, subject_type)
VALUES (?, 'public', ?, 'public')
ON CONFLICT (identity_id, sector_key) DO NOTHING`, identityID, identity.UserKey)
		if err != nil {
			return "", err
		}
		return identity.UserKey, nil
	}
	sector = strings.TrimSpace(sector)
	if subjectType != "pairwise" || sector == "" {
		return "", errors.New("invalid OIDC subject policy")
	}
	sectorKey := "pairwise:" + sector
	for range 3 {
		generated, err := user.NewPairwiseSubject()
		if err != nil {
			return "", err
		}
		_, err = p.db.Exec(ctx, `
INSERT INTO oidc_subjects (identity_id, sector_key, subject, subject_type)
VALUES (?, ?, ?, 'pairwise')
ON CONFLICT (identity_id, sector_key) DO NOTHING`, identityID, sectorKey, generated)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return "", err
		}
		value, err := p.db.Model("oidc_subjects").Ctx(ctx).Fields("subject").
			Where("identity_id", identityID).Where("sector_key", sectorKey).Value()
		if err != nil {
			return "", err
		}
		if value.String() != "" {
			return value.String(), nil
		}
	}
	return "", errors.New("pairwise subject collision")
}

func (p *PG) ListOIDCSubjects(ctx context.Context, identityID string) ([]string, error) {
	values, err := p.db.Model("oidc_subjects").Ctx(ctx).Fields("subject").
		Where("identity_id", identityID).OrderAsc("subject").Array()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.String())
	}
	return out, nil
}

// GetPasswordHash returns the bcrypt password hash stored for the given identity.
// Model.Scan does not support scalar destinations; we use Value() instead.
func (p *PG) GetPasswordHash(ctx context.Context, identityID string) (string, error) {
	val, err := p.db.Model("credentials_password").Ctx(ctx).
		Fields("password_hash").Where("identity_id", identityID).Value()
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// GetProfile returns the user_profiles row for an identity.
// Returns repo.ErrIdentityMissing when absent.
func (p *PG) GetProfile(ctx context.Context, identityID string) (model.Profile, error) {
	var out model.Profile
	if err := p.db.Model("user_profiles").Ctx(ctx).Where("identity_id", identityID).Scan(&out); err != nil {
		return model.Profile{}, err
	}
	if out.IdentityID == "" {
		return model.Profile{}, repo.ErrIdentityMissing
	}
	return out, nil
}

// UpdateProfile replaces the editable display fields of an identity's profile.
// social_links is stored as a JSONB document (marshalled here so the column gets
// valid JSON text rather than a Postgres array literal).
func (p *PG) UpdateProfile(ctx context.Context, identityID string, in repo.ProfileUpdate) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		current, err := tx.GetValue(
			"SELECT COALESCE(handle, '') FROM user_profiles WHERE identity_id = ? FOR UPDATE", identityID,
		)
		if err != nil {
			return err
		}
		if current.IsNil() {
			return repo.ErrIdentityMissing
		}
		currentHandle := current.String()
		if currentHandle != in.Handle {
			if currentHandle != "" {
				if _, err := tx.Exec(
					"UPDATE user_handle_history SET state = 'retired', retired_at = now() WHERE handle = ? AND identity_id = ?",
					currentHandle, identityID,
				); err != nil {
					return err
				}
			}
			if in.Handle != "" {
				var owner struct {
					IdentityID string `orm:"identity_id"`
				}
				if err := tx.Model("user_handle_history").Ctx(ctx).
					Where("handle", in.Handle).LockUpdate().Scan(&owner); err != nil {
					return err
				}
				switch {
				case owner.IdentityID == "":
					if _, err := tx.Model("user_handle_history").Ctx(ctx).Data(g.Map{
						"handle": in.Handle, "identity_id": identityID, "state": "current",
					}).Insert(); err != nil {
						if isUniqueViolation(err) {
							return repo.ErrHandleUnavailable
						}
						return err
					}
				case owner.IdentityID == identityID:
					if _, err := tx.Model("user_handle_history").Ctx(ctx).Where("handle", in.Handle).
						Data(g.Map{"state": "current", "retired_at": nil}).Update(); err != nil {
						return err
					}
				default:
					return repo.ErrHandleUnavailable
				}
			}
		}
		res, err := tx.Model("user_profiles").Ctx(ctx).Where("identity_id", identityID).Data(g.Map{
			"handle": nilIfEmpty(in.Handle), "display_name": in.DisplayName,
			"bio": in.Bio, "locale": orDefault(in.Locale, "zh-CN"),
		}).Update()
		if err != nil {
			if isUniqueViolation(err) {
				return repo.ErrHandleUnavailable
			}
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return repo.ErrIdentityMissing
		}
		return nil
	})
}

func (p *PG) GetPublicUserByKey(ctx context.Context, userKey string) (model.PublicUser, error) {
	var out model.PublicUser
	err := p.db.Model("identities i").Ctx(ctx).
		InnerJoin("user_profiles p", "p.identity_id = i.id").
		Fields("i.user_key, p.handle, p.display_name, p.avatar_media_key, p.cover_media_key, p.bio, p.social_links").
		Where("i.user_key", userKey).Where("i.status", string(model.StatusActive)).Scan(&out)
	if err != nil {
		return model.PublicUser{}, err
	}
	if out.UserKey == "" {
		return model.PublicUser{}, repo.ErrIdentityMissing
	}
	return out, nil
}

func (p *PG) GetPublicUserByHandle(ctx context.Context, handle string) (model.PublicUser, error) {
	var out model.PublicUser
	err := p.db.Model("user_handle_history h").Ctx(ctx).
		InnerJoin("identities i", "i.id = h.identity_id").
		InnerJoin("user_profiles p", "p.identity_id = i.id").
		Fields("i.user_key, p.handle, p.display_name, p.avatar_media_key, p.cover_media_key, p.bio, p.social_links").
		Where("h.handle", handle).Where("i.status", string(model.StatusActive)).Scan(&out)
	if err != nil {
		return model.PublicUser{}, err
	}
	if out.UserKey == "" {
		return model.PublicUser{}, repo.ErrIdentityMissing
	}
	return out, nil
}

func (p *PG) GetPublicUsersByKeys(ctx context.Context, userKeys []string) ([]model.PublicUser, error) {
	if len(userKeys) == 0 {
		return []model.PublicUser{}, nil
	}
	var rows []model.PublicUser
	err := p.db.Model("identities i").Ctx(ctx).
		InnerJoin("user_profiles p", "p.identity_id = i.id").
		Fields("i.user_key, p.handle, p.display_name, p.avatar_media_key, p.cover_media_key, p.bio, p.social_links").
		WhereIn("i.user_key", userKeys).Where("i.status", string(model.StatusActive)).Scan(&rows)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]model.PublicUser, len(rows))
	for _, row := range rows {
		byKey[row.UserKey] = row
	}
	out := make([]model.PublicUser, 0, len(rows))
	for _, key := range userKeys {
		if row, ok := byKey[key]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

func validateRoles(ctx context.Context, tx gdb.TX, roles []string) error {
	seen := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		if _, duplicate := seen[role]; duplicate {
			continue
		}
		seen[role] = struct{}{}
		exists, err := tx.Model("roles").Ctx(ctx).Where("slug", role).Exist()
		if err != nil {
			return err
		}
		if !exists {
			return repo.UnknownRoleError{Slug: role}
		}
	}
	return nil
}

func insertRoles(ctx context.Context, tx gdb.TX, identityID string, roles []string) error {
	seen := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		if _, duplicate := seen[role]; duplicate {
			continue
		}
		seen[role] = struct{}{}
		if _, err := tx.Exec(
			"INSERT INTO identity_roles (identity_id, role_slug) VALUES (?, ?)", identityID, role,
		); err != nil {
			return err
		}
	}
	return nil
}

// SetProfileImage updates one image's url + asset-id columns (kind "avatar" |
// "cover") without touching the other editable fields — used by the avatar/cover
// upload proxy, which commits the image immediately after a successful upload.
func (p *PG) SetProfileImage(ctx context.Context, identityID, kind, mediaKey, assetID string) error {
	mediaKeyCol, idCol := "avatar_media_key", "avatar_asset_id"
	if kind == "cover" {
		mediaKeyCol, idCol = "cover_media_key", "cover_asset_id"
	}
	res, err := p.db.Model("user_profiles").Ctx(ctx).Where("identity_id", identityID).
		Data(g.Map{mediaKeyCol: mediaKey, idCol: assetID}).Update()
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrIdentityMissing
	}
	return nil
}

// orDefault returns v if non-blank, otherwise d.
func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

// nilIfEmpty returns nil when s is blank (empty or whitespace-only), otherwise
// the ORIGINAL string s. Used to map empty-string sentinel values to SQL NULL
// for UUID columns and to keep nullable TEXT columns NULL rather than storing
// whitespace-only junk.
func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

var errPublicKeyCollision = errors.New("public user key collision")

func isPublicKeyCollision(err error) bool {
	return isUniqueViolation(err) && strings.Contains(err.Error(), "uq_identities_user_key")
}

// Compile-time interface assertion.
var _ repo.IdentityRepo = (*PG)(nil)
var _ repo.SessionStore = (*PG)(nil)
