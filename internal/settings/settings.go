// Package settings — настройки приложения с хранением в БД (ROADMAP §9).
// Приоритет источников: env > yaml > БД > дефолт. Значение, явно заданное
// в конфиге (env/yaml), в админке показывается, но заблокировано —
// «управляется конфигурацией» (паттерн Grafana/GitLab).
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"

	"team-docs/internal/config"
)

// Source — происхождение действующего значения.
type Source string

const (
	SourceEnv     Source = "env"
	SourceYAML    Source = "yaml"
	SourceDB      Source = "db"
	SourceDefault Source = "default"
)

// Definition описывает управляемый ключ.
type Definition struct {
	Key      string   // "branding.appName"
	Label    string   // подпись в админке
	Kind     string   // "string" | "int"
	EnvVar   string   // TEAMDOCS_BRANDING__APPNAME
	YamlPath []string // ["branding", "appName"]
	// configValue — значение из загруженного конфига (env/yaml уже слиты sconf).
	configValue any
	// defaultValue — дефолт (когда конфиг ключ не задавал).
	defaultValue any
}

// Item — настройка для админки: значение + источник + редактируемость.
type Item struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Value    any    `json:"value"`
	Source   Source `json:"source"`
	Editable bool   `json:"editable"`
}

// Service отдаёт действующие значения и принимает правки админки.
type Service struct {
	pool *pgxpool.Pool
	defs []Definition
	// заданные конфигом ключи (env/yaml) — заблокированы для правки.
	lockedEnv  map[string]bool
	lockedYAML map[string]bool

	mu sync.RWMutex
	db map[string]any // overlay из таблицы settings
}

// New строит сервис. yamlFile — путь к appsettings.yaml (может не существовать).
func New(pool *pgxpool.Pool, cfg *config.Settings, yamlFile string) (*Service, error) {
	defaults := defaultsOf()
	s := &Service{
		pool:       pool,
		lockedEnv:  map[string]bool{},
		lockedYAML: map[string]bool{},
		db:         map[string]any{},
		defs: []Definition{
			{Key: "branding.appName", Label: "Название приложения", Kind: "string",
				EnvVar: "TEAMDOCS_BRANDING__APPNAME", YamlPath: []string{"branding", "appName"},
				configValue: cfg.Branding.AppName, defaultValue: defaults.Branding.AppName},
			{Key: "branding.workspaceName", Label: "Название пространства", Kind: "string",
				EnvVar: "TEAMDOCS_BRANDING__WORKSPACENAME", YamlPath: []string{"branding", "workspaceName"},
				configValue: cfg.Branding.WorkspaceName, defaultValue: defaults.Branding.WorkspaceName},
			{Key: "branding.monogram", Label: "Монограмма", Kind: "string",
				EnvVar: "TEAMDOCS_BRANDING__MONOGRAM", YamlPath: []string{"branding", "monogram"},
				configValue: cfg.Branding.Monogram, defaultValue: defaults.Branding.Monogram},
			{Key: "branding.theme", Label: "Тема по умолчанию", Kind: "string",
				EnvVar: "TEAMDOCS_BRANDING__THEME", YamlPath: []string{"branding", "theme"},
				configValue: cfg.Branding.Theme, defaultValue: defaults.Branding.Theme},
			{Key: "maxUploadBytes", Label: "Лимит загрузки, байт", Kind: "int",
				EnvVar: "TEAMDOCS_MAXUPLOADBYTES", YamlPath: []string{"maxUploadBytes"},
				configValue: cfg.MaxUploadBytes, defaultValue: defaults.MaxUploadBytes},
		},
	}

	for _, d := range s.defs {
		if _, ok := os.LookupEnv(d.EnvVar); ok {
			s.lockedEnv[d.Key] = true
		}
	}
	if raw, err := os.ReadFile(yamlFile); err == nil {
		var doc map[string]any
		if err := yaml.Unmarshal(raw, &doc); err == nil {
			for _, d := range s.defs {
				if yamlHas(doc, d.YamlPath) {
					s.lockedYAML[d.Key] = true
				}
			}
		}
	}
	return s, s.reload(context.Background())
}

