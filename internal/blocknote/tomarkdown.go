package blocknote

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToMarkdown конвертирует документ BlockNote (JSON-массив блоков) в Markdown.
// Обратная к FromMarkdown операция; экзотические блоки деградируют в текст.
func ToMarkdown(content []byte) (string, error) {
	if len(content) == 0 {
		return "", nil
	}
	var blocks []map[string]any
	if err := json.Unmarshal(content, &blocks); err != nil {
		return "", err
	}
	var b strings.Builder
	renderBlocks(blocks, &b, "")
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func renderBlocks(blocks []map[string]any, b *strings.Builder, indent string) {
	num := 0
	prevList := false
	for _, blk := range blocks {
		typ, _ := blk["type"].(string)
		isList := typ == "bulletListItem" || typ == "numberedListItem" || typ == "checkListItem"
		if !isList {
			num = 0
			if prevList {
				b.WriteString("\n") // пустая строка после списка
			}
		}
		prevList = isList

		content := inlineToMd(blk["content"])
		switch typ {
		case "heading":
			b.WriteString(strings.Repeat("#", headingLevel(blk)) + " " + content + "\n\n")
		case "paragraph":
			if strings.TrimSpace(content) != "" {
				b.WriteString(content + "\n\n")
			}
		case "quote":
			b.WriteString("> " + content + "\n\n")
		case "callout":
			b.WriteString("> " + calloutEmoji(blk) + content + "\n\n")
		case "codeBlock":
			b.WriteString("```" + propString(blk, "language") + "\n" + plainInline(blk["content"]) + "\n```\n\n")
		case "mermaid":
			b.WriteString("```mermaid\n" + propString(blk, "code") + "\n```\n\n")
		case "openapi":
			src := propString(blk, "source")
			if strings.HasPrefix(strings.TrimSpace(src), "http://") || strings.HasPrefix(strings.TrimSpace(src), "https://") {
				b.WriteString("[OpenAPI](" + strings.TrimSpace(src) + ")\n\n")
			} else {
				b.WriteString("```yaml\n" + src + "\n```\n\n")
			}
		case "bulletListItem":
			b.WriteString(indent + "- " + content + "\n")
			renderChildren(blk, b, indent+"  ")
		case "numberedListItem":
			num++
			b.WriteString(indent + fmt.Sprintf("%d. ", num) + content + "\n")
			renderChildren(blk, b, indent+"   ")
		case "checkListItem":
			mark := "[ ]"
			if props, ok := blk["props"].(map[string]any); ok {
				if c, _ := props["checked"].(bool); c {
					mark = "[x]"
				}
			}
			b.WriteString(indent + "- " + mark + " " + content + "\n")
			renderChildren(blk, b, indent+"  ")
		default:
			if strings.TrimSpace(content) != "" {
				b.WriteString(content + "\n\n")
			}
		}
	}
}

func renderChildren(blk map[string]any, b *strings.Builder, indent string) {
	if children, ok := blk["children"].([]any); ok && len(children) > 0 {
		sub := make([]map[string]any, 0, len(children))
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				sub = append(sub, m)
			}
		}
		renderBlocks(sub, b, indent)
	}
}

func headingLevel(blk map[string]any) int {
	if props, ok := blk["props"].(map[string]any); ok {
		if l, ok := props["level"].(float64); ok && l >= 1 {
			return int(l)
		}
	}
	return 1
}

func propString(blk map[string]any, key string) string {
	if props, ok := blk["props"].(map[string]any); ok {
		if s, ok := props[key].(string); ok {
			return s
		}
	}
	return ""
}

func calloutEmoji(blk map[string]any) string {
	if e := propString(blk, "emoji"); e != "" {
		return e + " "
	}
	return ""
}

// inlineToMd рендерит inline-контент (text/link/mention/status) в Markdown.
func inlineToMd(c any) string {
	arr, ok := c.([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, it := range arr {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		switch m["type"] {
		case "text":
			txt, _ := m["text"].(string)
			styles, _ := m["styles"].(map[string]any)
			b.WriteString(applyStyles(txt, styles))
		case "link":
			href, _ := m["href"].(string)
			b.WriteString("[" + inlineToMd(m["content"]) + "](" + href + ")")
		case "mention":
			if props, ok := m["props"].(map[string]any); ok {
				if label, _ := props["label"].(string); label != "" {
					b.WriteString("@" + label)
				}
			}
		case "status":
			if props, ok := m["props"].(map[string]any); ok {
				if label, _ := props["label"].(string); label != "" {
					b.WriteString("`" + label + "`")
				}
			}
		}
	}
	return b.String()
}

func applyStyles(txt string, styles map[string]any) string {
	if styles == nil || txt == "" {
		return txt
	}
	if b, _ := styles["code"].(bool); b {
		return "`" + txt + "`" // код не форматируем дополнительно
	}
	if b, _ := styles["italic"].(bool); b {
		txt = "_" + txt + "_"
	}
	if b, _ := styles["bold"].(bool); b {
		txt = "**" + txt + "**"
	}
	return txt
}

// plainInline собирает голый текст (для кода — без разметки).
func plainInline(c any) string {
	arr, ok := c.([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, it := range arr {
		if m, ok := it.(map[string]any); ok {
			if txt, ok := m["text"].(string); ok {
				b.WriteString(txt)
			}
		}
	}
	return b.String()
}
