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
	"miniswar/internal/version"
)

func TestIndexIncludesVersionAndCopyright(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodGet, "/", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"Version " + version.Display(), "Copyright (c) 2026 Bruce Kroeze"} {
		if !strings.Contains(body, want) {
			t.Fatalf("index missing %q in body:\n%s", want, body)
		}
	}
}

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

func TestCreateGameRejectsInvalidSetupWithoutPersisting(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	cases := []struct {
		name string
		body string
		err  string
	}{
		{
			name: "zero mini unit",
			body: `{"battlemapId":"old_road","player1Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":0}],"player2Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}]}`,
			err:  "player1 unit 1: count must be between 1 and 20",
		},
		{
			name: "large base with multiple minis",
			body: `{"battlemapId":"old_road","player1Units":[{"baseWidthMm":50,"baseDepthMm":100,"count":2}],"player2Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}]}`,
			err:  "player1 unit 1: count must be between 1 and 1",
		},
		{
			name: "unknown battlemap",
			body: `{"battlemapId":"missing-map","player1Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}],"player2Units":[{"baseWidthMm":25,"baseDepthMm":25,"count":5}]}`,
			err:  `battlemap "missing-map" not found`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertJSONError(t, request(t, srv, http.MethodPost, "/api/games", tt.body), http.StatusBadRequest, tt.err)

			res := request(t, srv, http.MethodGet, "/api/games", "")
			if res.Code != http.StatusOK {
				t.Fatalf("list status %d: %s", res.Code, res.Body.String())
			}
			var listed game.APIResponse
			if err := json.Unmarshal(res.Body.Bytes(), &listed); err != nil {
				t.Fatal(err)
			}
			if len(listed.Games) != 0 {
				t.Fatalf("rejected setup persisted %d game(s), want 0", len(listed.Games))
			}
		})
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

func TestHTTPPivotAndAboutFaceCanRewind(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	engine := game.NewEngine(1)
	g, err := engine.NewGame(game.Setup{
		Player1: game.UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 7},
		Player2: game.UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Phase = "activated"
	g.ActivePlayer = 1
	g.FirstPlayer = 1
	g.Units[0].Placed = true
	g.Units[0].X = 150
	g.Units[0].Y = 150
	g.Units[0].FacingDeg = 0
	g.Units[1].Placed = true
	g.Units[1].X = 600
	g.Units[1].Y = 400
	g.Units[1].FacingDeg = 180
	g.CurrentActivation = &game.Activation{UnitID: g.Units[0].ID, PlayerID: 1, Success: true, ActionsRemaining: 2}
	anchorKey := g.Units[0].Minis[0].Key
	if err := st.SaveGame(g); err != nil {
		t.Fatal(err)
	}

	srv := New(st, engine).Routes()
	res := request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"`+g.Units[0].ID+`","type":"pivot","facingDeg":90,"anchorKey":"`+anchorKey+`"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("pivot status %d: %s", res.Code, res.Body.String())
	}
	var pivoted game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &pivoted); err != nil {
		t.Fatal(err)
	}
	if unitByID(pivoted.Game, g.Units[0].ID).FacingDeg != 90 {
		t.Fatalf("pivot facing = %d, want 90", unitByID(pivoted.Game, g.Units[0].ID).FacingDeg)
	}
	if pivoted.Game.CurrentActivation == nil || pivoted.Game.CurrentActivation.ActionsRemaining != 1 {
		t.Fatalf("pivot activation = %#v, want one action remaining", pivoted.Game.CurrentActivation)
	}
	if !strings.Contains(strings.Join(pivoted.Messages, "\n"), "Pivoted to 90 degrees around "+anchorKey+".") {
		t.Fatalf("pivot messages = %v", pivoted.Messages)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"`+g.Units[0].ID+`","type":"about_face"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("about face status %d: %s", res.Code, res.Body.String())
	}
	var faced game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &faced); err != nil {
		t.Fatal(err)
	}
	if unitByID(faced.Game, g.Units[0].ID).FacingDeg != 270 {
		t.Fatalf("about face facing = %d, want 270", unitByID(faced.Game, g.Units[0].ID).FacingDeg)
	}
	if faced.Game.CurrentActivation != nil || faced.Game.Phase != "awaiting_activation" {
		t.Fatalf("about face should end activation: phase=%q activation=%#v", faced.Game.Phase, faced.Game.CurrentActivation)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":`+itoa(faced.Action.Index)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind about face status %d: %s", res.Code, res.Body.String())
	}
	var rewoundAboutFace game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewoundAboutFace); err != nil {
		t.Fatal(err)
	}
	if unitByID(rewoundAboutFace.Game, g.Units[0].ID).FacingDeg != 90 || rewoundAboutFace.Game.CurrentActivation == nil || rewoundAboutFace.Game.CurrentActivation.ActionsRemaining != 1 {
		t.Fatalf("rewind about face got facing=%d activation=%#v, want pivoted active state", unitByID(rewoundAboutFace.Game, g.Units[0].ID).FacingDeg, rewoundAboutFace.Game.CurrentActivation)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":`+itoa(pivoted.Action.Index)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind pivot status %d: %s", res.Code, res.Body.String())
	}
	var rewoundPivot game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewoundPivot); err != nil {
		t.Fatal(err)
	}
	if unitByID(rewoundPivot.Game, g.Units[0].ID).FacingDeg != 0 || rewoundPivot.Game.CurrentActivation == nil || rewoundPivot.Game.CurrentActivation.ActionsRemaining != 2 {
		t.Fatalf("rewind pivot got facing=%d activation=%#v, want original active state", unitByID(rewoundPivot.Game, g.Units[0].ID).FacingDeg, rewoundPivot.Game.CurrentActivation)
	}
}

