package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"miniswar/internal/game"
)

func TestOpenSeedsBuiltInBattlemaps(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	battlemaps, err := st.ListBattlemaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(battlemaps) < 2 {
		t.Fatalf("seeded battlemaps = %d, want at least 2", len(battlemaps))
	}
	oldRoad, err := st.GetBattlemap("old_road")
	if err != nil {
		t.Fatal(err)
	}
	if !oldRoad.IsBuiltin {
		t.Fatal("old road should be marked built in")
	}
	if oldRoad.WidthMM != game.ArenaWidthMM || oldRoad.HeightMM != game.ArenaHeightMM || len(oldRoad.Terrains) == 0 {
		t.Fatalf("old road = %#v, want dimensions and terrain", oldRoad)
	}
}

func TestBattlemapCRUD(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	created, err := st.CreateBattlemap(game.Battlemap{
		Name:     "Big Field",
		WidthMM:  1200,
		HeightMM: 800,
		Terrains: []game.TerrainZone{{
			ID:     "rough-1",
			Type:   game.TerrainRough,
			Label:  "rough",
			Shape:  "rect",
			X:      100,
			Y:      120,
			Width:  200,
			Height: 80,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Name != "Big Field" || created.WidthMM != 1200 || len(created.Terrains) != 1 {
		t.Fatalf("created battlemap = %#v", created)
	}

	updated, err := st.UpdateBattlemap(created.ID, game.Battlemap{
		Name:     "Bigger Field",
		WidthMM:  1400,
		HeightMM: 900,
		Terrains: []game.TerrainZone{{
			ID:     "wall-1",
			Type:   game.TerrainImpassable,
			Label:  "wall",
			Shape:  "rect",
			X:      300,
			Y:      200,
			Width:  40,
			Height: 300,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Name != "Bigger Field" || updated.WidthMM != 1400 || updated.Terrains[0].Type != game.TerrainImpassable {
		t.Fatalf("updated battlemap = %#v", updated)
	}

	if err := st.DeleteBattlemap(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetBattlemap(created.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted battlemap lookup error = %v, want sql.ErrNoRows", err)
	}
}

func TestBattlemapPersistenceRejectsInvalidGeometry(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	_, err = st.CreateBattlemap(game.Battlemap{
		Name:     "Bad Field",
		WidthMM:  100,
		HeightMM: 100,
		Terrains: []game.TerrainZone{{
			ID:     "rough-1",
			Type:   game.TerrainRough,
			Label:  "rough",
			Shape:  "rect",
			X:      90,
			Y:      90,
			Width:  20,
			Height: 20,
		}},
	})
	if err == nil {
		t.Fatal("expected invalid geometry error")
	}
}

func TestMutatingBuiltInBattlemapIsRejected(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if _, err := st.UpdateBattlemap("old_road", game.Battlemap{Name: "Changed", WidthMM: 800, HeightMM: 600}); err == nil {
		t.Fatal("expected built-in update to be rejected")
	}
	if err := st.DeleteBattlemap("old_road"); err == nil {
		t.Fatal("expected built-in deletion to be rejected")
	}
	if _, err := st.GetBattlemap("old_road"); err != nil {
		t.Fatalf("built-in battlemap should remain after rejected delete: %v", err)
	}
}
