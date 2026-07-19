package config

import (
	"os"

	"github.com/dvislobokov/sconf"
)

// Settings описывает конфигурацию приложения.
// Значения берутся из appsettings.yaml (опционально) и переменных окружения
// с префиксом TEAMDOCS_ (например TEAMDOCS_HTTP__PORT=9000).
type Settings struct {
	HTTP struct {
		Host string `yaml:"host" default:"0.0.0.0"`
		Port int    `yaml:"port" default:"8080"`
	} `yaml:"http"`

	DB struct {
		DSN string `yaml:"dsn" default:"postgres://postgres:postgres@localhost:5432/teamdocs?sslmode=disable"`
	} `yaml:"db"`

	// MaxUploadBytes ограничивает размер загружаемого файла. Файлы хранятся в БД.
	MaxUploadBytes int64 `yaml:"maxUploadBytes" default:"20971520"` // 20 MiB

	Auth     AuthSettings     `yaml:"auth"`
	Branding BrandingSettings `yaml:"branding"`
}

// BrandingSettings — брендинг и цветовая палитра, которые бэкенд отдаёт фронту
// (GET /api/branding). Позволяет менять название/логотип/цвета без пересборки.
type BrandingSettings struct {
	AppName       string `yaml:"appName" default:"team-docs"`
	WorkspaceName string `yaml:"workspaceName" default:"рабочее пространство"`
	Monogram      string `yaml:"monogram" default:"td"`
	// Theme — цветовая схема по умолчанию (id пресета): default, dracula, nord,
	// tokyo, gruvbox, solarized, catppuccin. Пользователь может переопределить в
	// UI (выбор сохраняется в localStorage). Пустое → "default".
	Theme string `yaml:"theme" default:""`
}

// DefaultThemeID — нормализованный id темы по умолчанию из конфига.
func (b BrandingSettings) DefaultThemeID() string { return resolveThemeID(b.Theme) }

// Palette — токены цветов для светлой и тёмной темы (компоненты «R G B»).
type Palette struct {
	Light PaletteColors `yaml:"light"`
	Dark  PaletteColors `yaml:"dark"`
}

// PaletteColors — значения токенов вида "251 250 248" (сырые RGB-компоненты,
// как в CSS-переменных --c-*). Конкретные наборы заданы пресетами в themes.go.
type PaletteColors struct {
	Paper      string `yaml:"paper"`
	Card       string `yaml:"card"`
	Ink        string `yaml:"ink"`
	Body       string `yaml:"body"`
	Muted      string `yaml:"muted"`
	Faint      string `yaml:"faint"`
	Line       string `yaml:"line"`
	Accent     string `yaml:"accent"`
	AccentSoft string `yaml:"accentSoft"`
	Marker     string `yaml:"marker"`
}

// AuthSettings — валидация JWT, выданного IAM-прокси (напр. oauth2-proxy перед
// Keycloak). Приложение стоит за прокси; сам логин делает прокси, а сюда
// приходит запрос с JWT в заголовке — мы его проверяем и достаём identity.
type AuthSettings struct {
	// Enabled=false — открытый режим для локальной разработки (identity = DevUser).
	Enabled bool `yaml:"enabled" default:"false"`
	// Header — из какого заголовка брать токен (снимаем префикс "Bearer ").
	Header string `yaml:"header" default:"Authorization"`
	// JWKSURL — endpoint JWKS IdP для проверки RS256-подписи (Keycloak certs).
	JWKSURL string `yaml:"jwksUrl" default:""`
	// HMACSecret — альтернатива JWKS: общий секрет для HS256-токенов прокси.
	HMACSecret string `yaml:"hmacSecret" default:""`
	// Issuer/Audience — необязательная доп. проверка claims iss/aud.
	Issuer   string `yaml:"issuer" default:""`
	Audience string `yaml:"audience" default:""`
	// Из каких claim брать поля пользователя.
	UsernameClaim string `yaml:"usernameClaim" default:"preferred_username"`
	NameClaim     string `yaml:"nameClaim" default:"name"`
	EmailClaim    string `yaml:"emailClaim" default:"email"`
	// Имя пользователя в открытом (dev) режиме.
	DevUser string `yaml:"devUser" default:"Разработчик"`

	// EditorGroups — группы/роли (из claim groups либо realm_access.roles),
	// которым разрешено редактирование (запись). Пусто → редактировать может
	// любой аутентифицированный пользователь.
	EditorGroups []string `yaml:"editorGroups"`
	// PublicRead — при true неаутентифицированные пользователи могут читать (GET),
	// но не изменять данные. false → любое обращение к API требует токен.
	PublicRead bool `yaml:"publicRead" default:"true"`

	// --- Встроенный OAuth (ROADMAP §8; решение: без обязательного IAM) ---
	// Работает параллельно с проверкой JWT из заголовка: middleware сначала
	// пробует cookie-сессию, затем заголовок.

	// PublicURL — внешний адрес приложения для redirect_uri
	// (например https://docs.example.com). Обязателен для OAuth-провайдеров.
	PublicURL string `yaml:"publicUrl" default:""`
	// SessionSecret подписывает cookie-сессии (HS256). Пусто → случайный на
	// старте: сессии слетают при рестарте — в проде задать явно.
	SessionSecret string `yaml:"sessionSecret" default:""`
	// SessionTTLHours — время жизни сессии (по умолчанию 30 дней).
	SessionTTLHours int `yaml:"sessionTtlHours" default:"720"`
	// DefaultRole — роль новых пользователей: reader | editor.
	DefaultRole string `yaml:"defaultRole" default:"editor"`
	// AdminEmails — бутстрап администраторов: пользователю с таким email
	// (или subject) роль admin проставляется при входе.
	AdminEmails []string `yaml:"adminEmails"`

	Providers ProvidersSettings `yaml:"providers"`
}

// ProvidersSettings — клиенты OAuth-провайдеров. Провайдер считается
// настроенным, если задан clientId.
type ProvidersSettings struct {
	Google OAuthClientSettings `yaml:"google"`
	Yandex OAuthClientSettings `yaml:"yandex"`
	VK     OAuthClientSettings `yaml:"vk"`
	Apple  AppleClientSettings `yaml:"apple"`
}

type OAuthClientSettings struct {
	ClientID     string `yaml:"clientId" default:""`
	ClientSecret string `yaml:"clientSecret" default:""`
}

// AppleClientSettings — Sign in with Apple: client_secret не статический,
// а ES256-JWT, подписываемый ключом .p8 (генерируется на лету).
type AppleClientSettings struct {
	ClientID string `yaml:"clientId" default:""` // Services ID
	TeamID   string `yaml:"teamId" default:""`
	KeyID    string `yaml:"keyId" default:""`
	// PrivateKey — содержимое .p8 (PEM, PKCS#8), можно через env с \n.
	PrivateKey string `yaml:"privateKey" default:""`
}

// Load читает конфигурацию из файла (опционально) и переменных окружения.
func Load() (*Settings, error) {
	s, err := sconf.Load[Settings](
		sconf.New().
			AddYAMLFile("appsettings.yaml", sconf.Optional()).
			AddEnvironmentVariables("TEAMDOCS_"),
		os.Args[1:],
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}
