package workbench

import (
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func snippetGroupsManagerState(t *testing.T) *State {
	t.Helper()
	state := newTestState(t, freshOptions(t))
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "deploy", Command: "kubectl apply -f staging/", Group: "Ops", Run: true},
		snippet{Name: "logs", Command: "journalctl -fu app", Group: "Ops"},
		snippet{Name: "editor", Command: "code .", Group: "dev"},
	)
	return state
}

// Renaming a group rewrites the name on every command that carries it, leaves
// the other groups alone, and keeps a filter on the renamed group following it.
func TestRenamingASnippetGroupRenamesItsCommandsAndFilter(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "deploy", Command: "kubectl apply -f staging/", Group: "Ops", Run: true},
		snippet{Name: "logs", Command: "journalctl -fu app", Group: "Ops"},
		snippet{Name: "editor", Command: "code .", Group: "dev"},
	)
	state.snippets.filter = snippetFilter{group: "Ops"}
	state.refreshSnippets()

	state.openSnippetGroupRenameDirect("Ops")
	if state.modal != modalSnippetGroupRename || state.snippetGroups.editor.String() != "Ops" {
		t.Fatalf("rename dialog: modal %d, name %q", state.modal, state.snippetGroups.editor.String())
	}
	state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, "Prod")
	state.Click(hitSnippetGroupRenameSubmit)
	if state.modal != modalNone {
		t.Fatalf("saving the rename did not close the dialog: modal %d", state.modal)
	}
	if got := []string{state.snippets.list[0].Group, state.snippets.list[1].Group, state.snippets.list[2].Group}; !slices.Equal(got, []string{"Prod", "Prod", "dev"}) {
		t.Fatalf("groups after rename = %v", got)
	}
	if state.snippets.filter.group != "Prod" {
		t.Fatalf("filter = %q, want the renamed group", state.snippets.filter.group)
	}
	if !slices.Equal(state.snippets.groups, []string{"dev", "Prod"}) {
		t.Fatalf("groups = %v", state.snippets.groups)
	}
	if state.snippets.list[0].Command != "kubectl apply -f staging/" || !state.snippets.list[0].Run || state.snippets.list[1].Command != "journalctl -fu app" {
		t.Fatal("rename changed a command or its run setting")
	}
	if !strings.Contains(state.status, "Prod") {
		t.Fatalf("status %q does not mention the rename", state.status)
	}

	restored := newTestState(t, options)
	if got := []string{restored.snippets.list[0].Group, restored.snippets.list[1].Group, restored.snippets.list[2].Group}; !slices.Equal(got, []string{"Prod", "Prod", "dev"}) {
		t.Fatalf("restored groups = %v", got)
	}
}

// A name another group already has is refused, as is a name no group may carry,
// and neither changes the commands.
func TestRenamingASnippetGroupRejectsInvalidAndConflictingNames(t *testing.T) {
	state := snippetGroupsManagerState(t)
	state.openSnippetGroupRenameDirect("Ops")

	for _, name := range []string{"", "   ", "dev", "DEV", strings.Repeat("g", maxSnippetGroup+1), "bad\x01name"} {
		state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, name)
		state.Click(hitSnippetGroupRenameSubmit)
		if state.modal != modalSnippetGroupRename || state.snippetGroups.error == "" {
			t.Fatalf("name %q: modal %d, error %q", name, state.modal, state.snippetGroups.error)
		}
		if got := []string{state.snippets.list[0].Group, state.snippets.list[1].Group, state.snippets.list[2].Group}; !slices.Equal(got, []string{"Ops", "Ops", "dev"}) {
			t.Fatalf("name %q changed groups to %v", name, got)
		}
	}
}

// A group's one spelling may change case without merging it into another group.
func TestRenamingASnippetGroupChangesOnlyItsCase(t *testing.T) {
	state := snippetGroupsManagerState(t)
	state.openSnippetGroupRenameDirect("Ops")
	state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, "ops")
	state.Click(hitSnippetGroupRenameSubmit)
	if state.modal != modalNone || state.snippetGroups.error != "" {
		t.Fatalf("case-only rename: modal %d, error %q", state.modal, state.snippetGroups.error)
	}
	if state.snippets.list[0].Group != "ops" || state.snippets.list[1].Group != "ops" || state.snippets.list[2].Group != "dev" {
		t.Fatalf("groups = %v", []string{state.snippets.list[0].Group, state.snippets.list[1].Group, state.snippets.list[2].Group})
	}
	if !slices.Equal(state.snippets.groups, []string{"dev", "ops"}) {
		t.Fatalf("groups = %v", state.snippets.groups)
	}
}

