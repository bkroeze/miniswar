package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"miniswar/internal/game"
	"miniswar/internal/store"
)

func TestHealthcheckIncludesUptime(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodGet, "/healthcheck", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	var got struct {
		OK            bool    `json:"ok"`
		Status        string  `json:"status"`
		Uptime        string  `json:"uptime"`
		UptimeSeconds float64 `json:"uptimeSeconds"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Status != "ok" {
		t.Fatalf("healthcheck status = %#v, want ok", got)
	}
	if got.Uptime == "" || got.UptimeSeconds < 0 {
		t.Fatalf("healthcheck uptime = %#v, want populated non-negative uptime", got)
	}
}

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

func TestCombatHTTPFlowPersistsPendingChoiceAndRewinds(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv, g := createCombatGameWithFirstPlayer(t, st, 1)
	placeCombatUnits(t, srv, g)
	g = getGame(t, srv, g.ID)
	unit := firstUnitForPlayer(g, 1)

	res := request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/activate", `{"playerId":1,"unitId":"`+unit.ID+`"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("activate status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"`+unit.ID+`","type":"move","direction":"forward","distanceMm":40}`)
	if res.Code != http.StatusOK {
		t.Fatalf("combat move status %d: %s", res.Code, res.Body.String())
	}
	var moved game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &moved); err != nil {
		t.Fatal(err)
	}
	if moved.Game.PendingCombatChoice == nil {
		t.Fatalf("move into combat should create pending choice: %#v", moved.Game)
	}
	if len(moved.LegalActions) != 1 || moved.LegalActions[0] != game.ActionCombatPushback {
		t.Fatalf("legal actions got %v, want only combat pushback", moved.LegalActions)
	}
	actionResult := moved.Action.Result.(map[string]any)
	if _, ok := actionResult["combatRound"].(map[string]any); !ok {
		t.Fatalf("combat result missing from action JSON: %#v", actionResult)
	}
	if rounds, ok := actionResult["combatRounds"].([]any); !ok || len(rounds) != 1 {
		t.Fatalf("canonical combatRounds missing from action JSON: %#v", actionResult)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"`+unit.ID+`","type":"combat_pushback","combatChoice":"sideways"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("invalid pushback status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"u2","type":"combat_pushback","combatChoice":"decline"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("mismatched pushback unit status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"`+unit.ID+`","type":"combat_pushback","combatChoice":"decline"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("decline pushback status %d: %s", res.Code, res.Body.String())
	}
	var declined game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &declined); err != nil {
		t.Fatal(err)
	}
	if declined.Game.PendingCombatChoice != nil {
		t.Fatalf("decline should clear pending choice: %#v", declined.Game.PendingCombatChoice)
	}
	if len(declined.Game.Engagements) != 1 || declined.Game.Engagements[0].Active {
		t.Fatalf("decline should deactivate engagement: %#v", declined.Game.Engagements)
	}
	reloaded := getGame(t, srv, g.ID)
	if reloaded.PendingCombatChoice != nil {
		t.Fatalf("reloaded game should persist cleared pending choice: %#v", reloaded.PendingCombatChoice)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":`+itoa(moved.Action.Index)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind status %d: %s", res.Code, res.Body.String())
	}
	var rewound game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewound); err != nil {
		t.Fatal(err)
	}
	if rewound.Game.PendingCombatChoice != nil || len(rewound.Game.Engagements) != 0 {
		t.Fatalf("rewind before combat should clear combat state: pending=%#v engagements=%#v", rewound.Game.PendingCombatChoice, rewound.Game.Engagements)
	}
	if rewound.Game.RandomRollIndex >= moved.Game.RandomRollIndex {
		t.Fatalf("rewind should restore earlier random progress: got %d, combat had %d", rewound.Game.RandomRollIndex, moved.Game.RandomRollIndex)
	}
	afterRewind := getGame(t, srv, g.ID)
	for _, snapshot := range afterRewind.Snapshots {
		if snapshot.Index > moved.Action.Index {
			t.Fatalf("rewind should prune future snapshots, kept %+v after rewinding to %d", snapshot, moved.Action.Index)
		}
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

func TestGetGameStepReturnsSnapshotWithoutRewindingCurrentGame(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	createBody := `{"battlemapId":"old_road","player1Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}],"player2Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}]}`
	res := request(t, srv, http.MethodPost, "/api/games", createBody)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	unit := firstUnplacedUnitForPlayer(created.Game, created.Game.ActivePlayer)
	x, y := placementPoint(unit.PlayerID)
	res = request(t, srv, http.MethodPost, "/api/games/"+created.Game.ID+"/placements", `{"playerId":`+itoa(unit.PlayerID)+`,"unitId":"`+unit.ID+`","x":`+itoa(x)+`,"y":`+itoa(y)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("placement status %d: %s", res.Code, res.Body.String())
	}
	var placed game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &placed); err != nil {
		t.Fatal(err)
	}
	if len(placed.Game.ActionHistory) != 1 {
		t.Fatalf("placed action history len = %d, want 1", len(placed.Game.ActionHistory))
	}

	res = request(t, srv, http.MethodGet, "/api/games/"+created.Game.ID+"/steps/0", "")
	if res.Code != http.StatusOK {
		t.Fatalf("step 0 status %d: %s", res.Code, res.Body.String())
	}
	var step0 game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &step0); err != nil {
		t.Fatal(err)
	}
	if len(step0.Game.ActionHistory) != 0 {
		t.Fatalf("step 0 action history len = %d, want 0", len(step0.Game.ActionHistory))
	}
	if unitByID(step0.Game, unit.ID).Placed {
		t.Fatalf("step 0 should show %s before placement", unit.ID)
	}

	current := getGame(t, srv, created.Game.ID)
	if len(current.ActionHistory) != 1 || !unitByID(current, unit.ID).Placed {
		t.Fatalf("reading step 0 mutated current game: actions=%d unit=%#v", len(current.ActionHistory), unitByID(current, unit.ID))
	}

	res = request(t, srv, http.MethodGet, "/api/games/"+created.Game.ID+"/steps/1", "")
	if res.Code != http.StatusOK {
		t.Fatalf("step 1 status %d: %s", res.Code, res.Body.String())
	}
	var step1 game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &step1); err != nil {
		t.Fatal(err)
	}
	if len(step1.Game.ActionHistory) != 1 || !unitByID(step1.Game, unit.ID).Placed {
		t.Fatalf("step 1 should match current game after one action: actions=%d unit=%#v", len(step1.Game.ActionHistory), unitByID(step1.Game, unit.ID))
	}

	res = request(t, srv, http.MethodGet, "/games/"+created.Game.ID+"/steps/1", "")
	if res.Code != http.StatusOK {
		t.Fatalf("game step page status %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Miniswar") {
		t.Fatalf("game step page did not serve app shell: %s", res.Body.String())
	}
}

