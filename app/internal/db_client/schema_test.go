package db_client

import (
	"testing"

	"selectDb/internal/graph"

	core "github.com/selectDb/dialect/core"
)

// childNamesOfType returns the names of the nodes under the table's child
// section of the given type ("indexes", "triggers", "columns") — i.e. what the
// sidebar renders and what the badge counts.
func childNamesOfType(t *testing.T, table *graph.DBInstanceItemNode, sectionType string) []string {
	t.Helper()
	for _, section := range table.Children {
		if section.Type != sectionType {
			continue
		}
		names := make([]string, 0, len(section.Children))
		for _, node := range section.Children {
			names = append(names, node.Name)
		}
		return names
	}
	t.Fatalf("table %q has no %q section", table.Name, sectionType)
	return nil
}

func columnNode(t *testing.T, table *graph.DBInstanceItemNode, name string) *graph.DBInstanceItemNode {
	t.Helper()
	for _, section := range table.Children {
		if section.Type != "columns" {
			continue
		}
		for _, col := range section.Children {
			if col.Name == name {
				return col
			}
		}
	}
	t.Fatalf("table %q has no column %q", table.Name, name)
	return nil
}

func tableNode(t *testing.T, tables []*graph.DBInstanceItemNode, name string) *graph.DBInstanceItemNode {
	t.Helper()
	for _, table := range tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("no table node named %q", name)
	return nil
}

func hasIndexMeta(t *testing.T, node *graph.DBInstanceItemNode) bool {
	t.Helper()
	meta, ok := node.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("node %q has no metadata map", node.Name)
	}
	return meta["hasIndex"] == true
}

// TestConvertTablesToNodesGroupsByExactTableName covers the case where one
// table's name is a prefix of another's ("alpha" / "alphabet"). Grouping the
// indexes and triggers by node-ID prefix used to leak every "alphabet" object
// onto "alpha", which then attached the leaked index to any "alpha" column that
// happened to share a name with an indexed "alphabet" column.
func TestConvertTablesToNodesGroupsByExactTableName(t *testing.T) {
	const dbID = "db:schema:s"
	const schemaPath = "db / s"

	tables := []core.Table{
		{Name: "alpha", Columns: []core.Column{{Name: "id"}, {Name: "label"}}},
		{Name: "alphabet", Columns: []core.Column{{Name: "label"}, {Name: "owner_id"}}},
	}
	indexes := []core.IndexInfo{
		{Name: "alpha_pkey", TableName: "alpha", Columns: []core.IndexColumnInfo{
			{Name: "id", Position: 1},
		}},
		{Name: "alphabet_label_owner_id_key", TableName: "alphabet", Columns: []core.IndexColumnInfo{
			{Name: "label", Position: 1},
			{Name: "owner_id", Position: 2},
		}},
	}
	triggers := []core.TriggerInfo{
		{Name: "alphabet_touch", TableName: "alphabet"},
	}

	_, indexesByTable := convertIndexesToNodes(indexes, dbID, schemaPath)
	_, triggersByTable := convertTriggersToNodes(triggers, dbID, schemaPath)
	tableNodes, err := convertTablesToNodes(tables, dbID, schemaPath, nil, indexesByTable, triggersByTable)
	if err != nil {
		t.Fatalf("convertTablesToNodes: %v", err)
	}

	alpha := tableNode(t, tableNodes, "alpha")
	alphabet := tableNode(t, tableNodes, "alphabet")

	// "alpha" owns only its own index and no triggers.
	if got := childNamesOfType(t, alpha, "indexes"); len(got) != 1 || got[0] != "alpha_pkey" {
		t.Errorf("alpha indexes = %v, want [alpha_pkey]", got)
	}
	if got := childNamesOfType(t, alpha, "triggers"); len(got) != 0 {
		t.Errorf("alpha triggers = %v, want []", got)
	}

	// The prefix-sharing table keeps everything that is genuinely its own.
	if got := childNamesOfType(t, alphabet, "indexes"); len(got) != 1 || got[0] != "alphabet_label_owner_id_key" {
		t.Errorf("alphabet indexes = %v, want [alphabet_label_owner_id_key]", got)
	}
	if got := childNamesOfType(t, alphabet, "triggers"); len(got) != 1 || got[0] != "alphabet_touch" {
		t.Errorf("alphabet triggers = %v, want [alphabet_touch]", got)
	}

	// "label" exists in both tables but is indexed only in "alphabet", so
	// alpha.label must carry neither the index child nor the hasIndex flag.
	alphaLabel := columnNode(t, alpha, "label")
	if len(alphaLabel.Children) != 0 {
		t.Errorf("alpha.label has %d index children, want 0", len(alphaLabel.Children))
	}
	if hasIndexMeta(t, alphaLabel) {
		t.Error("alpha.label hasIndex = true, want false")
	}

	alphaID := columnNode(t, alpha, "id")
	if len(alphaID.Children) != 1 || alphaID.Children[0].Name != "alpha_pkey" {
		t.Errorf("alpha.id index children = %v, want [alpha_pkey]", alphaID.Children)
	}
	if !hasIndexMeta(t, alphaID) {
		t.Error("alpha.id hasIndex = false, want true")
	}

	alphabetLabel := columnNode(t, alphabet, "label")
	if len(alphabetLabel.Children) != 1 || alphabetLabel.Children[0].Name != "alphabet_label_owner_id_key" {
		t.Errorf("alphabet.label index children = %v, want [alphabet_label_owner_id_key]", alphabetLabel.Children)
	}
	if !hasIndexMeta(t, alphabetLabel) {
		t.Error("alphabet.label hasIndex = false, want true")
	}
}

// TestConvertIndexesToNodesGroupsAllIndexesOfATable checks the grouping itself:
// every index lands under its own table, and the flat list stays complete for
// the schema-level Indexes section.
func TestConvertIndexesToNodesGroupsAllIndexesOfATable(t *testing.T) {
	indexes := []core.IndexInfo{
		{Name: "one_a_idx", TableName: "one"},
		{Name: "two_a_idx", TableName: "two"},
		{Name: "one_b_idx", TableName: "one"},
	}

	flat, byTable := convertIndexesToNodes(indexes, "db:schema:s", "db / s")

	if len(flat) != len(indexes) {
		t.Errorf("flat list has %d nodes, want %d", len(flat), len(indexes))
	}
	if len(byTable) != 2 {
		t.Errorf("grouping has %d tables, want 2", len(byTable))
	}
	if got := byTable["one"]; len(got) != 2 {
		t.Errorf(`byTable["one"] has %d indexes, want 2`, len(got))
	} else if got[0].Name != "one_a_idx" || got[1].Name != "one_b_idx" {
		t.Errorf(`byTable["one"] = [%s %s], want [one_a_idx one_b_idx]`, got[0].Name, got[1].Name)
	}
	if got := len(byTable["two"]); got != 1 {
		t.Errorf(`byTable["two"] has %d indexes, want 1`, got)
	}
}
