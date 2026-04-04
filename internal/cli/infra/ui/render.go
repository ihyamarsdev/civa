package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("14")).
				Foreground(lipgloss.Color("14")).
				Padding(0, 1)

	outputBlockStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("12")).
				Padding(0, 1)

	outputBlockTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("12"))
)

type OutputBlock struct {
	Title string
	Lines []string
}

func CanStyleStdout() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func CanStyleStderr() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

func RenderSectionTitle(title string, styled bool) string {
	if !styled {
		return fmt.Sprintf("\n== %s ==\n----------------------------------------", title)
	}

	return sectionTitleStyle.Render(title)
}

func RenderOutputBlock(block OutputBlock, styled bool) string {
	if !styled {
		if len(block.Lines) == 0 {
			return block.Title
		}
		return block.Title + ":\n" + strings.Join(block.Lines, "\n")
	}

	title := outputBlockTitleStyle.Render(block.Title)
	body := " "
	if len(block.Lines) == 0 {
		body = " "
	} else {
		body = strings.Join(block.Lines, "\n")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, body)
	return outputBlockStyle.Render(content)
}

func RenderOutputBlocks(blocks []OutputBlock, styled bool) string {
	rendered := make([]string, 0, len(blocks))
	for _, block := range blocks {
		rendered = append(rendered, RenderOutputBlock(block, styled))
	}

	return strings.Join(rendered, "\n\n")
}
