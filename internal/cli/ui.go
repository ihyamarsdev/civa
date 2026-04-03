package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/alperdrsnn/clime"
	"golang.org/x/term"
)

type outputBlock struct {
	Title string
	Lines []string
}

func canStyleStdout() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func canStyleStderr() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

func renderSectionTitle(title string, styled bool) string {
	if !styled {
		return fmt.Sprintf("\n== %s ==\n----------------------------------------", title)
	}

	return clime.NewBox().
		WithStyle(clime.BoxStyleBold).
		WithBorderColor(clime.CyanColor).
		WithTitleColor(clime.CyanColor).
		WithPadding(0).
		AutoSize(true).
		AddLine(title).
		Render()
}

func renderOutputBlock(block outputBlock, styled bool) string {
	if !styled {
		if len(block.Lines) == 0 {
			return block.Title
		}
		return block.Title + ":\n" + strings.Join(block.Lines, "\n")
	}

	box := clime.NewBox().
		WithTitle(block.Title).
		WithStyle(clime.BoxStyleRounded).
		WithBorderColor(clime.BlueColor).
		WithTitleColor(clime.BlueColor).
		WithPadding(1)

	if len(block.Lines) == 0 {
		box.AddLine(" ")
	} else {
		box.AddLines(block.Lines...)
	}

	return box.Render()
}

func renderOutputBlocks(blocks []outputBlock, styled bool) string {
	rendered := make([]string, 0, len(blocks))
	for _, block := range blocks {
		rendered = append(rendered, renderOutputBlock(block, styled))
	}
	return strings.Join(rendered, "\n\n")
}
