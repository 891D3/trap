// markdown_html.go —— 笔记正文的 markdown → HTML 轻量转换 + 剪贴板写入。
//
// 零第三方依赖：仅覆盖常用 GFM 子集（标题/引用/列表/任务列表/代码块/分割线/
// 段落 + 行内 code/em/strong）。完整渲染由 WebView 端 Milkdown 承担；这里
// 只服务「复制为富文本」剪贴板需求（PRD v2.1 §3.5.2）。
package service

import (
	"html"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// markdownToHTML 把 GFM 文本转成带样式的 HTML 片段（粘贴进富文本编辑器可读）。
// title 作为文档一级标题；light/dark 由粘贴端默认样式决定，这里只承载语义结构。
func markdownToHTML(title, content string) (string, error) {
	var b strings.Builder
	b.WriteString("<section>")
	// 标题
	b.WriteString("<h1>" + html.EscapeString(title) + "</h1>")

	lines := strings.Split(content, "\n")
	i := 0
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "```"):
			// 围栏代码块：吞到结束围栏
			b.WriteString("<pre><code>" + html.EscapeString(strings.Join(peekBlock(lines, &i, "```"), "\n")) + "</code></pre>")
		case strings.HasPrefix(trimmed, "### "):
			b.WriteString("<h3>" + html.EscapeString(strings.TrimPrefix(trimmed, "### ")) + "</h3>")
			i++
		case strings.HasPrefix(trimmed, "## "):
			b.WriteString("<h2>" + html.EscapeString(strings.TrimPrefix(trimmed, "## ")) + "</h2>")
			i++
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString("<h1>" + html.EscapeString(strings.TrimPrefix(trimmed, "# ")) + "</h1>")
			i++
		case strings.HasPrefix(trimmed, "> "):
			// 引用块
			var quote []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
				quote = append(quote, strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")), ""))
				i++
			}
			b.WriteString("<blockquote>" + inline(strings.Join(quote, "\n")) + "</blockquote>")
		case isTaskLine(trimmed):
			// 任务列表（GFM task；需先于普通列表判定，否则被 "- " 吞掉）
			b.WriteString("<ul style=\"list-style:none\">")
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				if !isTaskLine(t) {
					break
				}
				checked := strings.Contains(t[:6], "[x]") || strings.Contains(t[:6], "[X]")
				box := "<input type=\"checkbox\" disabled>"
				if checked {
					box = "<input type=\"checkbox\" checked disabled>"
				}
				b.WriteString("<li>" + box + " " + inline(taskText(t)) + "</li>")
				i++
			}
			b.WriteString("</ul>")
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || isTaskLine(trimmed):
			// 列表段（混合普通与任务行：GFM 中同一列表段常混用）
			b.WriteString("<ul>")
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				if strings.HasPrefix(t, "- [") {
					checked := strings.Contains(t[:6], "[x]") || strings.Contains(t[:6], "[X]")
					box := "<input type=\"checkbox\" disabled>"
					if checked {
						box = "<input type=\"checkbox\" checked disabled>"
					}
					b.WriteString("<li>" + box + " " + inline(taskText(t)) + "</li>")
					i++
					continue
				}
				if !strings.HasPrefix(t, "- ") && !strings.HasPrefix(t, "* ") {
					break
				}
				b.WriteString("<li>" + inline(strings.TrimPrefix(t[1:], " ")) + "</li>")
				i += 1
			}
			b.WriteString("</ul>")
		case trimmed == "---" || trimmed == "***":
			b.WriteString("<hr/>")
			i++
		case trimmed == "":
			i++
		default:
			// 普通段落（连续非空行合并）
			var para []string
			for i < len(lines) && strings.TrimSpace(lines[i]) != "" &&
				!strings.HasPrefix(strings.TrimSpace(lines[i]), "-") &&
				!strings.HasPrefix(strings.TrimSpace(lines[i]), ">") &&
				!strings.HasPrefix(strings.TrimSpace(lines[i]), "#") &&
				!strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				para = append(para, strings.TrimSpace(lines[i]))
				i++
			}
			//mesg 防御：至少一行
			if len(para) > 0 {
				b.WriteString("<p>" + inline(strings.Join(para, "\n")) + "</p>")
			}
		}
	}
	b.WriteString("</section>")
	return b.String(), nil
}

