package projects

// Интеграционный тест матрицы доступа §10: видимость проекта × членство ×
// глобальная роль. Auth включён (HS256-конфиг), пользователи создаются в БД.

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/auth"
	"team-docs/internal/config"
	"team-docs/internal/db"
	"team-docs/internal/store"
)

func projPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEAMDOCS_TEST_DSN")
	if dsn == "" {
		t.Skip("TEAMDOCS_TEST_DSN не задан — пропускаю интеграционный тест")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func mkUser(t *testing.T, q *store.Queries, subject, role string) *auth.User {
	t.Helper()
	row, err := q.UpsertUser(context.Background(), store.UpsertUserParams{
		Subject: subject, Username: subject, Name: subject, Role: role,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetUserRole(context.Background(), store.SetUserRoleParams{ID: row.ID, Role: role}); err != nil {
		t.Fatal(err)
	}
	return &auth.User{ID: row.ID, Subject: subject, Name: subject, Role: role}
}

func mkProject(t *testing.T, q *store.Queries, key, visibility string) store.Project {
	t.Helper()
	p, err := q.CreateProject(context.Background(), store.CreateProjectParams{
		Key: key, Name: key, Visibility: visibility,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAccessMatrix(t *testing.T) {
	pool := projPool(t)
	q := store.New(pool)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE key LIKE 'am-%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE subject LIKE 'am-%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM groups WHERE name LIKE 'am-%'`)
	})

	a, err := auth.New(config.AuthSettings{Enabled: true, HMACSecret: "x", PublicRead: true})
	if err != nil {
		t.Fatal(err)
	}

	editor := mkUser(t, q, "am-editor", auth.RoleEditor)
	reader := mkUser(t, q, "am-reader", auth.RoleReader)
	globalAdmin := mkUser(t, q, "am-admin", auth.RoleAdmin)
	member := mkUser(t, q, "am-member", auth.RoleReader) // глобально reader

	pub := mkProject(t, q, "am-pub", VisPublic)
	internal := mkProject(t, q, "am-int", VisInternal)
	private := mkProject(t, q, "am-priv", VisPrivate)

	// member — редактор приватного проекта (членство поднимает роль).
	if err := q.UpsertProjectMember(ctx, store.UpsertProjectMemberParams{
		ProjectID: private.ID, UserID: member.ID, Role: auth.RoleEditor,
	}); err != nil {
		t.Fatal(err)
	}
	// editor — явно reader в internal (членство ограничивает).
	if err := q.UpsertProjectMember(ctx, store.UpsertProjectMemberParams{
		ProjectID: internal.ID, UserID: editor.ID, Role: auth.RoleReader,
	}); err != nil {
		t.Fatal(err)
	}

	// Группа: groupie (глобально reader) входит в группу-редактора private.
	groupie := mkUser(t, q, "am-groupie", auth.RoleReader)
	grp, err := q.CreateGroup(ctx, "am-team")
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AddGroupMember(ctx, store.AddGroupMemberParams{GroupID: grp.ID, UserID: groupie.ID}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertProjectGroup(ctx, store.UpsertProjectGroupParams{
		ProjectID: private.ID, GroupID: grp.ID, Role: auth.RoleEditor,
	}); err != nil {
		t.Fatal(err)
	}
	// А member (личный editor) состоит и в группе-reader — личное должно победить.
	if err := q.AddGroupMember(ctx, store.AddGroupMemberParams{GroupID: grp.ID, UserID: member.ID}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		u    *auth.User
		p    store.Project
		want string
	}{
		{"аноним в public — читатель", nil, pub, auth.RoleReader},
		{"аноним в internal — нет доступа", nil, internal, RoleNone},
		{"аноним в private — нет доступа", nil, private, RoleNone},
		{"editor в public — редактор", editor, pub, auth.RoleEditor},
		{"reader в internal — читатель", reader, internal, auth.RoleReader},
		{"editor в private без членства — нет доступа", editor, private, RoleNone},
		{"member(reader) в private — редактор (членство поднимает)", member, private, auth.RoleEditor},
		{"editor в internal — reader (членство ограничивает)", editor, internal, auth.RoleReader},
		{"глобальный админ в private — админ", globalAdmin, private, auth.RoleAdmin},
		{"группа даёт editor в private", groupie, private, auth.RoleEditor},
		{"личное членство приоритетнее группового", member, private, auth.RoleEditor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RoleFor(ctx, q, a, tc.u, tc.p)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("роль %q, ожидалась %q", got, tc.want)
			}
		})
	}

	// AccessibleIDs для reader: pub + internal, но не private.
	ids, err := AccessibleIDs(ctx, q, a, reader)
	if err != nil {
		t.Fatal(err)
	}
	has := func(id int64) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	if !has(pub.ID) || !has(internal.ID) || has(private.ID) {
		t.Fatalf("AccessibleIDs(reader) = %v", ids)
	}
}
