package pages

import (
	"encoding/json"
	"strings"
)

// extractText рекурсивно вытаскивает весь текст из документа BlockNote (JSON),
// чтобы сохранить его в content_text для полнотекстового поиска.
//
// Документ BlockNote — массив блоков. У блока есть:
//   - "content": массив inline-элементов, у текстовых есть поле "text";
//   - "children": массив вложенных блоков.
//
// Функция толерантна к структуре: собирает все строковые поля "text",
// обходя вложенные "content" и "children". Экспортирована — переиспользуется
// MCP-сервером для чтения текста существующих страниц.
func ExtractText(doc []byte) string {
	if len(doc) == 0 {
		return ""
	}
	var root any
	if err := json.Unmarshal(doc, &root); err != nil {
		return ""
	}
	var b strings.Builder
	walk(root, &b)
	return strings.TrimSpace(b.String())
}

func walk(node any, b *strings.Builder) {
	switch v := node.(type) {
	case []any:
		for _, item := range v {
			walk(item, b)
		}
	case map[string]any:
		if txt, ok := v["text"].(string); ok && txt != "" {
			b.WriteString(txt)
			b.WriteByte(' ')
		}
		if content, ok := v["content"]; ok {
			walk(content, b)
		}
		if children, ok := v["children"]; ok {
			walk(children, b)
		}
	}
}
