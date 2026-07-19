package pages

import "testing"

func TestExtractText(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"пустой вход", "", ""},
		{"битый JSON", "{не json", ""},
		{"плоский абзац", `[{"type":"paragraph","content":[{"type":"text","text":"привет мир"}]}]`, "привет мир"},
		{
			"вложенные children",
			`[{"type":"bulletListItem","content":[{"type":"text","text":"родитель"}],
			   "children":[{"type":"bulletListItem","content":[{"type":"text","text":"ребёнок"}]}]}]`,
			"родитель ребёнок",
		},
		{
			"таблица (content-объект с rows)",
			`[{"type":"table","content":{"type":"tableContent","rows":[
			   {"cells":[[{"type":"text","text":"ячейка"}]]}]}}]`,
			"", // rows не обходится — известное ограничение walk (content/children)
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractText([]byte(tc.doc)); got != tc.want {
				t.Fatalf("ExtractText = %q, ожидалось %q", got, tc.want)
			}
		})
	}
}