// Deleting a group keeps its commands on the page as ungrouped ones, at the
// same place and in the same order, and a filter on it shows everything again.
func TestDeletingASnippetGroupKeepsItsCommandsUngrouped(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "deploy", Command: "kubectl apply -f staging/", Group: "Ops", Run: true},
		snippet{Name: "logs", Command: "journalctl -fu app", Group: "Ops"},
		snippet{Name: "editor", Command: "code .", Group: "dev"},
	)
	state.snippets.filter = snippetFilter{group: "Ops"}
	state.refreshSnippets()

	state.openSnippetGroupDeleteDirect("Ops")
	if state.modal != modalSnippetGroupDelete {
		t.Fatalf("delete opened modal %d", state.modal)
	}
	_, texts := paintedTexts(state)
	if state.snippetGroupCount("Ops") != 2 || !contains(texts, "Ops") {
		t.Fatalf("the confirmation does not name the group and its count: %v", texts)
	}
	state.Click(hitSnippetGroupDeleteConfirm)
	if state.modal != modalNone {
		t.Fatalf("confirming the delete did not close the dialog: modal %d", state.modal)
	}
	if !slices.Equal(state.snippets.groups, []string{"dev"}) {
		t.Fatalf("groups after delete = %v", state.snippets.groups)
	}
	if !state.snippets.filter.all() {
		t.Fatalf("filter = %+v, want everything again", state.snippets.filter)
	}
	names := func() []string {
		return []string{state.snippets.list[0].Name, state.snippets.list[1].Name, state.snippets.list[2].Name}
	}
	if got := names(); !slices.Equal(got, []string{"deploy", "logs", "editor"}) {
		t.Fatalf("commands after delete = %v", got)
	}
	if state.snippets.list[0].Group != "" || state.snippets.list[1].Group != "" ||
		state.snippets.list[0].Command != "kubectl apply -f staging/" || !state.snippets.list[0].Run ||
		state.snippets.list[1].Command != "journalctl -fu app" {
		t.Fatalf("delete changed the commands: %+v", state.snippets.list)
	}
	if !strings.Contains(state.status, "Ops") {
		t.Fatalf("status %q does not mention the deleted group", state.status)
	}

	restored := newTestState(t, options)
	if restored.snippets.list[0].Group != "" || restored.snippets.list[1].Group != "" || restored.snippets.list[2].Group != "dev" {
		t.Fatalf("restored groups = %+v", restored.snippets.list)
	}
}

// Deleting a group together with all its commands removes the group, its
// subgroups, and every command beneath it, while leaving other groups intact.
func TestDeletingASnippetGroupWithSnippetsRemovesAllSubtreeCommands(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "deploy", Command: "kubectl apply -f staging/", Group: "Ops", Run: true},
		snippet{Name: "web", Command: "nginx -t", Group: "Ops/Web"},
		snippet{Name: "editor", Command: "code .", Group: "dev"},
	)

	state.snippets.folderMenu = snippetFolderMenu{open: true, group: "Ops", at: gfx.Point{X: 100, Y: 100}}
	state.chooseSnippetFolderMenu(hitSnippetFolderMenuDeleteWithSnippets)
	if state.modal != modalSnippetGroupDelete || !state.snippetGroups.deleteSnippets {
		t.Fatalf("delete with snippets opened modal=%d deleteSnippets=%v", state.modal, state.snippetGroups.deleteSnippets)
	}
	state.Click(hitSnippetGroupDeleteConfirm)
	if state.modal != modalNone {
		t.Fatalf("confirm left modal %d", state.modal)
	}
	if len(state.snippets.list) != 1 || state.snippets.list[0].Name != "editor" {
		t.Fatalf("remaining snippets = %+v, want only editor", state.snippets.list)
	}
	if !slices.Equal(state.snippets.groups, []string{"dev"}) {
		t.Fatalf("groups after delete with snippets = %v, want [dev]", state.snippets.groups)
	}
	restored := newTestState(t, options)
	if len(restored.snippets.list) != 1 || restored.snippets.list[0].Name != "editor" {
		t.Fatalf("restored snippets = %+v, want only editor", restored.snippets.list)
	}
}

