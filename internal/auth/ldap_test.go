package auth

// Юнит-тесты резолва имён/пресетов LDAP и break-glass локального админа;
// интеграционный тест — на живом OpenLDAP (docker-compose.test.yml, profile
// ldap; скип без TEAMDOCS_TEST_LDAP_URL). Структуру каталога тест сидирует
// сам через admin-bind.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dvislobokov/srog"
	"github.com/go-ldap/ldap/v3"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"team-docs/internal/config"
)

func TestResolveBindName(t *testing.T) {
	mk := func(cfg config.LDAPSettings) *LDAPAuthenticator {
		cfg.URL = "ldap://x"
		if cfg.BaseDN == "" {
			cfg.BaseDN = "dc=corp,dc=local"
		}
		if cfg.Preset == "" {
			cfg.Preset = "openldap"
		}
		l, err := NewLDAP(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	cases := []struct {
		name string
		cfg  config.LDAPSettings
		want string
	}{
		{"DN как есть", config.LDAPSettings{BindLogin: "cn=svc,ou=x,dc=corp,dc=local"}, "cn=svc,ou=x,dc=corp,dc=local"},
		{"шаблон DOMAIN", config.LDAPSettings{BindLogin: "svc", BindLoginTemplate: `CORP\%s`}, `CORP\svc`},
		{"шаблон UPN", config.LDAPSettings{BindLogin: "svc", BindLoginTemplate: "%s@corp.local"}, "svc@corp.local"},
		{"ad: авто-UPN из baseDN", config.LDAPSettings{Preset: "ad", BindLogin: "svc"}, "svc@corp.local"},
		{"freeipa: cn=users,cn=accounts", config.LDAPSettings{Preset: "freeipa", BindLogin: "svc"},
			"uid=svc,cn=users,cn=accounts,dc=corp,dc=local"},
		{"openldap: дефолтный шаблон", config.LDAPSettings{BindLogin: "svc"},
			"uid=svc,ou=people,dc=corp,dc=local"},
		{"openldap: свой serviceDnTemplate", config.LDAPSettings{BindLogin: "svc",
			ServiceDNTemplate: "uid=%s,ou=svc,dc=corp,dc=local"}, "uid=svc,ou=svc,dc=corp,dc=local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mk(tc.cfg).resolveBindName(); got != tc.want {
				t.Fatalf("resolveBindName = %q, ожидалось %q", got, tc.want)
			}
		})
	}

	// userFilter экранирует логин (инъекция фильтра).
	l := mk(config.LDAPSettings{})
	if f := l.userFilter("iva*)(uid=*"); strings.Contains(f, "*)(") {
		t.Fatalf("фильтр не экранирован: %s", f)
	}
}