func TestBattlemapHTTPCRUDAndValidation(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodGet, "/api/battlemaps", "")
	if res.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", res.Code, res.Body.String())
	}
	var listed struct {
		OK         bool             `json:"ok"`
		Battlemaps []game.Battlemap `json:"battlemaps"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Battlemaps) < 2 {
		t.Fatalf("listed battlemaps = %d, want seeded maps", len(listed.Battlemaps))
	}

	body := `{"name":"Big Field","widthMm":1200,"heightMm":800,"terrains":[{"id":"rough-1","type":"rough","label":"rough","shape":"rect","x":100,"y":100,"width":200,"height":100}]}`
	res = request(t, srv, http.MethodPost, "/api/battlemaps", body)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created struct {
		OK        bool           `json:"ok"`
		Battlemap game.Battlemap `json:"battlemap"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Battlemap.ID == "" || created.Battlemap.WidthMM != 1200 || len(created.Battlemap.Terrains) != 1 {
		t.Fatalf("created battlemap = %#v", created.Battlemap)
	}

	updateBody := `{"name":"Bigger Field","widthMm":1400,"heightMm":900,"terrains":[{"id":"wall-1","type":"impassable","label":"wall","shape":"rect","x":300,"y":100,"width":40,"height":300}]}`
	res = request(t, srv, http.MethodPatch, "/api/battlemaps/"+created.Battlemap.ID, updateBody)
	if res.Code != http.StatusOK {
		t.Fatalf("update status %d: %s", res.Code, res.Body.String())
	}
	var updated struct {
		Battlemap game.Battlemap `json:"battlemap"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Battlemap.Name != "Bigger Field" || updated.Battlemap.Terrains[0].Type != game.TerrainImpassable {
		t.Fatalf("updated battlemap = %#v", updated.Battlemap)
	}

	res = request(t, srv, http.MethodPatch, "/api/battlemaps/"+created.Battlemap.ID, `{"name":"Renamed Field"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("partial update status %d: %s", res.Code, res.Body.String())
	}
	var partiallyUpdated struct {
		Battlemap game.Battlemap `json:"battlemap"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &partiallyUpdated); err != nil {
		t.Fatal(err)
	}
	if partiallyUpdated.Battlemap.Name != "Renamed Field" || partiallyUpdated.Battlemap.WidthMM != 1400 || partiallyUpdated.Battlemap.HeightMM != 900 || len(partiallyUpdated.Battlemap.Terrains) != 1 {
		t.Fatalf("partial update reset battlemap geometry: %#v", partiallyUpdated.Battlemap)
	}

	res = request(t, srv, http.MethodPost, "/api/battlemaps", `{"name":"Bad","widthMm":100,"heightMm":100,"terrains":[{"id":"bad","type":"rough","shape":"rect","x":90,"y":90,"width":20,"height":20}]}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodDelete, "/api/battlemaps/"+created.Battlemap.ID, "")
	if res.Code != http.StatusOK {
		t.Fatalf("delete status %d: %s", res.Code, res.Body.String())
	}
	res = request(t, srv, http.MethodGet, "/api/battlemaps/"+created.Battlemap.ID, "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("get deleted status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodDelete, "/api/battlemaps/old_road", "")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("delete built-in status %d: %s", res.Code, res.Body.String())
	}
}

