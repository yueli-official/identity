package dao

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// adminListCols is the projected column set for the admin user list (qualified
// so the identities/user_profiles join is unambiguous on shared column names).
const adminListCols = "i.id, i.email, i.email_verified, i.status, i.created_at, " +
	"p.display_name, p.username, p.avatar_url"

// adminUserScan is the flat scan target for one joined identity+profile row.
type adminUserScan struct {
	ID            string    `orm:"id"`
	Email         string    `orm:"email"`
	EmailVerified bool      `orm:"email_verified"`
	Status        string    `orm:"status"`
	CreatedAt     time.Time `orm:"created_at"`
	DisplayName   string    `orm:"display_name"`
	Username      string    `orm:"username"`
	AvatarURL     string    `orm:"avatar_url"`
}

// AdminListUsers returns a filtered, paginated page of identities joined with
// their profile and role slugs, plus the total row count for the same filter
// (ignoring paging). Roles are batch-loaded in a second query to avoid N+1.
func (p *PG) AdminListUsers(ctx context.Context, f repo.AdminUserFilter) ([]repo.AdminUserRow, int, error) {
	m := p.db.Model("identities i").Ctx(ctx).
		LeftJoin("user_profiles p", "p.identity_id = i.id")

	if f.Role != "" {
		// inner-join semantics: only identities holding the role slug.
		m = m.LeftJoin("identity_roles ir", "ir.identity_id = i.id").
			Where("ir.role_slug", f.Role)
	}
	if f.Status != "" {
		m = m.Where("i.status", f.Status)
	} else {
		// default view hides soft-deleted accounts unless explicitly requested.
		m = m.Where("i.status <>", string(model.StatusDeleted))
	}
	if f.Keyword != "" {
		kw := "%" + f.Keyword + "%"
		m = m.Where("(i.email ILIKE ? OR p.display_name ILIKE ? OR p.username ILIKE ?)", kw, kw, kw)
	}

	total, err := m.Count()
	if err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	orderCol := "i.created_at"
	if f.OrderBy == "display_name" {
		orderCol = "p.display_name"
	}
	orderDir := "DESC"
	if f.Order == "asc" {
		orderDir = "ASC"
	}

	var rows []adminUserScan
	if err := m.Fields(adminListCols).
		Order(orderCol + " " + orderDir).Limit(limit).Offset(f.Offset).Scan(&rows); err != nil {
		return nil, 0, err
	}

	out := make([]repo.AdminUserRow, 0, len(rows))
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
		out = append(out, repo.AdminUserRow{
			ID:            r.ID,
			Email:         r.Email,
			EmailVerified: r.EmailVerified,
			Status:        model.Status(r.Status),
			CreatedAt:     r.CreatedAt,
			DisplayName:   r.DisplayName,
			Username:      r.Username,
			AvatarURL:     r.AvatarURL,
		})
	}

	rolesByID, err := p.rolesFor(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		out[i].Roles = rolesByID[out[i].ID]
	}
	return out, total, nil
}

// rolesFor batch-loads role slugs for the given identity ids, keyed by id.
func (p *PG) rolesFor(ctx context.Context, ids []string) (map[string][]string, error) {
	byID := map[string][]string{}
	if len(ids) == 0 {
		return byID, nil
	}
	var rr []struct {
		IdentityID string `orm:"identity_id"`
		RoleSlug   string `orm:"role_slug"`
	}
	if err := p.db.Model("identity_roles").Ctx(ctx).
		WhereIn("identity_id", ids).Order("role_slug ASC").Scan(&rr); err != nil {
		return nil, err
	}
	for _, r := range rr {
		byID[r.IdentityID] = append(byID[r.IdentityID], r.RoleSlug)
	}
	return byID, nil
}

// AdminUserStatusCounts returns the identity count per status plus a "total"
// (active + disabled, i.e. non-deleted accounts).
func (p *PG) AdminUserStatusCounts(ctx context.Context) (map[string]int, error) {
	var rows []struct {
		Status string `orm:"status"`
		C      int    `orm:"c"`
	}
	if err := p.db.Model("identities").Ctx(ctx).
		Fields("status, count(*) AS c").Group("status").Scan(&rows); err != nil {
		return nil, err
	}
	out := map[string]int{"active": 0, "disabled": 0, "deleted": 0}
	for _, r := range rows {
		out[r.Status] = r.C
	}
	out["total"] = out["active"] + out["disabled"]
	return out, nil
}

// SetIdentityStatus updates an identity's lifecycle status (and bumps updated_at).
// Returns repo.ErrIdentityMissing when no row matched.
func (p *PG) SetIdentityStatus(ctx context.Context, identityID string, status model.Status) error {
	res, err := p.db.Model("identities").Ctx(ctx).Where("id", identityID).
		Data(g.Map{"status": string(status), "updated_at": gdb.Raw("now()")}).Update()
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return repo.ErrIdentityMissing
	}
	return nil
}
