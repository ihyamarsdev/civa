package infra

import infui "civa/internal/cli/infra/ui"

type outputBlock = infui.OutputBlock

func canStyleStdout() bool { return infui.CanStyleStdout() }
func canStyleStderr() bool { return infui.CanStyleStderr() }
func renderSectionTitle(title string, styled bool) string {
	return infui.RenderSectionTitle(title, styled)
}
func renderOutputBlock(block outputBlock, styled bool) string {
	return infui.RenderOutputBlock(block, styled)
}
func renderOutputBlocks(blocks []outputBlock, styled bool) string {
	return infui.RenderOutputBlocks(blocks, styled)
}
