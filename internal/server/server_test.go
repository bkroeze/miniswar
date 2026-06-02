package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"miniswar/internal/game"
	"miniswar/internal/store"
)

func TestCreateActivateActionAndRewind(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	createBody := `{"player1":{"baseWidthMm":25,"baseDepthMm":25,"count":5},"player2":{"baseWidthMm":25,"baseDepthMm":25,"count":5}}`
	res := request(t, srv, http.MethodPost, "/api/games", createBody)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	g := created.Game
	if g.Phase != "setup" {
		t.Fatalf("created game should start setup, got %q", g.Phase)
	}
	for g.Phase == "setup" {
		unit := firstUnplacedUnitForPlayer(g, g.ActivePlayer)
		x, y := placementPoint(unit.PlayerID)
		res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/placements", `{"playerId":`+itoa(unit.PlayerID)+`,"unitId":"`+unit.ID+`","x":`+itoa(x)+`,"y":`+itoa(y)+`}`)
		if res.Code != http.StatusOK {
			t.Fatalf("placement status %d: %s", res.Code, res.Body.String())
		}
		var placed game.APIResponse
		if err := json.Unmarshal(res.Body.Bytes(), &placed); err != nil {
			t.Fatal(err)
		}
		g = placed.Game
	}
	unit := firstUnitForPlayer(g, g.ActivePlayer)

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/activate", `{"playerId":`+itoa(g.ActivePlayer)+`,"unitId":"`+unit.ID+`"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("activate status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":`+itoa(unit.PlayerID)+`,"unitId":"`+unit.ID+`","type":"move","direction":"forward","distanceMm":20}`)
	if res.Code != http.StatusOK {
		t.Fatalf("action status %d: %s", res.Code, res.Body.String())
	}
	var moved game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &moved); err != nil {
		t.Fatal(err)
	}
	movedUnit := unitByID(moved.Game, unit.ID)
	if movedUnit.X == unit.X && movedUnit.Y == unit.Y {
		t.Fatal("move response did not change unit position")
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":`+itoa(moved.Action.Index)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind status %d: %s", res.Code, res.Body.String())
	}
	var rewound game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewound); err != nil {
		t.Fatal(err)
	}
	rewoundUnit := unitByID(rewound.Game, unit.ID)
	if rewoundUnit.X != unit.X || rewoundUnit.Y != unit.Y {
		t.Fatalf("rewind did not restore unit position: got (%v,%v), want (%v,%v)", rewoundUnit.X, rewoundUnit.Y, unit.X, unit.Y)
	}
}

func TestListGamesReturnsSummaries(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	createBody := `{"battlemapId":"forest_wall","player1Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}],"player2Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}]}`
	res := request(t, srv, http.MethodPost, "/api/games", createBody)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	res = request(t, srv, http.MethodGet, "/api/games", "")
	if res.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", res.Code, res.Body.String())
	}
	var listed game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Games) != 1 {
		t.Fatalf("listed %d games, want 1", len(listed.Games))
	}
	summary := listed.Games[0]
	if summary.ID != created.Game.ID {
		t.Fatalf("summary ID %q, want %q", summary.ID, created.Game.ID)
	}
	if summary.BattlemapID != "forest_wall" || summary.Battlemap == "" {
		t.Fatalf("summary battlemap = (%q, %q), want forest_wall with name", summary.BattlemapID, summary.Battlemap)
	}
	if summary.ActionCount != 0 {
		t.Fatalf("summary action count %d, want 0", summary.ActionCount)
	}
	if summary.SnapshotCount != 1 {
		t.Fatalf("summary snapshot count %d, want 1", summary.SnapshotCount)
	}
	if summary.UpdatedAt == "" {
		t.Fatal("summary missing updated timestamp")
	}

	unit := firstUnplacedUnitForPlayer(created.Game, created.Game.ActivePlayer)
	x, y := placementPoint(unit.PlayerID)
	res = request(t, srv, http.MethodPost, "/api/games/"+created.Game.ID+"/placements", `{"playerId":`+itoa(unit.PlayerID)+`,"unitId":"`+unit.ID+`","x":`+itoa(x)+`,"y":`+itoa(y)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("placement status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodGet, "/api/games", "")
	if res.Code != http.StatusOK {
		t.Fatalf("list after placement status %d: %s", res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	summary = listed.Games[0]
	if summary.ActionCount != 1 {
		t.Fatalf("summary action count after placement %d, want 1", summary.ActionCount)
	}
	if summary.SnapshotCount != 2 {
		t.Fatalf("summary snapshot count after placement %d, want 2", summary.SnapshotCount)
	}
}

func TestCreateGameFromSavedArmies(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	units, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}
	tpl1, err := st.CreateArmyTemplate("P1 Template", 100)
	if err != nil {
		t.Fatal(err)
	}
	tpl1, err = st.AddTemplateUnit(tpl1.ID, units[0].ID, "P1 Moniker", 1)
	if err != nil {
		t.Fatal(err)
	}
	p1, err := st.CreateArmyFromTemplate(tpl1.ID, "P1 Army")
	if err != nil {
		t.Fatal(err)
	}

	tpl2, err := st.CreateArmyTemplate("P2 Template", 100)
	if err != nil {
		t.Fatal(err)
	}
	tpl2, err = st.AddTemplateUnit(tpl2.ID, units[1].ID, "P2 Moniker", 1)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := st.CreateArmyFromTemplate(tpl2.ID, "P2 Army")
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	body := `{"battlemapId":"old_road","player1ArmyId":"` + p1.ID + `","player2ArmyId":"` + p2.ID + `"}`
	res := request(t, srv, http.MethodPost, "/api/games", body)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Game.Units) != 2 {
		t.Fatalf("game units = %d, want 2", len(created.Game.Units))
	}
	p1Unit := firstUnitForPlayer(created.Game, 1)
	if p1Unit.Name != "P1 Moniker" || p1Unit.ArmyID != p1.ID || p1Unit.ArmyUnitID != p1.Units[0].ID || p1Unit.CatalogUnitID != units[0].ID {
		t.Fatalf("player 1 unit missing roster identity: %#v", p1Unit)
	}
	if p1Unit.MaxHealth != units[0].H || p1Unit.CurrentHealth != units[0].H || p1Unit.Stats.Pts != units[0].Pts {
		t.Fatalf("player 1 unit missing health/stats: %#v", p1Unit)
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func firstUnitForPlayer(g *game.Game, playerID int) game.Unit {
	for _, unit := range g.Units {
		if unit.PlayerID == playerID {
			return unit
		}
	}
	return game.Unit{}
}

func unitByID(g *game.Game, id string) game.Unit {
	for _, unit := range g.Units {
		if unit.ID == id {
			return unit
		}
	}
	return game.Unit{}
}

func firstUnplacedUnitForPlayer(g *game.Game, playerID int) game.Unit {
	for _, unit := range g.Units {
		if unit.PlayerID == playerID && !unit.Placed {
			return unit
		}
	}
	return game.Unit{}
}

func placementPoint(playerID int) (int, int) {
	if playerID == 1 {
		return 120, 120
	}
	return 620, 400
}
