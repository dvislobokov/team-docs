package auth

// LDAP-авторизация (ROADMAP §8): FreeIPA / OpenLDAP / Active Directory.
// Search-then-bind: сервисная учётка ищет пользователя по фильтру, затем
// bind найденным DN с паролем пользователя. Без сервисной учётки —
// direct-bind по userLoginTemplate. Пресеты задают фильтры/атрибуты.

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"team-docs/internal/config"
)

// ErrLDAPAuth — неверный логин/пароль (401 у вызывающего).
var ErrLDAPAuth = errors.New("неверный логин или пароль")

// adChainMatch — AD-правило вложенных групп (LDAP_MATCHING_RULE_IN_CHAIN).
const adChainMatch = "1.2.840.113556.1.4.1941"

type ldapPreset struct {
	userFilter  string // %s — экранированный логин
	loginAttr   string
	nameAttrs   []string
	groupFilter string // fallback-поиск групп: %s — DN, %u — логин
	nestedAD    bool   // вложенные группы chain-matching'ом
}

var ldapPresets = map[string]ldapPreset{
	"ad": {
		userFilter: "(&(objectClass=user)(sAMAccountName=%s))",
		loginAttr:  "sAMAccountName",
		nameAttrs:  []string{"displayName", "cn"},
		nestedAD:   true,
	},
	"freeipa": {
		userFilter: "(&(objectClass=person)(uid=%s))",
		loginAttr:  "uid",
		nameAttrs:  []string{"displayName", "cn"},
	},
	"openldap": {
		userFilter: "(&(objectClass=inetOrgPerson)(uid=%s))",
		loginAttr:  "uid",
		nameAttrs:  []string{"displayName", "cn"},
		// memberOf-overlay может отсутствовать — ищем группы по членству.
		groupFilter: "(|(&(objectClass=groupOfNames)(member=%s))(&(objectClass=posixGroup)(memberUid=%u)))",
	},
}

// LDAPAuthenticator проверяет логин/пароль в каталоге.
type LDAPAuthenticator struct {
	cfg    config.LDAPSettings
	preset ldapPreset
}

// NewLDAP возвращает nil, если LDAP не настроен (url пуст).
func NewLDAP(cfg config.LDAPSettings) (*LDAPAuthenticator, error) {
	if cfg.URL == "" {
		return nil, nil
	}
	p, ok := ldapPresets[cfg.Preset]
	if !ok {
		return nil, fmt.Errorf("ldap: неизвестный preset %q (ad|freeipa|openldap)", cfg.Preset)
	}
	if cfg.BaseDN == "" {
		return nil, errors.New("ldap: baseDn обязателен")
	}
	return &LDAPAuthenticator{cfg: cfg, preset: p}, nil
}

// --- резолв имён ---

