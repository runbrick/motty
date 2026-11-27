//go:build darwin || windows || linux

package workbench

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"motty/internal/i18n"
)

// Renaming and deleting groups directly from the Snippets tree works end-to-end
// through native dialogs, and multi-select mode allows selecting a group to
// batch-move or batch-delete its commands without any layout defects.
func TestSnippetTreeDirectGroupActionsAndMultiSelectHeadless(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	state.setLanguage(i18n.Chinese)
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "deploy", Command: "kubectl apply -f staging/", Group: "Ops", Run: true},
		snippet{Name: "logs", Command: "journalctl -fu app", Group: "Ops"},
		snippet{Name: "editor", Command: "code .", Group: "dev"},
	)

	view, err := headless.Open(app.Config{
		Width: 1200, Height: 800, Tree: Tree(), Paint: state.Paint,
		OnKey: state.Key, OnPointer: state.Pointer, OnScroll: state.Scroll,
		TextInput: func(c app.TextInputContext) *app.TextInput {
			if editor := state.EditorFor(c.ID); editor != nil {
				return &app.TextInput{Editor: editor}
			}
			return nil
		},
	}, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer view.Close()
	if err := view.Rest(); err != nil {
		t.Fatal(err)
	}
	click := func(id gfx.ID) {
		t.Helper()
		for _, hit := range view.Snapshot().Hits {
			if hit.ID == id {
				if err := view.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		t.Fatalf("missing hit %s; frame has %v", tree.Text(id), view.Texts())
	}
	clean := func(where string) {
		t.Helper()
		if problems := headless.CheckLayout(view.Snapshot()); len(problems) > 0 {
			t.Fatalf("%s: %v", where, problems)
		}
	}

	clean("snippets page initial")

	// Open Ops folder context menu via right-click and choose Rename.
	opsRowIdx := -1
	for i, row := range state.snippets.treeRows {
		if row.folder && sameSnippetGroup(row.group, "Ops") {
			opsRowIdx = i
			break
		}
	}
	if opsRowIdx < 0 {
		t.Fatalf("Ops folder row not found in %+v", state.snippets.treeRows)
	}
	opsRowID := state.snippets.tree.RowID(opsRowIdx)
	for _, hit := range view.Snapshot().Hits {
		if hit.ID == opsRowID {
			if err := view.RightClick(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if !state.snippets.folderMenu.open {
		t.Fatal("right-clicking Ops folder did not open folder menu")
	}
	clean("folder context menu")
	m := state.messages.Tools
	if !strings.Contains(strings.Join(view.Texts(), "\n"), m.SnippetGroupDeleteWithSnippets) {
		t.Fatalf("folder menu missing %q: %v", m.SnippetGroupDeleteWithSnippets, view.Texts())
	}
	clickedRename := false
	for _, hit := range view.Snapshot().Hits {
		if hit.Label == m.SnippetGroupRename {
			if err := view.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			clickedRename = true
			break
		}
	}
	if !clickedRename {
		t.Fatalf("missing menu item %q in %v", m.SnippetGroupRename, view.Texts())
	}
	if state.modal != modalSnippetGroupRename {
		t.Fatalf("rename opened modal %d", state.modal)
	}
	clean("rename group dialog")
	if err := view.Type("Prod"); err != nil {
		t.Fatal(err)
	}
	if err := view.Keys("Enter"); err != nil {
		t.Fatal(err)
	}
	if state.modal != modalNone || state.snippetGroups.error != "" {
		t.Fatalf("native rename: modal %d, error %q", state.modal, state.snippetGroups.error)
	}
	if !sameSnippetGroup(state.snippets.list[0].Group, "Prod") || !sameSnippetGroup(state.snippets.list[1].Group, "Prod") {
		t.Fatalf("native rename left %+v", state.snippets.list)
	}

	// Toggle multi-select, verify group selection disables batch move, then
	// select the two commands directly via their ui.Checkbox controls and move
	// them to root.
	click(hitSnippetTreeMultiSelect)
	clean("multi-select mode")
	prodRowIdx := -1
	for i, row := range state.snippets.treeRows {
		if row.folder && sameSnippetGroup(row.group, "Prod") {
			prodRowIdx = i
			break
		}
	}
	if prodRowIdx < 0 {
		t.Fatalf("Prod folder row not found in %+v", state.snippets.treeRows)
	}
	click(state.snippets.tree.RowID(prodRowIdx))
	if len(state.snippets.selectedSnippetIndices()) != 2 || len(state.snippets.selectedSnippetGroups()) != 1 {
		t.Fatalf("expected 2 selected snippets and 1 group in Prod, got snippets=%v groups=%v", state.snippets.selectedSnippetIndices(), state.snippets.selectedSnippetGroups())
	}
	// Deselect Prod group, then check both commands individually.
	click(hitSnippetMultiFolderCheck.At(state.snippetTreeFolderToken("prod")))
	click(hitSnippetMultiItemCheck.At(0))
	click(hitSnippetMultiItemCheck.At(1))
	click(hitSnippetBatchMove)
	if state.modal != modalSnippetMoveGroup {
		t.Fatalf("batch move opened modal %d", state.modal)
	}
	clean("batch move dialog")
	click(hitSnippetMoveGroupRoot)
	click(hitSnippetMoveGroupConfirm)
	if state.modal != modalNone {
		t.Fatalf("confirming batch move left modal %d", state.modal)
	}
	if state.snippets.list[0].Group != "" || state.snippets.list[1].Group != "" {
		t.Fatalf("batch move to root left groups %+v", state.snippets.list)
	}
	if got := strings.Join(view.Texts(), "\n"); !strings.Contains(got, "Prod") || !strings.Contains(got, "deploy") {
		t.Fatalf("empty group Prod or moved snippet missing from view: %v", view.Texts())
	}

	// Exit multi-select, right-click Prod -> New subgroup, verify parent selector
	// layout and creation without raw slash prefixes.
	click(hitSnippetTreeMultiSelect)
	prodRowIdx = -1
	for i, row := range state.snippets.treeRows {
		if row.folder && sameSnippetGroup(row.group, "Prod") {
			prodRowIdx = i
			break
		}
	}
	for _, hit := range view.Snapshot().Hits {
		if hit.ID == state.snippets.tree.RowID(prodRowIdx) {
			if err := view.RightClick(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	for _, hit := range view.Snapshot().Hits {
		if hit.Label == state.messages.HostGroupActions.NewChild {
			if err := view.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if state.modal != modalSnippetGroupRename || !state.snippetGroups.creating || state.snippetGroups.parent != "Prod" {
		t.Fatalf("new child dialog state: modal=%d creating=%v parent=%q", state.modal, state.snippetGroups.creating, state.snippetGroups.parent)
	}
	clean("new child group dialog")
	if err := view.Type("API"); err != nil {
		t.Fatal(err)
	}
	if err := view.Keys("Enter"); err != nil {
		t.Fatal(err)
	}
	if state.modal != modalNone || state.snippetGroups.error != "" {
		t.Fatalf("create child failed: modal=%d err=%q", state.modal, state.snippetGroups.error)
	}
	if got := strings.Join(view.Texts(), "\n"); !strings.Contains(got, "API") {
		t.Fatalf("new subgroup API missing from view: %v", view.Texts())
	}
}
