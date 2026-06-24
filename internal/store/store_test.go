package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"miniswar/internal/game"
)

func TestOpenCreatesMissingParentDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "storage", "nested", "miniswar.sqlite")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Fatalf("database parent directory was not created: %v", err)
	}
}

func TestEnsureSQLiteParentDirSkipsSQLiteDSNs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")

	if err := ensureSQLiteParentDir("file:" + filepath.Join(dir, "miniswar.sqlite")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("file: DSN parent directory stat error = %v, want not exist", err)
	}

	if err := ensureSQLiteParentDir(":memory:"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenExistingDatabasePreservesUserCreatedRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "storage", "miniswar.sqlite")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	battlemap, err := st.CreateBattlemap(game.Battlemap{
		Name:     "Campaign Field",
		WidthMM:  900,
		HeightMM: 600,
	})
	if err != nil {
		t.Fatal(err)
	}
	template, err := st.CreateArmyTemplate("Campaign Template", 300)
	if err != nil {
		t.Fatal(err)
	}
	army, err := st.CreateArmy("Campaign Army", 300)
	if err != nil {
		t.Fatal(err)
	}
	savedGame := &game.Game{
		ID:        "campaign-game",
		CreatedAt: time.Now().UTC(),
		Battlemap: battlemap,
	}
	if err := st.SaveGame(savedGame); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	if _, err := reopened.GetBattlemap(battlemap.ID); err != nil {
		t.Fatalf("user battlemap was not preserved: %v", err)
	}
	if _, err := reopened.GetArmyTemplate(template.ID); err != nil {
		t.Fatalf("user army template was not preserved: %v", err)
	}
	if _, err := reopened.GetArmy(army.ID); err != nil {
		t.Fatalf("user army was not preserved: %v", err)
	}
	if _, err := reopened.GetGame(savedGame.ID); err != nil {
		t.Fatalf("user game was not preserved: %v", err)
	}
}