// domainFromBaseDN: dc=corp,dc=local → corp.local.
func domainFromBaseDN(baseDN string) string {
	var parts []string
	for _, rdn := range strings.Split(baseDN, ",") {
		rdn = strings.TrimSpace(rdn)
		if v, ok := strings.CutPrefix(strings.ToLower(rdn), "dc="); ok {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ".")
}

// resolveBindName разворачивает bindLogin сервисной учётки:
// `=` → DN как есть; bindLoginTemplate → подстановка; иначе правило пресета.
func (l *LDAPAuthenticator) resolveBindName() string {
	login := l.cfg.BindLogin
	if login == "" || strings.Contains(login, "=") {
		return login
	}
	if l.cfg.BindLoginTemplate != "" {
		return strings.ReplaceAll(l.cfg.BindLoginTemplate, "%s", login)
	}
	if strings.Contains(login, "@") || strings.Contains(login, `\`) {
		return login
	}
	switch l.cfg.Preset {
	case "ad":
		return login + "@" + domainFromBaseDN(l.cfg.BaseDN)
	case "freeipa":
		return fmt.Sprintf("uid=%s,cn=users,cn=accounts,%s", login, l.cfg.BaseDN)
	default: // openldap
		tpl := l.cfg.ServiceDNTemplate
		if tpl == "" {
			tpl = "uid=%s,ou=people," + l.cfg.BaseDN
		}
		return strings.ReplaceAll(tpl, "%s", login)
	}
}

// resolveUserBindName — имя для direct-bind обычного пользователя.
func (l *LDAPAuthenticator) resolveUserBindName(login string) string {
	if l.cfg.UserLoginTemplate != "" {
		return strings.ReplaceAll(l.cfg.UserLoginTemplate, "%s", login)
	}
	if l.cfg.Preset == "ad" {
		return login + "@" + domainFromBaseDN(l.cfg.BaseDN)
	}
	return "" // без шаблона direct-bind невозможен
}

func (l *LDAPAuthenticator) userFilter(login string) string {
	f := l.cfg.UserFilter
	if f == "" {
		f = l.preset.userFilter
	}
	return strings.ReplaceAll(f, "%s", ldap.EscapeFilter(login))
}

func (l *LDAPAuthenticator) loginAttr() string {
	if l.cfg.LoginAttr != "" {
		return l.cfg.LoginAttr
	}
	return l.preset.loginAttr
}

// --- соединение ---

func (l *LDAPAuthenticator) dial() (*ldap.Conn, error) {
	timeout := time.Duration(l.cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: l.cfg.InsecureSkipVerify} //nolint:gosec // осознанная dev-опция
	if l.cfg.CACert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(l.cfg.CACert, `\n`, "\n"))) {
			return nil, errors.New("ldap: caCert не похож на PEM")
		}
		tlsCfg.RootCAs = pool
	}
	conn, err := ldap.DialURL(l.cfg.URL,
		ldap.DialWithTLSConfig(tlsCfg),
		ldap.DialWithDialer(&net.Dialer{Timeout: timeout}))
	if err != nil {
		return nil, fmt.Errorf("ldap: подключение: %w", err)
	}
	conn.SetTimeout(timeout)
	if l.cfg.StartTLS {
		if err := conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, fmt.Errorf("ldap: StartTLS: %w", err)
		}
	}
	return conn, nil
}

func (l *LDAPAuthenticator) userBases() []string {
	if len(l.cfg.UserBases) > 0 {
		return l.cfg.UserBases
	}
	return []string{l.cfg.BaseDN}
}

// Authenticate проверяет логин/пароль и возвращает identity с группами.
func (l *LDAPAuthenticator) Authenticate(login, password string) (*User, error) {
	if login == "" || password == "" {
		return nil, ErrLDAPAuth // пустой пароль в LDAP — анонимный bind, не пропускаем
	}
	conn, err := l.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	attrs := []string{l.loginAttr(), "displayName", "cn", l.cfg.EmailAttr, "memberOf"}

	var entry *ldap.Entry
	if bindName := l.resolveBindName(); bindName != "" {
		// search-then-bind.
		if err := conn.Bind(bindName, l.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap: bind сервисной учётки: %w", err)
		}
		entry, err = l.searchUser(conn, login, attrs)
		if err != nil {
			return nil, err
		}
		if err := conn.Bind(entry.DN, password); err != nil {
			return nil, ErrLDAPAuth
		}
		// Возвращаемся под сервисную учётку — для поиска групп.
		if err := conn.Bind(bindName, l.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap: повторный bind: %w", err)
		}
	} else {
		// direct-bind.
		userBind := l.resolveUserBindName(login)
		if userBind == "" {
			return nil, errors.New("ldap: не задана сервисная учётка (bindLogin) и userLoginTemplate")
		}
		if err := conn.Bind(userBind, password); err != nil {
			return nil, ErrLDAPAuth
		}
		if entry, err = l.searchUser(conn, login, attrs); err != nil {
			return nil, err
		}
	}

	groups, err := l.groupsOf(conn, entry, login)
	if err != nil {
		return nil, err
	}

	name := ""
	for _, a := range l.nameAttrs() {
		if v := entry.GetAttributeValue(a); v != "" {
			name = v
			break
		}
	}
	username := entry.GetAttributeValue(l.loginAttr())
	if username == "" {
		username = login
	}
	if name == "" {
		name = username
	}
	return &User{
		Subject:  "ldap:" + strings.ToLower(username),
		Username: username,
		Name:     name,
		Email:    entry.GetAttributeValue(l.cfg.EmailAttr),
		Groups:   groups,
	}, nil
}

func (l *LDAPAuthenticator) nameAttrs() []string {
	if l.cfg.NameAttr != "" {
		return []string{l.cfg.NameAttr}
	}
	return l.preset.nameAttrs
}

func (l *LDAPAuthenticator) searchUser(conn *ldap.Conn, login string, attrs []string) (*ldap.Entry, error) {
	filter := l.userFilter(login)
	for _, base := range l.userBases() {
		res, err := conn.Search(ldap.NewSearchRequest(
			base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 0, false,
			filter, attrs, nil))
		if err != nil {
			// База может отсутствовать в одном из userBases — пробуем следующую.
			continue
		}
		if len(res.Entries) == 1 {
			return res.Entries[0], nil
		}
		if len(res.Entries) > 1 {
			return nil, fmt.Errorf("ldap: фильтр %s нашёл несколько записей", filter)
		}
	}
	return nil, ErrLDAPAuth // не палим, существует ли пользователь
}

// groupsOf собирает группы: memberOf + AD chain-matching (вложенные) либо
// fallback-поиск по членству (OpenLDAP без overlay). Возвращает и DN, и CN.
func (l *LDAPAuthenticator) groupsOf(conn *ldap.Conn, entry *ldap.Entry, login string) ([]string, error) {
	set := map[string]bool{}
	add := func(dn string) {
		if dn == "" {
			return
		}
		set[dn] = true
		if cn := firstRDNValue(dn); cn != "" {
			set[cn] = true
		}
	}
	for _, dn := range entry.GetAttributeValues("memberOf") {
		add(dn)
	}

	groupBase := l.cfg.GroupBase
	if groupBase == "" {
		groupBase = l.cfg.BaseDN
	}
	if l.preset.nestedAD {
		// Вложенные группы AD одним запросом.
		res, err := conn.Search(ldap.NewSearchRequest(
			groupBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			fmt.Sprintf("(member:%s:=%s)", adChainMatch, ldap.EscapeFilter(entry.DN)),
			[]string{"dn"}, nil))
		if err == nil {
			for _, e := range res.Entries {
				add(e.DN)
			}
		}
	} else if len(set) == 0 {
		// Fallback: memberOf пуст (нет overlay) — ищем группы по членству.
		filter := l.cfg.GroupFilter
		if filter == "" {
			filter = l.preset.groupFilter
		}
		if filter != "" {
			filter = strings.ReplaceAll(filter, "%s", ldap.EscapeFilter(entry.DN))
			filter = strings.ReplaceAll(filter, "%u", ldap.EscapeFilter(login))
			res, err := conn.Search(ldap.NewSearchRequest(
				groupBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
				filter, []string{"dn"}, nil))
			if err != nil {
				return nil, fmt.Errorf("ldap: поиск групп: %w", err)
			}
			for _, e := range res.Entries {
				add(e.DN)
			}
		}
	}

	out := make([]string, 0, len(set))
	for g := range set {
		out = append(out, g)
	}
	return out, nil
}

// IsAdminGroupMember — состоит ли пользователь в одной из ldap.adminGroups
// (сравнение по DN или CN, без учёта регистра).
func (l *LDAPAuthenticator) IsAdminGroupMember(groups []string) bool {
	for _, g := range groups {
		for _, a := range l.cfg.AdminGroups {
			if strings.EqualFold(g, a) {
				return true
			}
		}
	}
	return false
}

// Check — диагностика для админки («Проверить авторизацию»): во что
// развернулся bindLogin, доступен ли каталог, проходит ли сервисный bind.
func (l *LDAPAuthenticator) Check() map[string]any {
	out := map[string]any{
		"url":    l.cfg.URL,
		"preset": l.cfg.Preset,
	}
	if bindName := l.resolveBindName(); bindName != "" {
		out["bindName"] = bindName
	} else if tpl := l.resolveUserBindName("<login>"); tpl != "" {
		out["directBind"] = tpl
	} else {
		out["warning"] = "не заданы ни bindLogin, ни userLoginTemplate — вход не заработает"
	}

	conn, err := l.dial()
	if err != nil {
		out["connect"] = "ошибка: " + err.Error()
		return out
	}
	defer conn.Close()
	out["connect"] = "ok"

	if bindName := l.resolveBindName(); bindName != "" {
		if err := conn.Bind(bindName, l.cfg.BindPassword); err != nil {
			out["serviceBind"] = "ошибка: " + err.Error()
		} else {
			out["serviceBind"] = "ok"
		}
	}
	return out
}

// firstRDNValue: cn=docs-admins,ou=groups,… → docs-admins.
func firstRDNValue(dn string) string {
	first := strings.SplitN(dn, ",", 2)[0]
	if i := strings.IndexByte(first, '='); i > 0 {
		return strings.TrimSpace(first[i+1:])
	}
	return ""
}