func TestCreateGameCopiesSavedBattlemap(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	battlemap, err := st.CreateBattlemap(game.Battlemap{
		Name:     "Wide Field",
		WidthMM:  1200,
		HeightMM: 520,
		Terrains: []game.TerrainZone{{
			ID:     "rough-1",
			Type:   game.TerrainRough,
			Label:  "rough",
			Shape:  "rect",
			X:      100,
			Y:      100,
			Width:  100,
			Height: 100,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	createBody := `{"battlemapId":"` + battlemap.ID + `","player1Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}],"player2Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}]}`
	res := request(t, srv, http.MethodPost, "/api/games", createBody)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Game.Battlemap.ID != battlemap.ID || created.Game.Battlemap.WidthMM != 1200 || len(created.Game.Battlemap.Terrains) != 1 {
		t.Fatalf("game copied battlemap = %#v", created.Game.Battlemap)
	}

	if _, err := st.UpdateBattlemap(battlemap.ID, game.Battlemap{Name: "Changed", WidthMM: 1400, HeightMM: 520}); err != nil {
		t.Fatal(err)
	}
	reloaded := getGame(t, srv, created.Game.ID)
	if reloaded.Battlemap.Name != "Wide Field" || reloaded.Battlemap.WidthMM != 1200 || len(reloaded.Battlemap.Terrains) != 1 {
		t.Fatalf("game battlemap mutated after library edit: %#v", reloaded.Battlemap)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+created.Game.ID+"/rewind", `{"actionIndex":-1}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind status %d: %s", res.Code, res.Body.String())
	}
	var rewound game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewound); err != nil {
		t.Fatal(err)
	}
	if rewound.Game.Battlemap.Name != "Wide Field" || rewound.Game.Battlemap.WidthMM != 1200 {
		t.Fatalf("rewind did not restore copied battlemap: %#v", rewound.Game.Battlemap)
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

func TestCreateGameFromSavedArmiesReportsBadRosterReferences(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	emptyArmy, err := st.CreateArmy("Empty Army", 100)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	assertJSONError(t, request(t, srv, http.MethodPost, "/api/games", `{"player1ArmyId":"missing-army","player2":{"baseWidthMm":25,"baseDepthMm":25,"count":5}}`), http.StatusBadRequest, `army "missing-army" not found`)
	assertJSONError(t, request(t, srv, http.MethodPost, "/api/games", `{"player1ArmyId":"`+emptyArmy.ID+`","player2":{"baseWidthMm":25,"baseDepthMm":25,"count":5}}`), http.StatusBadRequest, `army "`+emptyArmy.ID+`" has no units`)
}

func TestPatchArmyTemplateMetadataPreservesOmittedFields(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tpl, err := st.CreateArmyTemplate("Template", 100)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodPatch, "/api/army-templates/"+tpl.ID, `{"name":"Skirmish"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", res.Code, res.Body.String())
	}
	var patched struct {
		Template store.ArmyTemplate `json:"template"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Template.Name != "Skirmish" || patched.Template.TargetPoints != 100 {
		t.Fatalf("patched template = %#v, want changed name and preserved target points", patched.Template)
	}

	res = request(t, srv, http.MethodPatch, "/api/army-templates/"+tpl.ID, `{"targetPoints":75}`)
	if res.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Template.Name != "Skirmish" || patched.Template.TargetPoints != 75 {
		t.Fatalf("patched template = %#v, want preserved name and changed target points", patched.Template)
	}
}

func TestPatchArmyMetadataPreservesOmittedFields(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	army, err := st.CreateArmy("Army", 100)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodPatch, "/api/armies/"+army.ID, `{"name":"Skirmish"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", res.Code, res.Body.String())
	}
	var patched struct {
		Army store.Army `json:"army"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Army.Name != "Skirmish" || patched.Army.TargetPoints != 100 {
		t.Fatalf("patched army = %#v, want changed name and preserved target points", patched.Army)
	}

	res = request(t, srv, http.MethodPatch, "/api/armies/"+army.ID, `{"targetPoints":75}`)
	if res.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Army.Name != "Skirmish" || patched.Army.TargetPoints != 75 {
		t.Fatalf("patched army = %#v, want preserved name and changed target points", patched.Army)
	}
}

func TestPatchTemplateUnitPreservesOmittedFields(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	units, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := st.CreateArmyTemplate("Template", 100)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = st.AddTemplateUnit(tpl.ID, units[0].ID, "Old Name", 3)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodPatch, "/api/army-templates/"+tpl.ID+"/units/"+tpl.Units[0].ID, `{"moniker":"New Name"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", res.Code, res.Body.String())
	}
	var patched struct {
		Template store.ArmyTemplate `json:"template"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	line := patched.Template.Units[0]
	if line.DefaultMoniker != "New Name" || line.MiniCount != 3 {
		t.Fatalf("patched line = %#v, want changed moniker and preserved mini count", line)
	}
}

func TestPatchArmyUnitPreservesOmittedFields(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	units, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}
	army, err := st.CreateArmy("Army", 100)
	if err != nil {
		t.Fatal(err)
	}
	army, err = st.AddArmyUnit(army.ID, units[0].ID, "Old Name", 3)
	if err != nil {
		t.Fatal(err)
	}
	army, err = st.UpdateArmyUnit(army.ID, army.Units[0].ID, "Old Name", 3, 2)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodPatch, "/api/armies/"+army.ID+"/units/"+army.Units[0].ID, `{"moniker":"New Name"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", res.Code, res.Body.String())
	}
	var patched struct {
		Army store.Army `json:"army"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	line := patched.Army.Units[0]
	if line.Moniker != "New Name" || line.MiniCount != 3 || line.CurrentHealth != 2 {
		t.Fatalf("patched line = %#v, want changed moniker and preserved count/health", line)
	}
}

func TestArmyAPIMissingReferencesReturnClientErrors(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	units, err := st.CatalogUnits("Dwarf", "")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := st.CreateArmyTemplate("Template", 100)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = st.AddTemplateUnit(tpl.ID, units[0].ID, "Line", 1)
	if err != nil {
		t.Fatal(err)
	}
	army, err := st.CreateArmy("Army", 100)
	if err != nil {
		t.Fatal(err)
	}
	army, err = st.AddArmyUnit(army.ID, units[0].ID, "Line", 1)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, game.NewEngine(1)).Routes()
	assertJSONError(t, request(t, srv, http.MethodGet, "/api/army-templates/missing-template", ""), http.StatusNotFound, `army template "missing-template" not found`)
	assertJSONError(t, request(t, srv, http.MethodPost, "/api/army-templates/missing-template/units", `{"catalogUnitId":"`+units[0].ID+`","miniCount":1}`), http.StatusNotFound, `army template "missing-template" not found`)
	assertJSONError(t, request(t, srv, http.MethodPost, "/api/army-templates/"+tpl.ID+"/units", `{"catalogUnitId":"missing-catalog","miniCount":1}`), http.StatusBadRequest, `catalog unit "missing-catalog" not found`)
	assertJSONError(t, request(t, srv, http.MethodPatch, "/api/army-templates/"+tpl.ID+"/units/missing-unit", `{"moniker":"New"}`), http.StatusNotFound, `army template unit "missing-unit" not found`)
	assertJSONError(t, request(t, srv, http.MethodPost, "/api/armies/from-template", `{"templateId":"missing-template","name":"Army"}`), http.StatusBadRequest, `army template "missing-template" not found`)
	assertJSONError(t, request(t, srv, http.MethodGet, "/api/armies/missing-army", ""), http.StatusNotFound, `army "missing-army" not found`)
	assertJSONError(t, request(t, srv, http.MethodPost, "/api/armies/missing-army/units", `{"catalogUnitId":"`+units[0].ID+`","miniCount":1}`), http.StatusNotFound, `army "missing-army" not found`)
	assertJSONError(t, request(t, srv, http.MethodPost, "/api/armies/"+army.ID+"/units", `{"catalogUnitId":"missing-catalog","miniCount":1}`), http.StatusBadRequest, `catalog unit "missing-catalog" not found`)
	assertJSONError(t, request(t, srv, http.MethodPatch, "/api/armies/"+army.ID+"/units/missing-unit", `{"moniker":"New"}`), http.StatusNotFound, `army unit "missing-unit" not found`)
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func assertJSONError(t *testing.T, res *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	var got game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.OK {
		t.Fatalf("response ok = true, want false: %#v", got)
	}
	if len(got.Errors) != 1 || got.Errors[0] != message {
		t.Fatalf("errors = %#v, want [%q]", got.Errors, message)
	}
}

func createCombatGameWithFirstPlayer(t *testing.T, st *store.Store, playerID int) (http.Handler, *game.Game) {
	t.Helper()
	body := `{"player1":{"baseWidthMm":25,"baseDepthMm":25,"count":1,"stats":{"a":20,"d":20,"cd":1,"h":20}},"player2":{"baseWidthMm":25,"baseDepthMm":25,"count":1,"stats":{"a":1,"d":20,"cd":1,"h":20}}}`
	for seed := int64(1); seed < 100; seed++ {
		srv := New(st, game.NewEngine(seed)).Routes()
		res := request(t, srv, http.MethodPost, "/api/games", body)
		if res.Code != http.StatusCreated {
			t.Fatalf("create status %d: %s", res.Code, res.Body.String())
		}
		var created game.APIResponse
		if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		if created.Game.ActivePlayer == playerID {
			return srv, created.Game
		}
	}
	t.Fatalf("could not create game with first player %d", playerID)
	return nil, nil
}

func placeCombatUnits(t *testing.T, handler http.Handler, g *game.Game) {
	t.Helper()
	for g.Phase == "setup" {
		unit := firstUnplacedUnitForPlayer(g, g.ActivePlayer)
		x, y, facing := 112, 112, 0
		if unit.PlayerID == 2 {
			x, y, facing = 112, 62, 180
		}
		res := request(t, handler, http.MethodPost, "/api/games/"+g.ID+"/placements", `{"playerId":`+itoa(unit.PlayerID)+`,"unitId":"`+unit.ID+`","x":`+itoa(x)+`,"y":`+itoa(y)+`,"facingDeg":`+itoa(facing)+`}`)
		if res.Code != http.StatusOK {
			t.Fatalf("placement status %d: %s", res.Code, res.Body.String())
		}
		var placed game.APIResponse
		if err := json.Unmarshal(res.Body.Bytes(), &placed); err != nil {
			t.Fatal(err)
		}
		g = placed.Game
	}
}

func getGame(t *testing.T, handler http.Handler, gameID string) *game.Game {
	t.Helper()
	res := request(t, handler, http.MethodGet, "/api/games/"+gameID, "")
	if res.Code != http.StatusOK {
		t.Fatalf("get game status %d: %s", res.Code, res.Body.String())
	}
	var response game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Game
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
