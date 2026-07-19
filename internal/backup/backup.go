// Package backup реализует полный экспорт/импорт содержимого БД team-docs
// (страницы, ревизии, файлы, диаграммы) одним JSON-дампом. Работает на уровне
// приложения через pgx — без внешних утилит (pg_dump), в духе «одного бинаря».
package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DumpVersion — текущая версия формата дампа. Импорт принимает текущую и
// v1 (до users/projects/корзины): старым страницам назначается дефолтный
// проект, авторство и deleted_at остаются пустыми.
const DumpVersion = 2

// Dump — полный снимок содержимого БД.
type Dump struct {
	Version    int           `json:"version"`
	ExportedAt time.Time     `json:"exportedAt"`
	Users      []UserRow     `json:"users,omitempty"`
	Projects   []ProjectRow  `json:"projects,omitempty"`
	Pages      []PageRow     `json:"pages"`
	Revisions  []RevisionRow `json:"revisions"`
	Files      []FileRow     `json:"files"`
	Diagrams   []DiagramRow  `json:"diagrams"`
}

type UserRow struct {
	ID         int64     `json:"id"`
	Subject    string    `json:"subject"`
	Username   string    `json:"username"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

type ProjectRow struct {
	ID        int64     `json:"id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"createdAt"`
}

type PageRow struct {
	ID          int64           `json:"id"`
	ParentID    *int64          `json:"parentId"`
	Title       string          `json:"title"`
	Content     json.RawMessage `json:"content"`
	ContentText string          `json:"contentText"`
	Position    int32           `json:"position"`
	Version     int32           `json:"version"`
	Icon        string          `json:"icon"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	// v2: проект, авторство, корзина. В v1-дампах отсутствуют.
	ProjectID int64      `json:"projectId,omitempty"`
	CreatedBy *int64     `json:"createdBy,omitempty"`
	UpdatedBy *int64     `json:"updatedBy,omitempty"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

type RevisionRow struct {
	ID        int64           `json:"id"`
	PageID    int64           `json:"pageId"`
	Version   int32           `json:"version"`
	Title     string          `json:"title"`
	Content   json.RawMessage `json:"content"`
	CreatedAt time.Time       `json:"createdAt"`
	AuthorID  *int64          `json:"authorId,omitempty"` // v2
}

type FileRow struct {
	ID        uuid.UUID `json:"id"`
	PageID    *int64    `json:"pageId"`
	Filename  string    `json:"filename"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	Content   []byte    `json:"content"` // JSON-кодируется как base64
	CreatedAt time.Time `json:"createdAt"`
}

type DiagramRow struct {
	ID        uuid.UUID `json:"id"`
	PageID    *int64    `json:"pageId"`
	Xml       string    `json:"xml"`
	Preview   []byte    `json:"preview"` // base64
	UpdatedAt time.Time `json:"updatedAt"`
}

// Service выполняет экспорт/импорт поверх пула соединений.
type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Export читает все таблицы в один Dump. Порядок по id — стабильный.
func (s *Service) Export(ctx context.Context) (*Dump, error) {
	d := &Dump{
		Version:    DumpVersion,
		ExportedAt: time.Now().UTC(),
		Users:      []UserRow{},
		Projects:   []ProjectRow{},
		Pages:      []PageRow{},
		Revisions:  []RevisionRow{},
		Files:      []FileRow{},
		Diagrams:   []DiagramRow{},
	}

	if err := s.eachRow(ctx,
		`SELECT id, subject, username, name, email, created_at, last_seen_at
		 FROM users ORDER BY id`,
		func(rows pgx.Rows) error {
			var u UserRow
			if err := rows.Scan(&u.ID, &u.Subject, &u.Username, &u.Name, &u.Email,
				&u.CreatedAt, &u.LastSeenAt); err != nil {
				return err
			}
			d.Users = append(d.Users, u)
			return nil
		}); err != nil {
		return nil, fmt.Errorf("export users: %w", err)
	}

	if err := s.eachRow(ctx,
		`SELECT id, key, name, icon, created_at FROM projects ORDER BY id`,
		func(rows pgx.Rows) error {
			var p ProjectRow
			if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Icon, &p.CreatedAt); err != nil {
				return err
			}
			d.Projects = append(d.Projects, p)
			return nil
		}); err != nil {
		return nil, fmt.Errorf("export projects: %w", err)
	}

	if err := s.eachRow(ctx,
		`SELECT id, parent_id, title, content, content_text, position, version, icon,
		        created_at, updated_at, project_id, created_by, updated_by, deleted_at
		 FROM pages ORDER BY id`,
		func(rows pgx.Rows) error {
			var p PageRow
			var content []byte
			if err := rows.Scan(&p.ID, &p.ParentID, &p.Title, &content, &p.ContentText,
				&p.Position, &p.Version, &p.Icon, &p.CreatedAt, &p.UpdatedAt,
				&p.ProjectID, &p.CreatedBy, &p.UpdatedBy, &p.DeletedAt); err != nil {
				return err
			}
			p.Content = json.RawMessage(content)
			d.Pages = append(d.Pages, p)
			return nil
		}); err != nil {
		return nil, fmt.Errorf("export pages: %w", err)
	}

	if err := s.eachRow(ctx,
		`SELECT id, page_id, version, title, content, created_at, author_id
		 FROM page_revisions ORDER BY id`,
		func(rows pgx.Rows) error {
			var r RevisionRow
			var content []byte
			if err := rows.Scan(&r.ID, &r.PageID, &r.Version, &r.Title, &content,
				&r.CreatedAt, &r.AuthorID); err != nil {
				return err
			}
			r.Content = json.RawMessage(content)
			d.Revisions = append(d.Revisions, r)
			return nil
		}); err != nil {
		return nil, fmt.Errorf("export revisions: %w", err)
	}

	if err := s.eachRow(ctx,
		`SELECT id, page_id, filename, mime, size, content, created_at
		 FROM files ORDER BY created_at, id`,
		func(rows pgx.Rows) error {
			var f FileRow
			if err := rows.Scan(&f.ID, &f.PageID, &f.Filename, &f.Mime, &f.Size, &f.Content, &f.CreatedAt); err != nil {
				return err
			}
			d.Files = append(d.Files, f)
			return nil
		}); err != nil {
		return nil, fmt.Errorf("export files: %w", err)
	}

	if err := s.eachRow(ctx,
		`SELECT id, page_id, xml, preview, updated_at
		 FROM diagrams ORDER BY updated_at, id`,
		func(rows pgx.Rows) error {
			var dg DiagramRow
			if err := rows.Scan(&dg.ID, &dg.PageID, &dg.Xml, &dg.Preview, &dg.UpdatedAt); err != nil {
				return err
			}
			d.Diagrams = append(d.Diagrams, dg)
			return nil
		}); err != nil {
		return nil, fmt.Errorf("export diagrams: %w", err)
	}

	return d, nil
}

// eachRow выполняет запрос и вызывает scan для каждой строки, гарантируя Close.
func (s *Service) eachRow(ctx context.Context, sql string, scan func(pgx.Rows) error) error {
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Import ПОЛНОСТЬЮ заменяет содержимое БД данными из dump в одной транзакции:
// TRUNCATE всех таблиц → вставка с сохранением исходных id → правка sequence.
// Любая ошибка откатывает транзакцию — БД остаётся нетронутой.
// Принимает текущую версию и v1 (страницы уезжают в дефолтный проект 'main').
func (s *Service) Import(ctx context.Context, d *Dump) error {
	if d == nil {
		return fmt.Errorf("пустой дамп")
	}
	if d.Version != DumpVersion && d.Version != 1 {
		return fmt.Errorf("неподдерживаемая версия дампа %d (ожидается %d или 1)", d.Version, DumpVersion)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op после успешного Commit

	if _, err := tx.Exec(ctx,
		`TRUNCATE pages, page_revisions, files, diagrams, users, projects RESTART IDENTITY CASCADE`); err != nil {
		return fmt.Errorf("очистка таблиц: %w", err)
	}

	for _, u := range d.Users {
		if _, err := tx.Exec(ctx,
			`INSERT INTO users (id, subject, username, name, email, created_at, last_seen_at)
			 OVERRIDING SYSTEM VALUE VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			u.ID, u.Subject, u.Username, u.Name, u.Email, u.CreatedAt, u.LastSeenAt,
		); err != nil {
			return fmt.Errorf("вставка пользователя %d: %w", u.ID, err)
		}
	}

	for _, p := range d.Projects {
		if _, err := tx.Exec(ctx,
			`INSERT INTO projects (id, key, name, icon, created_at)
			 OVERRIDING SYSTEM VALUE VALUES ($1, $2, $3, $4, $5)`,
			p.ID, p.Key, p.Name, p.Icon, p.CreatedAt,
		); err != nil {
			return fmt.Errorf("вставка проекта %d: %w", p.ID, err)
		}
	}

	// v1-дампы (и страницы без проекта) требуют дефолтный проект.
	var defaultProject int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO projects (key, name) VALUES ('main', 'Основное пространство')
		ON CONFLICT (key) DO UPDATE SET name = projects.name
		RETURNING id`).Scan(&defaultProject); err != nil {
		return fmt.Errorf("дефолтный проект: %w", err)
	}

	// Страницы, проход 1: без parent_id, чтобы не зависеть от порядка вставки
	// (FK самоссылки pages.parent_id → pages.id не отложенный).
	for _, p := range d.Pages {
		content := p.Content
		if len(content) == 0 {
			content = json.RawMessage("[]")
		}
		projectID := p.ProjectID
		if projectID == 0 {
			projectID = defaultProject
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO pages (id, parent_id, title, content, content_text, position, version, icon,
			                    created_at, updated_at, project_id, created_by, updated_by, deleted_at)
			 OVERRIDING SYSTEM VALUE VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			p.ID, p.Title, []byte(content), p.ContentText, p.Position, p.Version, p.Icon,
			p.CreatedAt, p.UpdatedAt, projectID, p.CreatedBy, p.UpdatedBy, p.DeletedAt,
		); err != nil {
			return fmt.Errorf("вставка страницы %d: %w", p.ID, err)
		}
	}
	// Проход 2: проставляем parent_id (все страницы уже существуют).
	for _, p := range d.Pages {
		if p.ParentID == nil {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE pages SET parent_id = $2 WHERE id = $1`, p.ID, *p.ParentID); err != nil {
			return fmt.Errorf("связывание страницы %d: %w", p.ID, err)
		}
	}

	for _, r := range d.Revisions {
		content := r.Content
		if len(content) == 0 {
			content = json.RawMessage("[]")
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO page_revisions (id, page_id, version, title, content, created_at, author_id)
			 OVERRIDING SYSTEM VALUE VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			r.ID, r.PageID, r.Version, r.Title, []byte(content), r.CreatedAt, r.AuthorID,
		); err != nil {
			return fmt.Errorf("вставка ревизии %d: %w", r.ID, err)
		}
	}

	for _, f := range d.Files {
		if _, err := tx.Exec(ctx,
			`INSERT INTO files (id, page_id, filename, mime, size, content, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			f.ID, f.PageID, f.Filename, f.Mime, f.Size, f.Content, f.CreatedAt,
		); err != nil {
			return fmt.Errorf("вставка файла %s: %w", f.ID, err)
		}
	}

	for _, dg := range d.Diagrams {
		if _, err := tx.Exec(ctx,
			`INSERT INTO diagrams (id, page_id, xml, preview, updated_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			dg.ID, dg.PageID, dg.Xml, dg.Preview, dg.UpdatedAt,
		); err != nil {
			return fmt.Errorf("вставка диаграммы %s: %w", dg.ID, err)
		}
	}

	// Sequence identity-колонок сбит после вставки явных id — выставляем на MAX+1.
	// setval(..., false) => следующий nextval вернёт именно это значение.
	for _, tbl := range []string{"pages", "page_revisions", "users", "projects"} {
		if _, err := tx.Exec(ctx, fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('%s', 'id'),
			                (SELECT COALESCE(MAX(id), 0) + 1 FROM %s), false)`, tbl, tbl)); err != nil {
			return fmt.Errorf("сброс sequence %s: %w", tbl, err)
		}
	}

	return tx.Commit(ctx)
}