func TestCancellingSnippetGroupDeleteKeepsTheGroup(t *testing.T) {
	state := snippetGroupsManagerState(t)
	state.openSnippetGroupDeleteDirect("Ops")
	state.Click(hitSnippetGroupDeleteCancel)
	if state.modal != modalNone {
		t.Fatalf("cancelling did not close the dialog: modal %d", state.modal)
	}
	if state.snippets.list[0].Group != "Ops" || state.snippets.list[1].Group != "Ops" || !slices.Contains(state.snippets.groups, "Ops") {
		t.Fatal("cancelling deleted the group")
	}
}

func TestCancellingSnippetGroupRenameKeepsTheName(t *testing.T) {
	state := snippetGroupsManagerState(t)
	state.openSnippetGroupRenameDirect("Ops")
	state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, "Prod")
	state.Click(hitSnippetGroupRenameCancel)
	if state.modal != modalNone || state.snippets.list[0].Group != "Ops" || state.snippets.list[1].Group != "Ops" {
		t.Fatal("cancelling renamed the group")
	}
}

func TestRenamingOrDeletingGroupUpdatesInlineEditor(t *testing.T) {
	state := snippetGroupsManagerState(t)
	state.focusInlineSnippet(1) // "logs", in Ops
	state.snippets.inline.name.Insert(" updated")
	state.applyInlineSnippetEdits()

	state.openSnippetGroupRenameDirect("Ops")
	state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, "Prod")
	state.Click(hitSnippetGroupRenameSubmit)
	if state.modal != modalNone {
		t.Fatalf("rename left modal %d", state.modal)
	}
	if state.snippets.inline.groupPicker.Value() != "Prod" {
		t.Fatalf("the editor's group field = %q, want the renamed group", state.snippets.inline.groupPicker.Value())
	}
	if state.snippets.inline.name.String() != "logs updated" {
		t.Fatalf("the editor lost its edited name: %q", state.snippets.inline.name.String())
	}

	state.openSnippetGroupDeleteDirect("Prod")
	state.Click(hitSnippetGroupDeleteConfirm)
	if state.snippets.inline.groupPicker.Value() != "" {
		t.Fatalf("the editor's group field = %q, want none", state.snippets.inline.groupPicker.Value())
	}
}

func TestRenamingANestedSnippetGroupRemapsItsSubtreeAndRejectsCycles(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "prod", Command: "echo prod", Group: "Clients/Acme/Prod"},
		snippet{Name: "stage", Command: "echo stage", Group: "Clients/Acme/Staging"},
		snippet{Name: "beta", Command: "echo beta", Group: "Clients/Beta"},
		snippet{Name: "ops", Command: "echo ops", Group: "Ops"},
	)
	state.snippets.filter = snippetFilter{group: "Clients/Acme/Prod"}
	state.refreshSnippets()

	state.openSnippetGroupRenameDirect("Clients")
	state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, "Clients/Sub")
	state.Click(hitSnippetGroupRenameSubmit)
	if state.modal != modalSnippetGroupRename || state.snippetGroups.error == "" {
		t.Fatalf("self-nesting rename was accepted: modal=%d err=%q", state.modal, state.snippetGroups.error)
	}

	state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, "Customers")
	state.Click(hitSnippetGroupRenameSubmit)
	if state.modal != modalNone || state.snippetGroups.error != "" {
		t.Fatalf("rename failed: modal=%d err=%q", state.modal, state.snippetGroups.error)
	}
	got := []string{
		state.snippets.list[0].Group,
		state.snippets.list[1].Group,
		state.snippets.list[2].Group,
		state.snippets.list[3].Group,
	}
	want := []string{"Customers/Acme/Prod", "Customers/Acme/Staging", "Customers/Beta", "Ops"}
	if !slices.Equal(got, want) {
		t.Fatalf("groups after rename = %v, want %v", got, want)
	}
	if state.snippets.filter.group != "Customers/Acme/Prod" {
		t.Fatalf("filter = %q, want Customers/Acme/Prod", state.snippets.filter.group)
	}
}