// inline 处理行内元素：**strong** / *em* / `code` / 纯文本 escape。
func inline(s string) string {
	out := html.EscapeString(s)
	// code
	for strings.Contains(out, "`") {
		start := strings.IndexByte(out, '`')
		end := start + 1 + strings.IndexByte(out[start+1:], '`')
		if end <= start || start+1+strings.IndexByte(out[start+1:], '`') < 0 {
			break
		}
		// IndexByte 返回 -1 时上面条件会过滤；重新计算安全边界
		idx := strings.Index(out[start+1:], "`")
		if idx < 0 {
			break
		}
		end = start + 1 + idx
		out = out[:start] + "<code>" + out[start+1:end] + "</code>" + out[end+1:]
	}
	// **bold**
	for strings.Contains(out, "**") {
		start := strings.Index(out, "**")
		rest := strings.Index(out[start+2:], "**")
		if rest < 0 {
			break
		}
		end := start + 2 + rest
		out = out[:start] + "<b>" + out[start+2:end] + "</b>" + out[end+2:]
	}
	// 斜体 *em*
	for strings.Contains(out, "*") {
		start := strings.IndexByte(out, '*')
		rest := strings.IndexByte(out[start+1:], '*')
		if rest < 0 {
			break
		}
		end := start + 1 + rest
		out = out[:start] + "<i>" + out[start+1:end] + "</i>" + out[end+1:]
	}
	return strings.TrimSpace(out)
}

// peekBlock 消费 code fence 内容并推动当前行号至结束围栏之后。
func peekBlock(lines []string, i *int, fence string) []string {
	var out []string
	*i++ // 跳过开始围栏
	for *i < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[*i]), fence) {
			*i++ // 跳过结束围栏
			break
		}
		out = append(out, lines[*i])
		*i++
	}
	return out
}

func isTaskLine(t string) bool {
	return strings.HasPrefix(t, "- [x] ") || strings.HasPrefix(t, "- [X] ") || strings.HasPrefix(t, "- [ ] ")
}

func taskText(t string) string {
	t = strings.TrimPrefix(t, "- [x] ")
	t = strings.TrimPrefix(t, "- [X] ")
	return strings.TrimPrefix(t, "- [ ] ")
}

// writeClipboard 经 mygo 桌面剪贴板 API 写入（text/plain 版本；富文本版本若
// 底层支持 text/html 则优先）。mygo 0.4.8 Clipboard 仅支持 UTF-8 文本。
func writeClipboard(htmlBody, plain string) error {
	// mygo Clipboard 支持多格式 Data（transfer.HTML + transfer.Text），粘贴进
	// 富文本编辑器/浏览器可保留结构；纯文本环境取 text/plain 兑底。
	data := transfer.New(
		transfer.NewItem(
			transfer.Bytes(transfer.HTML, []byte(htmlBody)),
			transfer.Bytes(transfer.Text, []byte(plain)),
		),
	)
	return writeClipboardData(data)
}

// clipboardWriter 是剪贴板写入扩展点（单测注入断言用；默认真实写入）。
var clipboardWriter = func(data transfer.Data) error {
	return mygo.Clipboard.Write(data)
}

func writeClipboardData(data transfer.Data) error {
	return clipboardWriter(data)
}

// plainBodyForClipboard 把 HTML 主体抽回可读文本（剪贴板正文仍友好）。
func plainBodyForClipboard(htmlBody string) string {
	s := strings.NewReplacer(
		"<h1>", "\n", "</h1>", "\n",
		"<h2>", "\n", "</h2>", "\n",
		"<h3>", "\n", "</h3>", "\n",
		"<p>", "", "</p>", "\n",
		"<li>", "• ", "</li>", "\n",
		"<blockquote>", "「 ", "</blockquote>", " 」\n",
		"<pre><code>", "", "</code></pre>", "\n",
		"<b>", "**", "</b>", "**",
		"<i>", "*", "</i>", "*",
		"<code>", "`", "</code>", "`",
		"<hr/>", "———\n",
		"<input type=\"checkbox\" checked disabled>", "[x] ",
		"<input type=\"checkbox\" disabled>", "[ ] ",
	).Replace(htmlBody)
	return strings.ReplaceAll(s, "\n\n\n", "\n\n")
}
