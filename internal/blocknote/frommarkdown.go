// Package blocknote конвертирует Markdown в формат документа BlockNote
// (массив JSON-блоков), чтобы MCP-инструменты могли принимать доки в Markdown
// и сохранять их как обычные страницы team-docs.
package blocknote

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type block = map[string]any
type inline = map[string]any

// FromMarkdown парсит Markdown и возвращает документ BlockNote (JSON-массив
// блоков) и плоский текст (для content_text/поиска).
func FromMarkdown(md string) (json.RawMessage, string, error) {
	source := []byte(md)
	root := goldmark.New().Parser().Parse(text.NewReader(source))

	var tb bytes.Buffer
	var blocks []block
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		blocks = append(blocks, blocksFrom(n, source, &tb)...)
	}
	if len(blocks) == 0 {
		// BlockNote не любит пустой документ — стартуем с чистого абзаца.
		blocks = []block{{"type": "paragraph", "content": []inline{}}}
	}

	raw, err := json.Marshal(blocks)
	if err != nil {
		return nil, "", err
	}
	return raw, strings.TrimSpace(tb.String()), nil
}

// blocksFrom превращает один блочный AST-узел в один или несколько BlockNote-блоков.
func blocksFrom(n ast.Node, src []byte, tb *bytes.Buffer) []block {
	switch v := n.(type) {
	case *ast.Heading:
		level := v.Level
		if level > 3 {
			level = 3
		}
		return []block{{
			"type":    "heading",
			"props":   map[string]any{"level": level},
			"content": inlineFrom(n, src, tb),
		}}

	case *ast.Paragraph, *ast.TextBlock:
		return []block{{"type": "paragraph", "content": inlineFrom(n, src, tb)}}

	case *ast.FencedCodeBlock:
		code := linesText(v, src)
		tb.WriteString(code + " ")
		return []block{{
			"type":    "codeBlock",
			"props":   map[string]any{"language": string(v.Language(src))},
			"content": []inline{textItem(code, nil)},
		}}

	case *ast.CodeBlock:
		code := linesText(v, src)
		tb.WriteString(code + " ")
		return []block{{
			"type":    "codeBlock",
			"props":   map[string]any{"language": "text"},
			"content": []inline{textItem(code, nil)},
		}}

	case *ast.Blockquote:
		var content []inline
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			content = append(content, inlineFrom(c, src, tb)...)
		}
		return []block{{"type": "quote", "content": ensureInline(content)}}

	case *ast.List:
		return listBlocks(v, src, tb)

	case *ast.ThematicBreak:
		return nil // у BlockNote нет дефолтного блока-разделителя

	default:
		// Неизвестный контейнер — пытаемся вытащить абзац из содержимого.
		if n.HasChildren() {
			content := inlineFrom(n, src, tb)
			if len(content) > 0 {
				return []block{{"type": "paragraph", "content": content}}
			}
		}
		return nil
	}
}

func listBlocks(list *ast.List, src []byte, tb *bytes.Buffer) []block {
	btype := "bulletListItem"
	if list.IsOrdered() {
		btype = "numberedListItem"
	}
	var items []block
	for li := list.FirstChild(); li != nil; li = li.NextSibling() {
		var content []inline
		var children []block
		for cc := li.FirstChild(); cc != nil; cc = cc.NextSibling() {
			if nested, ok := cc.(*ast.List); ok {
				children = append(children, listBlocks(nested, src, tb)...)
			} else {
				content = append(content, inlineFrom(cc, src, tb)...)
			}
		}
		b := block{"type": btype, "content": ensureInline(content)}
		if len(children) > 0 {
			b["children"] = children
		}
		items = append(items, b)
	}
	return items
}

// inlineFrom собирает inline-контент BlockNote из дочерних inline-узлов.
func inlineFrom(n ast.Node, src []byte, tb *bytes.Buffer) []inline {
	out := []inline{}
	appendInline(n, src, map[string]bool{}, &out, tb)
	return out
}

func appendInline(n ast.Node, src []byte, styles map[string]bool, out *[]inline, tb *bytes.Buffer) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			s := string(v.Segment.Value(src))
			if s != "" {
				*out = append(*out, textItem(s, styles))
				tb.WriteString(s + " ")
			}
			if v.HardLineBreak() {
				*out = append(*out, textItem("\n", styles))
			} else if v.SoftLineBreak() {
				*out = append(*out, textItem(" ", styles))
			}
		case *ast.String:
			s := string(v.Value)
			*out = append(*out, textItem(s, styles))
			tb.WriteString(s + " ")
		case *ast.CodeSpan:
			s := codeSpanText(v, src)
			*out = append(*out, textItem(s, withStyle(styles, "code")))
			tb.WriteString(s + " ")
		case *ast.Emphasis:
			st := "italic"
			if v.Level == 2 {
				st = "bold"
			}
			appendInline(v, src, withStyle(styles, st), out, tb)
		case *ast.Link:
			*out = append(*out, inline{
				"type":    "link",
				"href":    string(v.Destination),
				"content": inlineFrom(v, src, tb),
			})
		case *ast.AutoLink:
			url := string(v.URL(src))
			*out = append(*out, inline{
				"type":    "link",
				"href":    url,
				"content": []inline{textItem(url, nil)},
			})
			tb.WriteString(url + " ")
		default:
			if c.HasChildren() {
				appendInline(c, src, styles, out, tb)
			}
		}
	}
}

func textItem(txt string, styles map[string]bool) inline {
	st := map[string]any{}
	for k, v := range styles {
		if v {
			st[k] = true
		}
	}
	return inline{"type": "text", "text": txt, "styles": st}
}

func withStyle(styles map[string]bool, key string) map[string]bool {
	next := make(map[string]bool, len(styles)+1)
	for k, v := range styles {
		next[k] = v
	}
	next[key] = true
	return next
}

func codeSpanText(n *ast.CodeSpan, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(src))
		}
	}
	return b.String()
}

func linesText(n ast.Node, src []byte) string {
	var b bytes.Buffer
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return strings.TrimRight(b.String(), "\n")
}

func ensureInline(c []inline) []inline {
	if c == nil {
		return []inline{}
	}
	return c
}
