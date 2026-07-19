package blocknote

import (
	"encoding/json"
	"strings"
	"testing"
)

func parse(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatalf("документ не парсится как массив блоков: %v", err)
	}
	return blocks
}

func TestFromMarkdownEmpty(t *testing.T) {
	raw, text, err := FromMarkdown("")
	if err != nil {
		t.Fatal(err)
	}
	blocks := parse(t, raw)
	if len(blocks) != 1 || blocks[0]["type"] != "paragraph" {
		t.Fatalf("пустой markdown должен давать один пустой абзац, получено: %s", raw)
	}
	if text != "" {
		t.Fatalf("ожидался пустой текст, получено %q", text)
	}
}

func TestFromMarkdownHeadingsAndText(t *testing.T) {
	md := "# Заголовок\n\nПервый абзац со **жирным** и *курсивом*.\n\n##### Глубокий заголовок\n"
	raw, text, err := FromMarkdown(md)
	if err != nil {
		t.Fatal(err)
	}
	blocks := parse(t, raw)
	if blocks[0]["type"] != "heading" {
		t.Fatalf("первый блок должен быть heading, получено %v", blocks[0]["type"])
	}
	// Уровень заголовка клампится к 1..3 (BlockNote не знает h4+).
	last := blocks[len(blocks)-1]
	props, _ := last["props"].(map[string]any)
	if level, _ := props["level"].(float64); level != 3 {
		t.Fatalf("h5 должен клампиться к level 3, получено %v", props["level"])
	}
	for _, part := range []string{"Заголовок", "Первый абзац", "жирным", "курсивом"} {
		if !strings.Contains(text, part) {
			t.Fatalf("плоский текст должен содержать %q, получено %q", part, text)
		}
	}
}

func TestFromMarkdownCodeBlock(t *testing.T) {
	raw, _, err := FromMarkdown("```go\nfmt.Println(1)\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	blocks := parse(t, raw)
	if blocks[0]["type"] != "codeBlock" {
		t.Fatalf("ожидался codeBlock, получено %v", blocks[0]["type"])
	}
	props, _ := blocks[0]["props"].(map[string]any)
	if props["language"] != "go" {
		t.Fatalf("язык блока кода должен быть go, получено %v", props["language"])
	}
}

func TestRoundTripPreservesContent(t *testing.T) {
	md := strings.Join([]string{
		"# Настройки",
		"",
		"Абзац со **жирным**, *курсивом* и `кодом`.",
		"",
		"- пункт один",
		"- пункт два",
		"",
		"1. первый",
		"2. второй",
		"",
		"> цитата",
		"",
		"```sql",
		"SELECT 1;",
		"```",
	}, "\n")

	raw, _, err := FromMarkdown(md)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ToMarkdown(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Round-trip не обязан быть байт-в-байт, но содержимое обязано выжить.
	for _, part := range []string{
		"Настройки", "жирным", "курсивом", "кодом",
		"пункт один", "пункт два", "первый", "второй",
		"цитата", "SELECT 1;",
	} {
		if !strings.Contains(back, part) {
			t.Fatalf("round-trip потерял %q; результат:\n%s", part, back)
		}
	}
}
