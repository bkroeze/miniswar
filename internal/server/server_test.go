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