func TestLocalAdminBreakGlass(t *testing.T) {
	pool := oauthPool(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(t.Context(), `DELETE FROM users WHERE subject = 'local:root'`)
	})

	hash, err := bcrypt.GenerateFromPassword([]byte("break-glass"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.AuthSettings{
		Enabled: true, HMACSecret: "x", PublicRead: true, Header: "Authorization",
		SessionSecret: "la-secret", SessionTTLHours: 1, DefaultRole: "editor",
		LocalAdmin: config.LocalAdminSettings{Username: "root", PasswordHash: string(hash)},
	}
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	log := srog.NewConsole()
	t.Cleanup(func() { _ = log.Close() })
	reg := NewRegistry(pool, cfg)

	e := echo.New()
	NewPasswordHandler(a, reg, nil, cfg.LocalAdmin, log).Register(e)
	api := e.Group("/api", Middleware(a, reg, log))
	NewHandler(a).Register(api)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/auth/password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	if rec := post(`{"login":"root","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("неверный пароль: code=%d", rec.Code)
	}
	rec := post(`{"login":"root","password":"break-glass"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("вход локального админа: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var session *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "td_session" {
			session = ck
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(session)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req)
	var me map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &me)
	if me["isAdmin"] != true {
		t.Fatalf("локальный админ должен быть admin: %v", me)
	}
}

// --- интеграция с живым OpenLDAP ---

const (
	ldapTestBase  = "dc=corp,dc=local"
	ldapTestAdmin = "cn=admin,dc=corp,dc=local"
	ldapTestPass  = "adminpass"
)

// seedLDAP наполняет каталог: ou=people с ivanov/petrov, ou=groups с
// docs-admins (member: ivanov). Идемпотентно.
func seedLDAP(t *testing.T, url string) {
	t.Helper()
	conn, err := ldap.DialURL(url)
	if err != nil {
		t.Skipf("LDAP недоступен: %v", err)
	}
	defer conn.Close()
	if err := conn.Bind(ldapTestAdmin, ldapTestPass); err != nil {
		t.Fatalf("admin bind: %v", err)
	}
	add := func(dn string, attrs map[string][]string) {
		req := ldap.NewAddRequest(dn, nil)
		for k, v := range attrs {
			req.Attribute(k, v)
		}
		if err := conn.Add(req); err != nil && !ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
			t.Fatalf("add %s: %v", dn, err)
		}
	}
	add("ou=people,"+ldapTestBase, map[string][]string{
		"objectClass": {"organizationalUnit"}, "ou": {"people"}})
	add("ou=groups,"+ldapTestBase, map[string][]string{
		"objectClass": {"organizationalUnit"}, "ou": {"groups"}})
	add("uid=ivanov,ou=people,"+ldapTestBase, map[string][]string{
		"objectClass": {"inetOrgPerson"}, "uid": {"ivanov"},
		"cn": {"Иван Иванов"}, "sn": {"Иванов"}, "displayName": {"Иван Иванов"},
		"mail": {"ivanov@corp.local"}, "userPassword": {"ivanov-pass"}})
	add("uid=petrov,ou=people,"+ldapTestBase, map[string][]string{
		"objectClass": {"inetOrgPerson"}, "uid": {"petrov"},
		"cn": {"Пётр Петров"}, "sn": {"Петров"},
		"mail": {"petrov@corp.local"}, "userPassword": {"petrov-pass"}})
	add("cn=docs-admins,ou=groups,"+ldapTestBase, map[string][]string{
		"objectClass": {"groupOfNames"}, "cn": {"docs-admins"},
		"member": {"uid=ivanov,ou=people," + ldapTestBase}})
}

func TestOpenLDAPIntegration(t *testing.T) {
	url := os.Getenv("TEAMDOCS_TEST_LDAP_URL")
	if url == "" {
		t.Skip("TEAMDOCS_TEST_LDAP_URL не задан — пропускаю LDAP-тест")
	}
	seedLDAP(t, url)

	l, err := NewLDAP(config.LDAPSettings{
		URL: url, Preset: "openldap", BaseDN: ldapTestBase,
		BindLogin: ldapTestAdmin, BindPassword: ldapTestPass,
		EmailAttr:   "mail",
		AdminGroups: []string{"docs-admins"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Успешный вход: профиль + группы через fallback-поиск (overlay memberOf
	// в тестовом контейнере может отсутствовать).
	u, err := l.Authenticate("ivanov", "ivanov-pass")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if u.Subject != "ldap:ivanov" || u.Email != "ivanov@corp.local" || u.Name != "Иван Иванов" {
		t.Fatalf("профиль: %+v", u)
	}
	hasGroup := false
	for _, g := range u.Groups {
		if g == "docs-admins" {
			hasGroup = true
		}
	}
	if !hasGroup {
		t.Fatalf("группа docs-admins не найдена: %v", u.Groups)
	}
	if !l.IsAdminGroupMember(u.Groups) {
		t.Fatal("ivanov должен матчиться в adminGroups")
	}

	// petrov — без групп, не админ.
	p, err := l.Authenticate("petrov", "petrov-pass")
	if err != nil {
		t.Fatalf("authenticate petrov: %v", err)
	}
	if l.IsAdminGroupMember(p.Groups) {
		t.Fatalf("petrov не должен быть админом: %v", p.Groups)
	}

	// Неверный пароль / несуществующий пользователь → ErrLDAPAuth.
	if _, err := l.Authenticate("ivanov", "wrong"); err != ErrLDAPAuth {
		t.Fatalf("неверный пароль: %v", err)
	}
	if _, err := l.Authenticate("nosuch", "x"); err != ErrLDAPAuth {
		t.Fatalf("несуществующий: %v", err)
	}
	// Пустой пароль (анонимный bind) — отклоняется.
	if _, err := l.Authenticate("ivanov", ""); err != ErrLDAPAuth {
		t.Fatalf("пустой пароль: %v", err)
	}
}
