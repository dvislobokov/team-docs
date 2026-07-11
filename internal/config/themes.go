package config

import "strings"

// Встроенные пресеты палитры. Токены — сырые «R G B», адаптированы из
// официальных палитр под нашу 10-токенную структуру (светлая + тёмная).

// ThemePreset — цветовая схема с идентификатором и подписью для UI.
type ThemePreset struct {
	ID      string
	Label   string
	Palette Palette
}

var defaultTheme = Palette{
	Light: PaletteColors{
		Paper: "251 250 248", Card: "255 255 255", Ink: "38 37 31", Body: "64 60 51",
		Muted: "139 133 122", Faint: "168 162 150", Line: "235 232 224",
		Accent: "53 104 89", AccentSoft: "231 239 234", Marker: "247 233 160",
	},
	Dark: PaletteColors{
		Paper: "26 25 23", Card: "33 31 28", Ink: "240 237 230", Body: "199 193 180",
		Muted: "146 139 125", Faint: "110 103 90", Line: "50 46 41",
		Accent: "78 158 120", AccentSoft: "33 46 40", Marker: "122 106 45",
	},
}

var draculaTheme = Palette{
	Light: PaletteColors{
		Paper: "248 248 250", Card: "255 255 255", Ink: "40 42 54", Body: "68 71 90",
		Muted: "108 116 150", Faint: "160 166 190", Line: "228 228 236",
		Accent: "124 88 199", AccentSoft: "238 232 250", Marker: "241 236 150",
	},
	Dark: PaletteColors{
		Paper: "40 42 54", Card: "52 55 70", Ink: "248 248 242", Body: "220 222 236",
		Muted: "98 114 164", Faint: "78 88 128", Line: "68 71 90",
		Accent: "189 147 249", AccentSoft: "58 50 84", Marker: "108 104 66",
	},
}

var nordTheme = Palette{
	Light: PaletteColors{
		Paper: "236 239 244", Card: "255 255 255", Ink: "46 52 64", Body: "76 86 106",
		Muted: "120 132 156", Faint: "168 178 196", Line: "216 222 233",
		Accent: "94 129 172", AccentSoft: "224 232 240", Marker: "235 203 139",
	},
	Dark: PaletteColors{
		Paper: "46 52 64", Card: "59 66 82", Ink: "236 239 244", Body: "216 222 233",
		Muted: "143 153 176", Faint: "106 116 140", Line: "67 76 94",
		Accent: "136 192 208", AccentSoft: "44 62 72", Marker: "108 104 70",
	},
}

var tokyoTheme = Palette{
	Light: PaletteColors{
		Paper: "225 226 231", Card: "255 255 255", Ink: "52 59 88", Body: "84 92 130",
		Muted: "130 138 170", Faint: "172 178 198", Line: "210 213 224",
		Accent: "52 96 191", AccentSoft: "223 231 246", Marker: "224 190 120",
	},
	Dark: PaletteColors{
		Paper: "26 27 38", Card: "41 46 66", Ink: "192 202 245", Body: "169 177 214",
		Muted: "122 132 175", Faint: "86 95 137", Line: "54 60 84",
		Accent: "122 162 247", AccentSoft: "36 46 78", Marker: "110 104 70",
	},
}

// gruvboxTheme — https://github.com/morhetz/gruvbox
var gruvboxTheme = Palette{
	Light: PaletteColors{
		Paper: "251 241 199", Card: "255 255 255", Ink: "60 56 54", Body: "80 73 69",
		Muted: "124 111 100", Faint: "168 153 132", Line: "235 219 178",
		Accent: "214 93 14", AccentSoft: "242 229 188", Marker: "247 218 120",
	},
	Dark: PaletteColors{
		Paper: "40 40 40", Card: "60 56 54", Ink: "235 219 178", Body: "213 196 161",
		Muted: "146 131 116", Faint: "120 108 96", Line: "80 73 69",
		Accent: "254 128 25", AccentSoft: "70 52 34", Marker: "110 96 50",
	},
}

// solarizedTheme — https://ethanschoonover.com/solarized
var solarizedTheme = Palette{
	Light: PaletteColors{
		Paper: "253 246 227", Card: "255 253 246", Ink: "88 110 117", Body: "101 123 131",
		Muted: "131 148 150", Faint: "147 161 161", Line: "238 232 213",
		Accent: "38 139 210", AccentSoft: "221 234 244", Marker: "240 224 150",
	},
	Dark: PaletteColors{
		Paper: "0 43 54", Card: "7 54 66", Ink: "147 161 161", Body: "131 148 150",
		Muted: "101 123 131", Faint: "88 110 117", Line: "18 66 78",
		Accent: "38 139 210", AccentSoft: "10 46 62", Marker: "90 84 40",
	},
}

// catppuccinTheme — https://catppuccin.com (Latte + Mocha, акцент Mauve)
var catppuccinTheme = Palette{
	Light: PaletteColors{
		Paper: "239 241 245", Card: "255 255 255", Ink: "76 79 105", Body: "92 95 119",
		Muted: "140 143 161", Faint: "156 160 176", Line: "188 192 204",
		Accent: "136 57 239", AccentSoft: "235 226 250", Marker: "245 226 175",
	},
	Dark: PaletteColors{
		Paper: "30 30 46", Card: "49 50 68", Ink: "205 214 244", Body: "186 194 222",
		Muted: "127 132 156", Faint: "108 112 134", Line: "69 71 90",
		Accent: "203 166 247", AccentSoft: "58 48 78", Marker: "110 100 66",
	},
}

// themeList — упорядоченный набор для UI и /api/branding.
var themeList = []ThemePreset{
	{ID: "default", Label: "Бумага", Palette: defaultTheme},
	{ID: "dracula", Label: "Dracula", Palette: draculaTheme},
	{ID: "nord", Label: "Nord", Palette: nordTheme},
	{ID: "tokyo", Label: "Tokyo Night", Palette: tokyoTheme},
	{ID: "gruvbox", Label: "Gruvbox", Palette: gruvboxTheme},
	{ID: "solarized", Label: "Solarized", Palette: solarizedTheme},
	{ID: "catppuccin", Label: "Catppuccin", Palette: catppuccinTheme},
}

// Псевдонимы имён тем → канонический id.
var themeAliases = map[string]string{
	"":            "default",
	"paper":       "default",
	"nordic":      "nord",
	"tokyonight":  "tokyo",
	"tokyo-night": "tokyo",
}

// Themes возвращает все пресеты (для API).
func Themes() []ThemePreset { return themeList }

// resolveThemeID нормализует имя темы из конфига в канонический id.
func resolveThemeID(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if id, ok := themeAliases[key]; ok {
		return id
	}
	for _, t := range themeList {
		if t.ID == key {
			return key
		}
	}
	return "default"
}