// defaultsOf — дефолтные значения (sconf без файла и env).
func defaultsOf() *config.Settings {
	d := &config.Settings{}
	d.Branding = config.BrandingSettings{AppName: "team-docs", WorkspaceName: "рабочее пространство", Monogram: "td"}
	d.MaxUploadBytes = 20971520
	return d
}

func yamlHas(doc map[string]any, path []string) bool {
	cur := any(doc)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		if cur, ok = m[p]; !ok {
			return false
		}
	}
	return true
}

// reload перечитывает overlay из таблицы settings.
func (s *Service) reload(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return fmt.Errorf("settings load: %w", err)
	}
	defer rows.Close()
	db := map[string]any{}
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			return err
		}
		var v any
		if json.Unmarshal(raw, &v) == nil {
			db[key] = v
		}
	}
	s.mu.Lock()
	s.db = db
	s.mu.Unlock()
	return rows.Err()
}

func (s *Service) def(key string) *Definition {
	for i := range s.defs {
		if s.defs[i].Key == key {
			return &s.defs[i]
		}
	}
	return nil
}

// resolve возвращает действующее значение и источник.
func (s *Service) resolve(d *Definition) (any, Source) {
	if s.lockedEnv[d.Key] {
		return d.configValue, SourceEnv
	}
	if s.lockedYAML[d.Key] {
		return d.configValue, SourceYAML
	}
	s.mu.RLock()
	v, ok := s.db[d.Key]
	s.mu.RUnlock()
	if ok {
		return v, SourceDB
	}
	return d.defaultValue, SourceDefault
}

// List — все настройки для админки.
func (s *Service) List() []Item {
	out := make([]Item, 0, len(s.defs))
	for i := range s.defs {
		d := &s.defs[i]
		v, src := s.resolve(d)
		out = append(out, Item{
			Key: d.Key, Label: d.Label, Kind: d.Kind, Value: v, Source: src,
			Editable: src == SourceDB || src == SourceDefault,
		})
	}
	return out
}

// Set сохраняет значение в БД (с аудитом). Ключи, заданные конфигом, менять
// нельзя — вернётся ошибка.
func (s *Service) Set(ctx context.Context, key string, value any, userID *int64) error {
	d := s.def(key)
	if d == nil {
		return fmt.Errorf("неизвестная настройка %q", key)
	}
	if s.lockedEnv[key] || s.lockedYAML[key] {
		return fmt.Errorf("настройка %q управляется конфигурацией (env/yaml) и не редактируется", key)
	}
	switch d.Kind {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("настройка %q ожидает строку", key)
		}
	case "int":
		f, ok := value.(float64) // JSON-числа приходят как float64
		if !ok || f != float64(int64(f)) || f < 0 {
			return fmt.Errorf("настройка %q ожидает целое число ≥ 0", key)
		}
		value = int64(f)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO settings (key, value, updated_at, updated_by)
		VALUES ($1, $2, now(), $3)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now(), updated_by = $3`,
		key, raw, userID); err != nil {
		return err
	}
	s.mu.Lock()
	var v any
	_ = json.Unmarshal(raw, &v)
	s.db[key] = v
	s.mu.Unlock()
	return nil
}

// --- Типизированные геттеры для рантайма ---

func (s *Service) getString(key string) string {
	d := s.def(key)
	v, _ := s.resolve(d)
	if str, ok := v.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", v)
}

// Branding — действующий брендинг (для GET /api/branding).
func (s *Service) Branding() config.BrandingSettings {
	return config.BrandingSettings{
		AppName:       s.getString("branding.appName"),
		WorkspaceName: s.getString("branding.workspaceName"),
		Monogram:      s.getString("branding.monogram"),
		Theme:         s.getString("branding.theme"),
	}
}

// MaxUploadBytes — действующий лимит загрузки.
func (s *Service) MaxUploadBytes() int64 {
	d := s.def("maxUploadBytes")
	v, _ := s.resolve(d)
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		var out int64
		_, _ = fmt.Sscanf(strings.TrimSpace(n), "%d", &out)
		return out
	}
	return 20971520
}
