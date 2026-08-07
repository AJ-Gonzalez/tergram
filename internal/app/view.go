package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"tergram/internal/tgc"
	"tergram/internal/ui"
)

func (m Model) View() tea.View {
	if m.level() == 0 {
		return tea.NewView(m.chatListView())
	}
	return tea.NewView(m.chatView())
}

// chatListView renders level 0: the conversation list.
func (m Model) chatListView() string {
	var sb strings.Builder
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Left, ui.Title.Render("tergram")) + "\n")
	sb.WriteString(ui.Dim.Render(strings.Repeat("─", max(m.width, 0))) + "\n")

	if m.status != "" {
		sb.WriteString(ui.Dim.Render(m.status) + "\n")
	}
	if m.err != "" {
		sb.WriteString(ui.Err.Render(m.err) + "\n")
	}
	if len(m.dialogs) == 0 {
		if m.err == "" {
			if m.dialogsLoaded {
				sb.WriteString(ui.Dim.Render("no conversations — press q to quit"))
			} else {
				sb.WriteString(ui.Dim.Render("loading conversations…"))
			}
		}
		return sb.String()
	}

	for i, d := range m.dialogs {
		line := fmt.Sprintf(" %-40s %s", truncate(d.Title, 40), d.LastText)
		if i == m.listIdx {
			line = ui.Highlight.Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(line)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n" + ui.Hint.Render("j/k or ↑/↓ move · space open · q quit"))
	return sb.String()
}

// chatView renders level 1: message pane + composer.
func (m Model) chatView() string {
	d, ok := m.currentDialog()
	title := "?"
	if ok {
		title = d.Title
	}
	var sb strings.Builder
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Left, ui.Title.Render(title)) + "\n")
	sb.WriteString(ui.Dim.Render(strings.Repeat("─", max(m.width, 0))) + "\n")

	if m.status != "" {
		sb.WriteString(ui.Dim.Render(m.status) + "\n")
	}
	if m.loadingChat {
		sb.WriteString(ui.Dim.Render("loading…"))
		return sb.String()
	}
	if m.err != "" {
		sb.WriteString(ui.Err.Render(m.err) + "\n")
	}

	start, end := m.msgWindow()
	for i := start; i <= end; i++ {
		msg := m.messages[i]
		line := renderMessage(msg)
		if i == m.msgCur {
			line = ui.Highlight.Render(line)
		}
		sb.WriteString(line + "\n")
	}

	space := m.height - 4 - (end - start + 1)
	if space > 0 {
		sb.WriteString(strings.Repeat("\n", space))
	}
	sb.WriteString(ui.Dim.Render(strings.Repeat("─", max(m.width, 0))) + "\n")
	sb.WriteString(ui.Hint.Render("> ") + renderComposer(m.composer, m.composerN))
	sb.WriteString("\n" + ui.Hint.Render("enter send · esc/q back"))
	return sb.String()
}

func renderMessage(msg tgc.Message) string {
	time := "        "
	if msg.At > 0 {
		time = fmt.Sprintf("%02d:%02d", (msg.At%86400+86400)%86400/3600, (msg.At%86400)%3600/60)
	}
	sender := msg.Sender
	if sender == "" {
		if msg.Outgoing {
			sender = "You"
		} else {
			sender = "?"
		}
	}
	head := ui.Sender.Render(sender) + " " + ui.Time.Render(time)
	body := messageBody(msg)
	return head + "\n    " + body
}

func messageBody(msg tgc.Message) string {
	if msg.Outgoing {
		return ui.OutMsg.Render(msg.Text)
	}
	return ui.InMsg.Render(msg.Text)
}

func renderComposer(runes []rune, caret int) string {
	before := string(runes[:min(caret, len(runes))])
	after := string(runes[min(caret, len(runes)):])
	return before + "▏" + after
}

// msgWindow returns the [start,end] inclusive slice of m.messages to render.
func (m Model) msgWindow() (int, int) {
	n := len(m.messages)
	if n == 0 {
		return 0, -1
	}
	visible := m.chatVisible()
	if visible < 1 {
		visible = 1
	}
	cur := m.msgCur
	if cur < 0 {
		cur = 0
	}
	if cur >= n {
		cur = n - 1
	}
	start := cur - (visible - 1)
	if start < 0 {
		start = 0
	}
	end := start + visible - 1
	if end >= n {
		end = n - 1
		start = end - (visible - 1)
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