func TestActionsEndpointAndInvalidRewind(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	res := request(t, srv, http.MethodPost, "/api/games", `{"player1":{"baseWidthMm":25,"baseDepthMm":25,"count":5},"player2":{"baseWidthMm":25,"baseDepthMm":25,"count":5}}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	g := created.Game
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

	res = request(t, srv, http.MethodGet, "/api/games/"+g.ID+"/actions", "")
	if res.Code != http.StatusOK {
		t.Fatalf("actions status %d: %s", res.Code, res.Body.String())
	}
	var timeline struct {
		OK      bool                `json:"ok"`
		Actions []game.ActionRecord `json:"actions"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &timeline); err != nil {
		t.Fatal(err)
	}
	if !timeline.OK || len(timeline.Actions) != 1 || timeline.Actions[0].Type != game.ActionPlace || timeline.Actions[0].UnitID != unit.ID {
		t.Fatalf("timeline = %#v, want one placement action for %s", timeline, unit.ID)
	}

	assertJSONError(t, request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":99}`), http.StatusBadRequest, "game snapshot not found")

	current := getGame(t, srv, g.ID)
	currentUnit := unitByID(current, unit.ID)
	placedUnit := unitByID(placed.Game, unit.ID)
	if len(current.ActionHistory) != 1 || !currentUnit.Placed || currentUnit.X != placedUnit.X || currentUnit.Y != placedUnit.Y {
		t.Fatalf("invalid rewind mutated current game: actions=%d unit=%#v", len(current.ActionHistory), currentUnit)
	}
}

func TestGetGameIncludesShootLegalActionDetails(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	engine := game.NewEngine(1)
	g, err := engine.NewGame(game.Setup{
		Player1: game.UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, Stats: game.UnitStats{A: 6, D: 8, CD: 1, H: 1}, Equipment: []string{"Bow"}},
		Player2: game.UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, Stats: game.UnitStats{A: 5, D: 8, CD: 1, H: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Phase = "activated"
	g.ActivePlayer = 1
	g.CurrentActivation = &game.Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2}
	g.Units[0].Placed = true
	g.Units[0].X = 100
	g.Units[0].Y = 300
	g.Units[0].FacingDeg = 0
	g.Units[1].Placed = true
	g.Units[1].X = 100
	g.Units[1].Y = 100
	g.Units[1].FacingDeg = 180
	if err := st.SaveGame(g); err != nil {
		t.Fatal(err)
	}

	srv := New(st, engine).Routes()
	res := request(t, srv, http.MethodGet, "/api/games/"+g.ID, "")
	if res.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", res.Code, res.Body.String())
	}
	var got game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !containsLegalAction(got.LegalActions, game.ActionShoot) {
		t.Fatalf("legal actions = %v, want shoot", got.LegalActions)
	}
	var shoot game.LegalAction
	for _, detail := range got.LegalActionDetails {
		if detail.Type == game.ActionShoot {
			shoot = detail
		}
	}
	if len(shoot.Targets) != 1 || shoot.Targets[0].UnitID != "u2" || shoot.Targets[0].Weapon != "Bow" {
		t.Fatalf("shoot details = %+v, want one bow target", shoot)
	}
}

func TestHTTPShootActionPersistsFeedbackRejectsRepeatAndRewinds(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	engine := game.NewEngine(3)
	g, err := engine.NewGame(game.Setup{
		Player1: game.UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, Stats: game.UnitStats{A: 20, D: 8, CD: 1, H: 1}, Equipment: []string{"Bow"}},
		Player2: game.UnitSetup{BaseWidthMM: 25, BaseDepthMM: 25, Count: 5, MaxHealth: 3, CurrentHealth: 3, CurrentHealthSet: true, Stats: game.UnitStats{A: 11, D: 1, CD: 1, H: 3}, Special: []string{"Shielding (1)"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Phase = "activated"
	g.ActivePlayer = 1
	g.CurrentActivation = &game.Activation{UnitID: "u1", PlayerID: 1, Success: true, ActionsRemaining: 2}
	g.Units[0].Placed = true
	g.Units[0].X = 100
	g.Units[0].Y = 300
	g.Units[0].FacingDeg = 0
	g.Units[1].Placed = true
	g.Units[1].X = 100
	g.Units[1].Y = 100
	g.Units[1].FacingDeg = 180
	beforeTargetMiniHealth := unitByID(g, "u2").Minis[0].HealthRemaining
	if err := st.SaveGame(g); err != nil {
		t.Fatal(err)
	}

	srv := New(st, engine).Routes()
	res := request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"u1","type":"shoot","targetUnitId":"u2"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("shoot status %d: %s", res.Code, res.Body.String())
	}
	var shot game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &shot); err != nil {
		t.Fatal(err)
	}
	if shot.Action == nil || shot.Action.Type != game.ActionShoot {
		t.Fatalf("action = %#v, want shoot record", shot.Action)
	}
	actionResult := shot.Action.Result.(map[string]any)
	shootingResult, ok := actionResult["shooting"].(map[string]any)
	if !ok {
		t.Fatalf("shooting result missing from action JSON: %#v", actionResult)
	}
	if shootingResult["targetUnitId"] != "u2" || shootingResult["weapon"] != "Bow" || shootingResult["diceCount"].(float64) != 4 {
		t.Fatalf("shooting result = %#v, want u2 bow with shielding-reduced dice", shootingResult)
	}
	if shot.Game.CurrentActivation == nil || shot.Game.CurrentActivation.ActionsRemaining != 1 || shot.Game.CurrentActivation.ShotsTaken != 1 {
		t.Fatalf("activation after shot = %#v, want one action remaining and one shot taken", shot.Game.CurrentActivation)
	}
	if containsLegalAction(shot.LegalActions, game.ActionShoot) {
		t.Fatalf("legal actions after shot = %v, want no repeat shoot action", shot.LegalActions)
	}
	if unitByID(shot.Game, "u2").Minis[0].HealthRemaining >= beforeTargetMiniHealth {
		t.Fatalf("target mini health = %d, want less than %d", unitByID(shot.Game, "u2").Minis[0].HealthRemaining, beforeTargetMiniHealth)
	}
	if !strings.Contains(strings.Join(shot.Messages, "\n"), "Shot u2 with Bow") {
		t.Fatalf("shoot response missing feedback message: %v", shot.Messages)
	}

	assertJSONError(t, request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"u1","type":"shoot","targetUnitId":"u2"}`), http.StatusBadRequest, "unit has already shot this activation")
	afterRejectedRepeat := getGame(t, srv, g.ID)
	if len(afterRejectedRepeat.ActionHistory) != 1 || unitByID(afterRejectedRepeat, "u2").Minis[0].HealthRemaining != unitByID(shot.Game, "u2").Minis[0].HealthRemaining {
		t.Fatalf("rejected repeat shot mutated game: actions=%d target=%#v", len(afterRejectedRepeat.ActionHistory), unitByID(afterRejectedRepeat, "u2"))
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":`+itoa(shot.Action.Index)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind shot status %d: %s", res.Code, res.Body.String())
	}
	var rewound game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewound); err != nil {
		t.Fatal(err)
	}
	if len(rewound.Game.ActionHistory) != 0 || rewound.Game.CurrentActivation == nil || rewound.Game.CurrentActivation.ShotsTaken != 0 || unitByID(rewound.Game, "u2").Minis[0].HealthRemaining != beforeTargetMiniHealth {
		t.Fatalf("rewind shot got actions=%d activation=%#v target=%#v, want active pre-shot state", len(rewound.Game.ActionHistory), rewound.Game.CurrentActivation, unitByID(rewound.Game, "u2"))
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

func TestHTTPCombatCanCompleteGameAndRewind(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	engine := game.NewEngine(43)
	g, err := engine.NewGame(game.Setup{
		Player1Units: []game.UnitSetup{{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1, Stats: game.UnitStats{A: 1, D: 1, CD: 1, H: 1}}},
		Player2Units: []game.UnitSetup{{BaseWidthMM: 25, BaseDepthMM: 25, Count: 1, Stats: game.UnitStats{A: 20, D: 20, CD: 1, H: 20}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Phase = "awaiting_activation"
	g.ActivePlayer = 1
	g.FirstPlayer = 1
	g.Units[0].Placed = true
	g.Units[0].X = 100
	g.Units[0].Y = 100
	g.Units[0].FacingDeg = 0
	g.Units[1].Placed = true
	g.Units[1].X = 100
	g.Units[1].Y = 50
	g.Units[1].FacingDeg = 0
	snap, err := game.Snapshot(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveGame(g); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSnapshot(g.ID, -1, snap); err != nil {
		t.Fatal(err)
	}

	srv := New(st, engine).Routes()
	res := request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/activate", `{"playerId":1,"unitId":"u1"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("activate status %d: %s", res.Code, res.Body.String())
	}
	var activated game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &activated); err != nil {
		t.Fatal(err)
	}
	if !containsLegalAction(activated.LegalActions, game.ActionMove) {
		t.Fatalf("legal actions after activate = %v, want move", activated.LegalActions)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"u1","type":"move","direction":"forward","distanceMm":25}`)
	if res.Code != http.StatusOK {
		t.Fatalf("combat move status %d: %s", res.Code, res.Body.String())
	}
	var completed game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Game.Phase != "complete" || completed.Game.WinnerPlayerID != 2 {
		t.Fatalf("combat should complete with player 2 win: phase=%q winner=%d", completed.Game.Phase, completed.Game.WinnerPlayerID)
	}
	if len(completed.LegalActions) != 0 || len(completed.LegalActionDetails) != 0 {
		t.Fatalf("complete game legal actions = %v details=%v, want none", completed.LegalActions, completed.LegalActionDetails)
	}
	if unitByID(completed.Game, "u1").Placed || !unitByID(completed.Game, "u1").Broken {
		t.Fatalf("losing unit should be removed from the battlefield: %#v", unitByID(completed.Game, "u1"))
	}
	if !strings.Contains(strings.Join(completed.Messages, "\n"), "Player 2 wins.") {
		t.Fatalf("completion response missing winner message: %v", completed.Messages)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":1,"unitId":"u1","type":"skip"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("post-completion action status %d: %s", res.Code, res.Body.String())
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":`+itoa(completed.Action.Index)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind completed action status %d: %s", res.Code, res.Body.String())
	}
	var rewound game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewound); err != nil {
		t.Fatal(err)
	}
	if rewound.Game.Phase == "complete" || rewound.Game.WinnerPlayerID != 0 || unitByID(rewound.Game, "u1").Broken {
		t.Fatalf("rewind should restore the pre-completion activation state: phase=%q winner=%d unit=%#v", rewound.Game.Phase, rewound.Game.WinnerPlayerID, unitByID(rewound.Game, "u1"))
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
	if !step0.ReadOnly {
		t.Fatal("step 0 response should be read-only")
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
	if step1.ReadOnly {
		t.Fatal("current step response should be writable")
	}

	res = request(t, srv, http.MethodGet, "/games/"+created.Game.ID+"/steps/1", "")
	if res.Code != http.StatusOK {
		t.Fatalf("game step page status %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Miniswar") {
		t.Fatalf("game step page did not serve app shell: %s", res.Body.String())
	}
}

func TestTenUnitsPerSideCanCompleteRoundAndRewind(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := New(st, game.NewEngine(1)).Routes()
	createBody := `{"battlemapId":"old_road","player1Units":` + unitSetupsJSON(10) + `,"player2Units":` + unitSetupsJSON(10) + `}`
	res := request(t, srv, http.MethodPost, "/api/games", createBody)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	var created game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	g := created.Game
	if len(g.Units) != 20 {
		t.Fatalf("created %d units, want 20", len(g.Units))
	}

	for g.Phase == "setup" {
		unit := firstUnplacedUnitForPlayer(g, g.ActivePlayer)
		x, y := manyUnitPlacementPoint(unit.PlayerID, unit.ID)
		facing := 0
		if unit.PlayerID == 2 {
			facing = 180
		}
		res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/placements", `{"playerId":`+itoa(unit.PlayerID)+`,"unitId":"`+unit.ID+`","x":`+itoa(x)+`,"y":`+itoa(y)+`,"facingDeg":`+itoa(facing)+`}`)
		if res.Code != http.StatusOK {
			t.Fatalf("placement for %s status %d: %s", unit.ID, res.Code, res.Body.String())
		}
		var placed game.APIResponse
		if err := json.Unmarshal(res.Body.Bytes(), &placed); err != nil {
			t.Fatal(err)
		}
		g = placed.Game
	}
	if g.Phase != "awaiting_activation" || len(g.ActionHistory) != 20 {
		t.Fatalf("after setup phase=%q actions=%d, want awaiting_activation with 20 placements", g.Phase, len(g.ActionHistory))
	}

	startingPlayer := g.ActivePlayer
	var firstActivationIndex int
	activations := 0
	for g.Round == 1 {
		unit, ok := firstUnactivatedUnitForPlayerThisRound(g, g.ActivePlayer)
		if !ok {
			t.Fatalf("player %d has no unit to activate in round %d: actions=%d", g.ActivePlayer, g.Round, len(g.ActionHistory))
		}
		res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/activate", `{"playerId":`+itoa(g.ActivePlayer)+`,"unitId":"`+unit.ID+`"}`)
		if res.Code != http.StatusOK {
			t.Fatalf("activate %s status %d: %s", unit.ID, res.Code, res.Body.String())
		}
		var activated game.APIResponse
		if err := json.Unmarshal(res.Body.Bytes(), &activated); err != nil {
			t.Fatal(err)
		}
		if activations == 0 {
			firstActivationIndex = activated.Action.Index
		}
		if !containsLegalAction(activated.LegalActions, game.ActionSkip) {
			t.Fatalf("legal actions after activating %s = %v, want skip", unit.ID, activated.LegalActions)
		}
		res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/actions", `{"playerId":`+itoa(unit.PlayerID)+`,"unitId":"`+unit.ID+`","type":"skip"}`)
		if res.Code != http.StatusOK {
			t.Fatalf("skip %s status %d: %s", unit.ID, res.Code, res.Body.String())
		}
		var skipped game.APIResponse
		if err := json.Unmarshal(res.Body.Bytes(), &skipped); err != nil {
			t.Fatal(err)
		}
		g = skipped.Game
		activations++
		if activations > 20 {
			t.Fatalf("round 1 did not complete after 20 activations: round=%d active=%d actions=%d", g.Round, g.ActivePlayer, len(g.ActionHistory))
		}
	}
	if activations != 20 || g.Round != 2 || g.Phase != "awaiting_activation" || g.ActivePlayer != startingPlayer {
		t.Fatalf("after round 1 activations=%d round=%d phase=%q active=%d, want 20 round 2 awaiting player %d", activations, g.Round, g.Phase, g.ActivePlayer, startingPlayer)
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":`+itoa(firstActivationIndex)+`}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind status %d: %s", res.Code, res.Body.String())
	}
	var rewound game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewound); err != nil {
		t.Fatal(err)
	}
	if rewound.Game.Round != 1 || rewound.Game.Phase != "awaiting_activation" || len(rewound.Game.ActionHistory) != 20 || rewound.Game.CurrentActivation != nil {
		t.Fatalf("rewound game round=%d phase=%q actions=%d activation=%#v, want post-setup state", rewound.Game.Round, rewound.Game.Phase, len(rewound.Game.ActionHistory), rewound.Game.CurrentActivation)
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

func containsLegalAction(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func unitSetupsJSON(count int) string {
	setups := make([]string, 0, count)
	for i := 0; i < count; i++ {
		setups = append(setups, `{"baseWidthMm":25,"baseDepthMm":25,"count":1}`)
	}
	return `[` + strings.Join(setups, ",") + `]`
}

func firstUnplacedUnitForPlayer(g *game.Game, playerID int) game.Unit {
	for _, unit := range g.Units {
		if unit.PlayerID == playerID && !unit.Placed {
			return unit
		}
	}
	return game.Unit{}
}

func firstUnactivatedUnitForPlayerThisRound(g *game.Game, playerID int) (game.Unit, bool) {
	activated := map[string]bool{}
	for _, rec := range g.ActionHistory {
		if rec.Round == g.Round && rec.Type == game.ActionActivate {
			activated[rec.UnitID] = true
		}
	}
	for _, unit := range g.Units {
		if unit.PlayerID == playerID && unit.Placed && !unit.Broken && !activated[unit.ID] {
			return unit, true
		}
	}
	return game.Unit{}, false
}

func placementPoint(playerID int) (int, int) {
	if playerID == 1 {
		return 120, 120
	}
	return 620, 400
}

func manyUnitPlacementPoint(playerID int, unitID string) (int, int) {
	index := 0
	parts := strings.Split(unitID, "-u")
	if len(parts) == 2 {
		if parsed, err := strconv.Atoi(parts[1]); err == nil {
			index = parsed - 1
		}
	}
	if unitID == "u1" || unitID == "u2" {
		index = 0
	}
	col := index % 5
	row := index / 5
	if playerID == 1 {
		return 90 + col*55, 95 + row*55
	}
	return 500 + col*55, 330 + row*55
}
