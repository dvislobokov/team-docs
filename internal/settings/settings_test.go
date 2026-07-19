package settings

// Интеграционные тесты настроек: провенанс env > yaml > БД > дефолт,
// блокировка конфигом, запись с валидацией.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/config"
	"team-docs/internal/db"
)

func settingsPool(t *testing.T) *pgxpool.Pool {
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
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM settings`) })
	return pool
}

func item(t *testing.T, s *Service, key string) Item {
	t.Helper()
	for _, it := range s.List() {
		if it.Key == key {
			return it
		}
	}
	t.Fatalf("настройка %q не найдена", key)
	return Item{}
}

func TestProvenanceAndLocking(t *testing.T) {
	pool := settingsPool(t)
	ctx := context.Background()

	// yaml задаёт monogram; env задаёт appName; workspaceName свободен.
	yamlFile := filepath.Join(t.TempDir(), "appsettings.yaml")
	if err := os.WriteFile(yamlFile, []byte("branding:\n  monogram: yml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEAMDOCS_BRANDING__APPNAME", "env-app")

	cfg := &config.Settings{}
	cfg.Branding = config.BrandingSettings{AppName: "env-app", Monogram: "yml", WorkspaceName: "рабочее пространство"}
	cfg.MaxUploadBytes = 20971520

	s, err := New(pool, cfg, yamlFile)
	if err != nil {
		t.Fatal(err)
	}

	if it := item(t, s, "branding.appName"); it.Source != SourceEnv || it.Editable {
		t.Fatalf("appName: source=%s editable=%v, ожидался env/false", it.Source, it.Editable)
	}
	if it := item(t, s, "branding.monogram"); it.Source != SourceYAML || it.Editable {
		t.Fatalf("monogram: source=%s editable=%v, ожидался yaml/false", it.Source, it.Editable)
	}
	if it := item(t, s, "branding.workspaceName"); it.Source != SourceDefault || !it.Editable {
		t.Fatalf("workspaceName: source=%s editable=%v, ожидался default/true", it.Source, it.Editable)
	}

	// Заблокированные конфигом ключи не пишутся.
	if err := s.Set(ctx, "branding.appName", "hack", nil); err == nil {
		t.Fatal("запись env-ключа должна отклоняться")
	}
	// Валидация типа.
	if err := s.Set(ctx, "maxUploadBytes", "not-a-number", nil); err == nil {
		t.Fatal("строка в int-ключ должна отклоняться")
	}

	// Свободный ключ пишется, источник становится db, значение действует.
	if err := s.Set(ctx, "branding.workspaceName", "Моя команда", nil); err != nil {
		t.Fatalf("set: %v", err)
	}
	if it := item(t, s, "branding.workspaceName"); it.Source != SourceDB || it.Value != "Моя команда" {
		t.Fatalf("после set: %+v", it)
	}
	if b := s.Branding(); b.WorkspaceName != "Моя команда" || b.AppName != "env-app" {
		t.Fatalf("Branding() = %+v", b)
	}

	// Новый инстанс (рестарт) читает overlay из БД.
	s2, err := New(pool, cfg, yamlFile)
	if err != nil {
		t.Fatal(err)
	}
	if b := s2.Branding(); b.WorkspaceName != "Моя команда" {
		t.Fatalf("после перезапуска WorkspaceName = %q", b.WorkspaceName)
	}

	// int-настройка через float64 (как из JSON).
	if err := s.Set(ctx, "maxUploadBytes", float64(1048576), nil); err != nil {
		t.Fatal(err)
	}
	if got := s.MaxUploadBytes(); got != 1048576 {
		t.Fatalf("MaxUploadBytes = %d", got)
	}
}
