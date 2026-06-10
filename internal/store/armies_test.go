package store

import (
	"path/filepath"
	"testing"
)

func TestCatalogImportAndFilters(t *testing.T) {
	st := openTestStore(t)
	units, err := st.CatalogUnits("", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 106 {
		t.Fatalf("catalog has %d units, want 106", len(units))
	}

	filters, err := st.CatalogFilters()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(filters["nations"], "Dwarf") {
		t.Fatalf("filters missing Dwarf nation: %#v", filters["nations"])
	}
	if !contains(filters["terrains"], "Dwarf City") {
		t.Fatalf("filters missing Dwarf City terrain")
	}

	dwarves, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(dwarves) != 22 {
		t.Fatalf("Dwarf catalog count = %d, want 22", len(dwarves))
	}
	if dwarves[0].UnitName != "Barghest" {
		t.Fatalf("catalog not sorted by name, first = %q", dwarves[0].UnitName)
	}
}

func TestTemplateRosterAndArmyUnitSetup(t *testing.T) {
	st := openTestStore(t)
	catalog, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}
	unit := catalog[0]

	tpl, err := st.CreateArmyTemplate("Dwarf Patrol", 200)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = st.AddTemplateUnit(tpl.ID, unit.ID, "Stone Watch", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tpl.Units) != 1 {
		t.Fatalf("template units = %d, want 1", len(tpl.Units))
	}
	if tpl.Units[0].MiniCount != 3 {
		t.Fatalf("default mini count = %d, want one 50x50 rank of 3", tpl.Units[0].MiniCount)
	}
	if tpl.TotalPoints != unit.Pts*tpl.Units[0].MiniCount {
		t.Fatalf("template points = %d, want %d", tpl.TotalPoints, unit.Pts*tpl.Units[0].MiniCount)
	}

	army, err := st.CreateArmyFromTemplate(tpl.ID, "Campaign Patrol")
	if err != nil {
		t.Fatal(err)
	}
	if len(army.Units) != 1 {
		t.Fatalf("army units = %d, want 1", len(army.Units))
	}
	line := army.Units[0]
	if line.Moniker != "Stone Watch" {
		t.Fatalf("moniker = %q, want Stone Watch", line.Moniker)
	}
	if line.CurrentHealth != unit.H || line.MaxHealth != unit.H {
		t.Fatalf("health = %d/%d, want %d/%d", line.CurrentHealth, line.MaxHealth, unit.H, unit.H)
	}

	setups, err := st.ArmyUnitSetups(army.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(setups) != 1 {
		t.Fatalf("setups = %d, want 1", len(setups))
	}
	setup := setups[0]
	if setup.Name != "Stone Watch" || setup.CatalogUnitID != unit.ID || setup.ArmyID != army.ID || setup.ArmyUnitID != line.ID {
		t.Fatalf("setup did not preserve roster identity: %#v", setup)
	}
	if setup.Stats.Pts != unit.Pts || setup.MaxHealth != unit.H || setup.CurrentHealth != unit.H {
		t.Fatalf("setup did not preserve stats/health: %#v", setup)
	}
}

func TestForeignKeysRejectOrphanArmyUnits(t *testing.T) {
	st := openTestStore(t)
	catalog, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := st.AddArmyUnit("missing-army", catalog[0].ID, "Orphan", 1); err == nil {
		t.Fatal("expected foreign key error")
	}
	var count int
	if err := st.db.QueryRow(`select count(*) from army_units where army_id = ?`, "missing-army").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("orphan rows = %d, want 0", count)
	}
}

func TestDefaultMiniCountUsesOneRank(t *testing.T) {
	st := openTestStore(t)
	units, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}
	var infantry CatalogUnit
	for _, unit := range units {
		if unit.BaseWidthMM == 25 && unit.BaseDepthMM == 25 {
			infantry = unit
			break
		}
	}
	if infantry.ID == "" {
		t.Fatal("missing 25x25 infantry fixture")
	}
	tpl, err := st.CreateArmyTemplate("Rank Defaults", 100)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = st.AddTemplateUnit(tpl.ID, infantry.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Units[0].MiniCount != 5 {
		t.Fatalf("default mini count = %d, want one 25x25 rank of 5", tpl.Units[0].MiniCount)
	}
	if tpl.TotalPoints != infantry.Pts*5 {
		t.Fatalf("template points = %d, want %d", tpl.TotalPoints, infantry.Pts*5)
	}

	tpl, err = st.UpdateTemplateUnit(tpl.ID, tpl.Units[0].ID, tpl.Units[0].DefaultMoniker, 7)
	if err != nil {
		t.Fatal(err)
	}
	if tpl.TotalPoints != infantry.Pts*7 {
		t.Fatalf("updated template points = %d, want %d", tpl.TotalPoints, infantry.Pts*7)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
