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
	unit := g.Units[g.ActivePlayer-1]

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
	movedUnit := moved.Game.Units[g.ActivePlayer-1]
	if movedUnit.X == unit.X && movedUnit.Y == unit.Y {
		t.Fatal("move response did not change unit position")
	}

	res = request(t, srv, http.MethodPost, "/api/games/"+g.ID+"/rewind", `{"actionIndex":0}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rewind status %d: %s", res.Code, res.Body.String())
	}
	var rewound game.APIResponse
	if err := json.Unmarshal(res.Body.Bytes(), &rewound); err != nil {
		t.Fatal(err)
	}
	rewoundUnit := rewound.Game.Units[g.ActivePlayer-1]
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