func TestDeletingANestedSnippetGroupUngroupsItsSubtree(t *testing.T) {
	state := newTestState(t, freshOptions(t))
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "prod", Command: "echo prod", Group: "Clients/Acme/Prod"},
		snippet{Name: "stage", Command: "echo stage", Group: "Clients/Acme/Staging"},
		snippet{Name: "beta", Command: "echo beta", Group: "Clients/Beta"},
	)
	state.snippets.filter = snippetFilter{group: "Clients/Acme/Prod"}
	state.refreshSnippets()

	state.openSnippetGroupDeleteDirect("Clients/Acme")
	if state.snippetGroupCount("Clients/Acme") != 2 {
		t.Fatalf("subtree count = %d, want 2", state.snippetGroupCount("Clients/Acme"))
	}
	state.Click(hitSnippetGroupDeleteConfirm)
	got := []string{state.snippets.list[0].Group, state.snippets.list[1].Group, state.snippets.list[2].Group}
	if !slices.Equal(got, []string{"", "", "Clients/Beta"}) {
		t.Fatalf("groups after subtree delete = %v", got)
	}
	if !state.snippets.filter.all() {
		t.Fatalf("filter = %+v, want reset to all", state.snippets.filter)
	}
}

// Dragging a grouped snippet onto its own top-level folder or using the Move
// to Group dialog allows moving it to root (ungrouped), while keeping the
// now-empty source group visible in the tree.
func TestSnippetMoveToRootAndEmptyGroupRemainsVisible(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "only", Command: "echo only", Group: "Ops"},
	)

	// Tree rows: Ops(0), only(1). Dragging "only" onto "Ops" rejects Inside
	// (already in Ops) and allows Before/After (parent of Ops = root "").
	if state.snippetTreeCanDrop(1, 0, ui.TreeDropInside) {
		t.Fatal("dropping snippet inside its own group should be rejected")
	}
	if !state.snippetTreeCanDrop(1, 0, ui.TreeDropBefore) {
		t.Fatal("dropping snippet before its top-level folder should move it to root")
	}
	state.dropSnippetTreeRow(ui.TreeMove{From: 1, To: 0, Where: ui.TreeDropBefore})
	if state.snippets.list[0].Group != "" {
		t.Fatalf("snippet group after root drop = %q, want empty", state.snippets.list[0].Group)
	}
	if !slices.Contains(state.snippets.groups, "Ops") {
		t.Fatalf("emptied group Ops vanished: groups=%v", state.snippets.groups)
	}

	// Move back into Ops via Move to Group dialog, then delete the snippet:
	// the empty group Ops must still be displayed in the tree even with 0 snippets.
	state.openSnippetMoveGroup([]int{0})
	if state.modal != modalSnippetMoveGroup {
		t.Fatalf("move dialog opened modal %d", state.modal)
	}
	state.Click(hitSnippetMoveGroupChoices.At(0))
	state.Click(hitSnippetMoveGroupConfirm)
	if state.snippets.list[0].Group != "Ops" {
		t.Fatalf("snippet group after move dialog = %q, want Ops", state.snippets.list[0].Group)
	}

	state.deleteSnippet(0)
	if len(state.snippets.list) != 0 {
		t.Fatalf("list after delete = %+v", state.snippets.list)
	}
	if !slices.Equal(state.snippets.groups, []string{"Ops"}) || len(state.snippets.treeRows) != 1 || !state.snippets.treeRows[0].folder {
		t.Fatalf("empty group not kept in tree: groups=%v treeRows=%+v", state.snippets.groups, state.snippets.treeRows)
	}
	var canvas gfx.Canvas
	paintSnippetsPage(state, &canvas)
	if !hasTextValue(&canvas, "Ops") {
		t.Fatal("empty group Ops was not painted when snippet list is empty")
	}
}

