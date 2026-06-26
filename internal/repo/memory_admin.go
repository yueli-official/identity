package repo

import (
	"context"
	"sort"
	"strings"

	"platform/services/identity/internal/model"
)

// AdminListUsers filters/sorts/pages the in-memory identities, mirroring the PG
// implementation (default view hides soft-deleted; keyword matches email /
// display_name / username case-insensitively).
func (m *Memory) AdminListUsers(_ context.Context, f AdminUserFilter) ([]AdminUserRow, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	kw := strings.ToLower(strings.TrimSpace(f.Keyword))
	var matched []AdminUserRow
	for id, idn := range m.byID {
		if f.Status != "" {
			if string(idn.Status) != f.Status {
				continue
			}
		} else if idn.Status == model.StatusDeleted {
			continue
		}
		prof := m.profiles[id]
		if kw != "" {
			hay := strings.ToLower(idn.Email + " " + prof.DisplayName + " " + prof.Username)
			if !strings.Contains(hay, kw) {
				continue
			}
		}
		roles := m.roleSlugsLocked(id)
		if f.Role != "" && !contains(roles, f.Role) {
			continue
		}
		matched = append(matched, AdminUserRow{
			ID:            idn.ID,
			Email:         idn.Email,
			EmailVerified: idn.EmailVerified,
			Status:        idn.Status,
			CreatedAt:     idn.CreatedAt,
			DisplayName:   prof.DisplayName,
			Username:      prof.Username,
			AvatarURL:     prof.AvatarURL,
			Roles:         roles,
		})
	}

	asc := f.Order == "asc"
	sort.Slice(matched, func(i, j int) bool {
		var less bool
		if f.OrderBy == "display_name" {
			less = matched[i].DisplayName < matched[j].DisplayName
		} else {
			less = matched[i].CreatedAt.Before(matched[j].CreatedAt)
		}
		if asc {
			return less
		}
		return !less
	})

	total := len(matched)
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	lo := f.Offset
	if lo > total {
		lo = total
	}
	hi := lo + limit
	if hi > total {
		hi = total
	}
	return matched[lo:hi], total, nil
}

// roleSlugsLocked returns the sorted role slugs for an identity. Caller holds m.mu.
func (m *Memory) roleSlugsLocked(id string) []string {
	set := m.roles[id]
	slugs := make([]string, 0, len(set))
	for s := range set {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	return slugs
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

// AdminUserStatusCounts counts identities per status plus a non-deleted total.
func (m *Memory) AdminUserStatusCounts(_ context.Context) (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]int{"active": 0, "disabled": 0, "deleted": 0}
	for _, idn := range m.byID {
		out[string(idn.Status)]++
	}
	out["total"] = out["active"] + out["disabled"]
	return out, nil
}

// SetIdentityStatus updates an identity's status. ErrIdentityMissing if absent.
func (m *Memory) SetIdentityStatus(_ context.Context, identityID string, status model.Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	idn, ok := m.byID[identityID]
	if !ok {
		return ErrIdentityMissing
	}
	idn.Status = status
	idn.UpdatedAt = m.now()
	m.byID[identityID] = idn
	return nil
}
