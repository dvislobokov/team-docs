// Package projects — проекты (пространства) и их ролевая модель (ROADMAP §10).
package projects

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"team-docs/internal/auth"
	"team-docs/internal/store"
)

// Видимость проекта.
const (
	VisPublic   = "public"   // читают все (анонимы — при publicRead)
	VisInternal = "internal" // все вошедшие
	VisPrivate  = "private"  // только участники
)

// RoleNone — доступа нет (проект невидим для пользователя).
const RoleNone = ""

// RoleFor вычисляет роль пользователя в проекте:
//   - открытый режим (auth off) → admin (локальная разработка);
//   - глобальный админ → admin;
//   - явное членство перекрывает глобальную роль (в т.ч. ограничивает);
//   - иначе по видимости: public/internal → глобальная роль
//     (reader/editor через CanEdit), private → нет доступа;
//   - аноним: public → reader, иначе нет доступа.
func RoleFor(ctx context.Context, q *store.Queries, a *auth.Authenticator, u *auth.User, project store.Project) (string, error) {
	if !a.Enabled() {
		return auth.RoleAdmin, nil
	}
	if u != nil && a.IsAdmin(u) {
		return auth.RoleAdmin, nil
	}
	if u != nil && u.ID != 0 {
		// 1) Личное членство.
		role, err := q.GetProjectMemberRole(ctx, store.GetProjectMemberRoleParams{
			ProjectID: project.ID, UserID: u.ID,
		})
		if err == nil {
			return role, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return RoleNone, err
		}
		// 2) Групповое членство (лучшая роль среди групп пользователя).
		role, err = q.GetProjectGroupRole(ctx, store.GetProjectGroupRoleParams{
			ProjectID: project.ID, UserID: u.ID,
		})
		if err == nil {
			return role, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return RoleNone, err
		}
	}
	switch project.Visibility {
	case VisPublic:
		if u == nil {
			return auth.RoleReader, nil
		}
		return globalRole(a, u), nil
	case VisInternal:
		if u == nil {
			return RoleNone, nil
		}
		return globalRole(a, u), nil
	default: // private
		return RoleNone, nil
	}
}

func globalRole(a *auth.Authenticator, u *auth.User) string {
	if a.CanEdit(u) {
		return auth.RoleEditor
	}
	return auth.RoleReader
}

// CanRead/CanWrite/IsProjectAdmin — интерпретация роли.
func CanRead(role string) bool  { return role != RoleNone }
func CanWrite(role string) bool { return role == auth.RoleEditor || role == auth.RoleAdmin }
func IsProjectAdmin(role string) bool {
	return role == auth.RoleAdmin
}

// RoleForID — как RoleFor, но по id проекта.
func RoleForID(ctx context.Context, q *store.Queries, a *auth.Authenticator, u *auth.User, projectID int64) (string, error) {
	p, err := q.GetProject(ctx, projectID)
	if err != nil {
		return RoleNone, err
	}
	return RoleFor(ctx, q, a, u, p)
}

// AccessibleIDs — id проектов, доступных пользователю на чтение (для поиска).
func AccessibleIDs(ctx context.Context, q *store.Queries, a *auth.Authenticator, u *auth.User) ([]int64, error) {
	list, err := q.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	var out []int64
	for _, p := range list {
		role, err := RoleFor(ctx, q, a, u, p)
		if err != nil {
			return nil, err
		}
		if CanRead(role) {
			out = append(out, p.ID)
		}
	}
	return out, nil
}
