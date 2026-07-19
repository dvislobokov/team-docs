package pages_test

// Интеграционный тест тегов: сохранение через UpdatePage, nil не трогает
// теги (MCP-сценарий), ListTags со счётчиками, фильтр тега в поиске.

import (
	"context"
	"testing"

	"team-docs/internal/store"
)

func TestPageTags(t *testing.T) {
	pool := testPool(t)
	q := store.New(pool)
	ctx := context.Background()

	page := createPage(t, pool, nil, "it-tags-a")
	upd, err := q.UpdatePage(ctx, store.UpdatePageParams{
		ID: page.ID, Title: page.Title, Content: []byte(`[]`),
		ContentText: "содержимое про теги и настройки",
		Version:     page.Version,
		Tags:        []string{"docs", "инфра"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.Tags) != 2 || upd.Tags[0] != "docs" {
		t.Fatalf("tags = %v", upd.Tags)
	}

	// nil → теги не трогаются (запись без поддержки тегов, MCP).
	upd, err = q.UpdatePage(ctx, store.UpdatePageParams{
		ID: page.ID, Title: page.Title, Content: []byte(`[]`),
		ContentText: "содержимое про теги и настройки",
		Version:     upd.Version, Tags: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.Tags) != 2 {
		t.Fatalf("nil-теги перезаписали значение: %v", upd.Tags)
	}

	// Вторая страница с общим тегом — для счётчиков.
	page2 := createPage(t, pool, nil, "it-tags-b")
	if _, err := q.UpdatePage(ctx, store.UpdatePageParams{
		ID: page2.ID, Title: page2.Title, Content: []byte(`[]`),
		ContentText: "другая страница", Version: page2.Version,
		Tags: []string{"docs"},
	}); err != nil {
		t.Fatal(err)
	}

	mainID := mainProjectID(t, q)
	rows, err := q.ListTags(ctx, mainID)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, r := range rows {
		counts[r.Tag] = r.Pages
	}
	if counts["docs"] < 2 || counts["инфра"] < 1 {
		t.Fatalf("счётчики тегов: %v", counts)
	}

	// Поиск с фильтром тега: «теги» есть на обеих... контент разный —
	// запрос «содержимое» матчит обе, фильтр tag=инфра оставляет одну.
	tag := "инфра"
	hits, err := q.SearchPages(ctx, store.SearchPagesParams{
		PlaintoTsquery: "содержимое", ProjectIds: []int64{mainID}, Tag: &tag,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.ID == page2.ID {
			t.Fatal("фильтр tag=инфра не должен пропускать страницу без этого тега")
		}
	}
	found := false
	for _, h := range hits {
		if h.ID == page.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("страница с тегом «инфра» не найдена при фильтре")
	}
}
