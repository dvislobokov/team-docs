package pages_test

// Интеграционные тесты избранного и шаблонов (ROADMAP §7).
// Скипаются без TEAMDOCS_TEST_DSN, как и остальные.

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/pages"
	"team-docs/internal/store"
)

func createUser(t *testing.T, pool *pgxpool.Pool, subject string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (subject, username, name, email)
		 VALUES ($1, $1, $1, $1 || '@test')
		 ON CONFLICT (subject) DO UPDATE SET username = users.username
		 RETURNING id`, subject).Scan(&id)
	if err != nil {
		t.Fatalf("создание пользователя %q: %v", subject, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func TestTemplatesLifecycle(t *testing.T) {
	pool := testPool(t)
	q := store.New(pool)
	ctx := context.Background()
	projectID := mainProjectID(t, q)

	// Шаблон: корневой, is_template = true.
	tpl, err := q.CreatePage(ctx, store.CreatePageParams{
		Title: "it-tpl-Ретро", ProjectID: projectID, IsTemplate: true,
	})
	if err != nil {
		t.Fatalf("создание шаблона: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM pages WHERE id = $1`, tpl.ID)
	})

	// Наполняем шаблон контентом и тегами.
	upd, err := q.UpdatePage(ctx, store.UpdatePageParams{
		ID: tpl.ID, Title: tpl.Title, Icon: "📋",
		Content:     []byte(`[{"type":"paragraph","content":"Повестка ретроспективы"}]`),
		ContentText: "Повестка ретроспективы",
		Version:     tpl.Version,
		Tags:        []string{"ретро"},
	})
	if err != nil {
		t.Fatalf("наполнение шаблона: %v", err)
	}
	if !upd.IsTemplate {
		t.Fatal("UpdatePage должен возвращать is_template = true для шаблона")
	}

	// Шаблон скрыт из дерева, но есть в списке шаблонов.
	tree, err := q.GetPageTree(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tree {
		if n.ID == tpl.ID {
			t.Fatal("шаблон не должен попадать в дерево страниц")
		}
	}
	tpls, err := q.ListTemplates(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range tpls {
		if item.ID == tpl.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("шаблон должен быть в ListTemplates")
	}

	// Шаблон скрыт из поиска и недавних.
	hits, err := q.SearchPages(ctx, store.SearchPagesParams{
		PlaintoTsquery: "ретроспектива", ProjectIds: []int64{projectID},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.ID == tpl.ID {
			t.Fatal("шаблон не должен находиться поиском")
		}
	}
	recent, err := q.RecentPages(ctx, []int64{projectID})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recent {
		if r.ID == tpl.ID {
			t.Fatal("шаблон не должен попадать в недавние")
		}
	}

	// Создание из шаблона: копия контента/иконки/тегов обычной страницей.
	copyRow, err := q.CreatePageFromTemplate(ctx, store.CreatePageFromTemplateParams{
		TemplateID: tpl.ID,
	})
	if err != nil {
		t.Fatalf("создание из шаблона: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM pages WHERE id = $1`, copyRow.ID)
	})
	if copyRow.Title != tpl.Title || copyRow.Icon != "📋" {
		t.Fatalf("копия должна унаследовать заголовок и иконку: %q %q", copyRow.Title, copyRow.Icon)
	}
	if len(copyRow.Tags) != 1 || copyRow.Tags[0] != "ретро" {
		t.Fatalf("копия должна унаследовать теги, получено: %v", copyRow.Tags)
	}
	page, err := q.GetPage(ctx, copyRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if page.IsTemplate {
		t.Fatal("копия из шаблона не должна быть шаблоном")
	}
	if string(page.Content) != string(upd.Content) {
		t.Fatalf("контент копии должен совпадать с шаблоном:\n%s\n%s", page.Content, upd.Content)
	}
	tree, err = q.GetPageTree(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	inTree := false
	for _, n := range tree {
		if n.ID == copyRow.ID {
			inTree = true
		}
	}
	if !inTree {
		t.Fatal("копия из шаблона должна попадать в дерево")
	}
}

func TestFavoritesLifecycle(t *testing.T) {
	pool := testPool(t)
	q := store.New(pool)
	ctx := context.Background()

	uid := createUser(t, pool, "it-fav-user")
	page := createPage(t, pool, nil, "it-fav-page")

	if err := q.AddFavorite(ctx, store.AddFavoriteParams{UserID: uid, PageID: page.ID}); err != nil {
		t.Fatalf("добавление в избранное: %v", err)
	}
	// Повторное добавление идемпотентно (ON CONFLICT DO NOTHING).
	if err := q.AddFavorite(ctx, store.AddFavoriteParams{UserID: uid, PageID: page.ID}); err != nil {
		t.Fatalf("повторное добавление: %v", err)
	}

	favs, err := q.ListFavorites(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(favs) != 1 || favs[0].PageID != page.ID || favs[0].Title != page.Title {
		t.Fatalf("избранное: %+v", favs)
	}

	// Удалённая страница пропадает из списка, после восстановления — возвращается.
	if err := pages.SoftDelete(ctx, pool, page.ID); err != nil {
		t.Fatal(err)
	}
	if favs, err = q.ListFavorites(ctx, uid); err != nil || len(favs) != 0 {
		t.Fatalf("удалённая страница должна пропасть из избранного: %v %v", favs, err)
	}
	if err := pages.Restore(ctx, pool, page.ID); err != nil {
		t.Fatal(err)
	}
	if favs, err = q.ListFavorites(ctx, uid); err != nil || len(favs) != 1 {
		t.Fatalf("после восстановления избранное должно вернуться: %v %v", favs, err)
	}

	// Снятие звезды.
	if err := q.RemoveFavorite(ctx, store.RemoveFavoriteParams{UserID: uid, PageID: page.ID}); err != nil {
		t.Fatal(err)
	}
	if favs, err = q.ListFavorites(ctx, uid); err != nil || len(favs) != 0 {
		t.Fatalf("после удаления из избранного список должен быть пуст: %v %v", favs, err)
	}

	// FK-каскад: избранное не мешает окончательному удалению страницы.
	if err := q.AddFavorite(ctx, store.AddFavoriteParams{UserID: uid, PageID: page.ID}); err != nil {
		t.Fatal(err)
	}
	if err := pages.SoftDelete(ctx, pool, page.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := q.PurgePage(ctx, page.ID); err != nil || n == 0 {
		t.Fatalf("purge избранной страницы: n=%d err=%v", n, err)
	}
}
