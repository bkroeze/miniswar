package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"miniswar/internal/game"
	"miniswar/internal/store"
)

type Server struct {
	store  *store.Store
	engine *game.Engine
	tmpl   *template.Template
}

func New(store *store.Store, engine *game.Engine) *Server {
	return &Server{
		store:  store,
		engine: engine,
		tmpl:   template.Must(template.ParseFiles(projectPath("web/templates/index.html"))),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("POST /api/games", s.createGame)
	mux.HandleFunc("GET /api/games/{id}", s.getGame)
	mux.HandleFunc("POST /api/games/{id}/activate", s.activate)
	mux.HandleFunc("POST /api/games/{id}/actions", s.action)
	mux.HandleFunc("GET /api/games/{id}/actions", s.actions)
	mux.HandleFunc("POST /api/games/{id}/rewind", s.rewind)
	return mux
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.Execute(w, nil)
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var req game.Setup
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	g, err := s.engine.NewGame(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	snap, err := game.Snapshot(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SaveGame(g); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SaveSnapshot(g.ID, -1, snap); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, game.APIResponse{OK: true, Game: g, LegalActions: game.LegalActions(g), Messages: []string{"Game created."}})
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	g, err := s.store.GetGame(r.PathValue("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, game.APIResponse{OK: true, Game: g, LegalActions: game.LegalActions(g)})
}

func (s *Server) activate(w http.ResponseWriter, r *http.Request) {
	g, err := s.store.GetGame(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	before, err := game.Snapshot(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var req game.ActivateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rec, roll, err := s.engine.Activate(g, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.persistMutation(g, rec.Index, before); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, game.APIResponse{OK: true, Game: g, Action: rec, Roll: roll, LegalActions: game.LegalActions(g), Messages: rec.Messages})
}

func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	g, err := s.store.GetGame(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	before, err := game.Snapshot(g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var req game.ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rec, err := s.engine.ApplyAction(g, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.persistMutation(g, rec.Index, before); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, game.APIResponse{OK: true, Game: g, Action: rec, LegalActions: game.LegalActions(g), Messages: rec.Messages})
}

func (s *Server) actions(w http.ResponseWriter, r *http.Request) {
	g, err := s.store.GetGame(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "actions": g.ActionHistory})
}

func (s *Server) rewind(w http.ResponseWriter, r *http.Request) {
	var req game.RewindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	state, err := s.store.Snapshot(r.PathValue("id"), req.ActionIndex)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	g, err := game.Restore(state)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SaveGame(g); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, game.APIResponse{OK: true, Game: g, LegalActions: game.LegalActions(g), Messages: []string{"Game rewound."}})
}

func (s *Server) persistMutation(g *game.Game, actionIndex int, before string) error {
	if err := s.store.SaveSnapshot(g.ID, actionIndex, before); err != nil {
		return err
	}
	return s.store.SaveGame(g)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, game.APIResponse{OK: false, Messages: []string{}, Errors: []string{cleanErr(err)}})
}

func cleanErr(err error) string {
	msg := err.Error()
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "unknown error"
	}
	if _, convErr := strconv.Atoi(msg); convErr == nil {
		return "invalid request"
	}
	return msg
}

func projectPath(path string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return path
	}
	return filepath.Join(filepath.Dir(file), "..", "..", path)
}