// Multi-select mode lets the user select a group to select all of its commands
// (including nested subgroups), batch-move them to root or another group, and
// batch-delete them.
func TestSnippetMultiSelectGroupSelectionBatchMoveAndBatchDelete(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	state.activeSection = sectionSnippets
	addTestSnippets(state,
		snippet{Name: "deploy", Command: "echo deploy", Group: "Ops"},
		snippet{Name: "web", Command: "echo web", Group: "Ops/Web"},
		snippet{Name: "editor", Command: "code .", Group: "Dev"},
	)

	state.Click(hitSnippetTreeMultiSelect)
	if !state.snippets.multiSelect {
		t.Fatal("multi-select mode did not activate")
	}

	// Clicking the Ops folder checkbox selects both Ops and Ops/Web snippets.
	opsToken := state.snippetTreeFolderToken("ops")
	state.Click(hitSnippetMultiFolderCheck.At(opsToken))
	if got := state.snippets.selectedSnippetIndices(); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("selected after clicking Ops group = %v, want [0 1]", got)
	}

	// Batch move selected snippets to root (ungrouped).
	state.Click(hitSnippetBatchMove)
	if state.modal != modalSnippetMoveGroup {
		t.Fatalf("batch move opened modal %d", state.modal)
	}
	state.Click(hitSnippetMoveGroupRoot)
	state.Click(hitSnippetMoveGroupConfirm)
	if state.snippets.list[0].Group != "" || state.snippets.list[1].Group != "" || state.snippets.list[2].Group != "Dev" {
		t.Fatalf("groups after batch move to root = %+v", state.snippets.list)
	}
	// Source groups Ops and Ops/Web remain visible as empty groups.
	if !slices.Contains(state.snippets.groups, "Ops") || !slices.Contains(state.snippets.groups, "Ops/Web") {
		t.Fatalf("emptied groups disappeared after batch move: %v", state.snippets.groups)
	}

	// Batch delete removes the selected snippets AND the selected Ops + Ops/Web groups.
	state.Click(hitSnippetBatchDelete)
	if state.modal != modalSnippetDelete || len(state.snippets.deletingBatch) != 2 || len(state.snippets.deletingGroups) != 2 {
		t.Fatalf("batch delete opened modal=%d batch=%v groups=%v", state.modal, state.snippets.deletingBatch, state.snippets.deletingGroups)
	}
	state.Click(hitSnippetDeleteConfirm)
	if len(state.snippets.list) != 1 || state.snippets.list[0].Name != "editor" {
		t.Fatalf("remaining snippets after batch delete = %+v, want only editor", state.snippets.list)
	}
	if !slices.Equal(state.snippets.groups, []string{"Dev"}) {
		t.Fatalf("selected groups Ops/Web were not deleted with batch delete: groups=%v", state.snippets.groups)
	}

	// Selecting an empty group alone allows deleting that empty group via batch delete.
	state.createSnippetGroup("EmptyDir")
	emptyToken := state.snippetTreeFolderToken("emptydir")
	state.Click(hitSnippetMultiFolderCheck.At(emptyToken))
	state.Click(hitSnippetBatchDelete)
	if state.modal != modalSnippetDelete || len(state.snippets.deletingGroups) != 1 || len(state.snippets.deletingBatch) != 0 {
		t.Fatalf("empty group batch delete modal=%d groups=%v batch=%v", state.modal, state.snippets.deletingGroups, state.snippets.deletingBatch)
	}
	state.Click(hitSnippetDeleteConfirm)
	if slices.Contains(state.snippets.groups, "EmptyDir") {
		t.Fatalf("empty group EmptyDir was not deleted: groups=%v", state.snippets.groups)
	}
}

// Creating a subgroup keeps the parent path in the Parent selector rather than
// prefilling slashes in the Group name field, and renaming a nested group edits
// only its leaf name while preserving its parent path.
func TestSnippetGroupDialogSeparatesParentFromLeafName(t *testing.T) {
	state := newTestState(t, freshOptions(t))
	state.activeSection = sectionSnippets
	addTestSnippets(state, snippet{Name: "cmd", Command: "echo hi", Group: "23/2222/4444"})

	// Creating a child under 23/2222/4444 prefills parent, leaves name empty.
	state.openSnippetGroupCreate("23/2222/4444")
	if state.snippetGroups.parent != "23/2222/4444" || state.snippetGroups.editor.String() != "" {
		t.Fatalf("create child state: parent=%q editor=%q", state.snippetGroups.parent, state.snippetGroups.editor.String())
	}
	state.snippetGroups.editor.Insert("logs")
	state.Click(hitSnippetGroupRenameSubmit)
	if state.modal != modalNone || !slices.Contains(state.snippets.groups, "23/2222/4444/logs") {
		t.Fatalf("creating child failed: modal=%d err=%q groups=%v", state.modal, state.snippetGroups.error, state.snippets.groups)
	}

	// Renaming a nested group prefills only the leaf name and keeps its parent.
	state.openSnippetGroupRenameDirect("23/2222/4444")
	if state.snippetGroups.parent != "23/2222" || state.snippetGroups.editor.String() != "4444" {
		t.Fatalf("rename nested state: parent=%q editor=%q", state.snippetGroups.parent, state.snippetGroups.editor.String())
	}
	state.snippetGroups.editor.ReplaceRange(text.Range{Start: 0, End: state.snippetGroups.editor.RuneCount()}, "5555")
	state.Click(hitSnippetGroupRenameSubmit)
	if state.modal != modalNone || state.snippets.list[0].Group != "23/2222/5555" {
		t.Fatalf("rename nested failed: modal=%d err=%q group=%q", state.modal, state.snippetGroups.error, state.snippets.list[0].Group)
	}
}
