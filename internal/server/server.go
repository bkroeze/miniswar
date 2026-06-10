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
	store      *store.Store
	engine     *game.Engine
	indexTmpl  *template.Template
	armiesTmpl *template.Template
}

func New(store *store.Store, engine *game.Engine) *Server {
	return &Server{
		store:      store,
		engine:     engine,
		indexTmpl:  template.Must(template.ParseFiles(projectPath("web/templates/index.html"))),
		armiesTmpl: template.Must(template.ParseFiles(projectPath("web/templates/armies.html"))),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /armies", s.armiesPage)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("GET /api/catalog/units", s.catalogUnits)
	mux.HandleFunc("GET /api/catalog/filters", s.catalogFilters)
	mux.HandleFunc("GET /api/army-templates", s.listArmyTemplates)
	mux.HandleFunc("POST /api/army-templates", s.createArmyTemplate)
	mux.HandleFunc("GET /api/army-templates/{id}", s.getArmyTemplate)
	mux.HandleFunc("PATCH /api/army-templates/{id}", s.updateArmyTemplate)
	mux.HandleFunc("POST /api/army-templates/{id}/units", s.addTemplateUnit)
	mux.HandleFunc("PATCH /api/army-templates/{id}/units/{unitID}", s.updateTemplateUnit)
	mux.HandleFunc("DELETE /api/army-templates/{id}/units/{unitID}", s.deleteTemplateUnit)
	mux.HandleFunc("GET /api/armies", s.listArmies)
	mux.HandleFunc("POST /api/armies", s.createArmy)
	mux.HandleFunc("POST /api/armies/from-template", s.createArmyFromTemplate)
	mux.HandleFunc("GET /api/armies/{id}", s.getArmy)
	mux.HandleFunc("PATCH /api/armies/{id}", s.updateArmy)
	mux.HandleFunc("POST /api/armies/{id}/units", s.addArmyUnit)
	mux.HandleFunc("PATCH /api/armies/{id}/units/{unitID}", s.updateArmyUnit)
	mux.HandleFunc("DELETE /api/armies/{id}/units/{unitID}", s.deleteArmyUnit)
	mux.HandleFunc("POST /api/games", s.createGame)
	mux.HandleFunc("GET /api/games", s.listGames)
	mux.HandleFunc("GET /api/games/{id}", s.getGame)
	mux.HandleFunc("POST /api/games/{id}/placements", s.placeUnit)
	mux.HandleFunc("POST /api/games/{id}/activate", s.activate)
	mux.HandleFunc("POST /api/games/{id}/actions", s.action)
	mux.HandleFunc("GET /api/games/{id}/actions", s.actions)
	mux.HandleFunc("POST /api/games/{id}/rewind", s.rewind)
	return mux
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.indexTmpl.Execute(w, nil)
}

func (s *Server) armiesPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.armiesTmpl.Execute(w, nil)
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var req game.Setup
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Player1ArmyID != "" {
		units, err := s.store.ArmyUnitSetups(req.Player1ArmyID, 1)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		req.Player1Units = units
	}
	if req.Player2ArmyID != "" {
		units, err := s.store.ArmyUnitSetups(req.Player2ArmyID, 2)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		req.Player2Units = units
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

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	games, err := s.store.ListGames()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, game.APIResponse{OK: true, Games: games, Messages: []string{}})
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

func (s *Server) placeUnit(w http.ResponseWriter, r *http.Request) {
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
	var req game.PlacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rec, err := s.engine.PlaceUnit(g, req)
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
	if err := s.store.DeleteSnapshotsAfter(g.ID, req.ActionIndex); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, game.APIResponse{OK: true, Game: g, LegalActions: game.LegalActions(g), Messages: []string{"Game rewound."}})
}

func (s *Server) catalogUnits(w http.ResponseWriter, r *http.Request) {
	units, err := s.store.CatalogUnits(r.URL.Query().Get("nation"), r.URL.Query().Get("terrain"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "units": units, "messages": []string{}})
}

func (s *Server) catalogFilters(w http.ResponseWriter, r *http.Request) {
	filters, err := s.store.CatalogFilters()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "filters": filters, "messages": []string{}})
}

func (s *Server) listArmyTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := s.store.ListArmyTemplates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "templates": templates, "messages": []string{}})
}

func (s *Server) createArmyTemplate(w http.ResponseWriter, r *http.Request) {
	var req armyNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	t, err := s.store.CreateArmyTemplate(req.Name, req.TargetPoints)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "template": t, "messages": []string{"Template created."}})
}

func (s *Server) getArmyTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetArmyTemplate(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "template": t, "messages": []string{}})
}

func (s *Server) updateArmyTemplate(w http.ResponseWriter, r *http.Request) {
	var req armyNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	t, err := s.store.UpdateArmyTemplate(r.PathValue("id"), req.Name, req.TargetPoints)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "template": t, "messages": []string{"Template updated."}})
}

func (s *Server) addTemplateUnit(w http.ResponseWriter, r *http.Request) {
	var req unitLineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	t, err := s.store.AddTemplateUnit(r.PathValue("id"), req.CatalogUnitID, req.Moniker, req.MiniCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "template": t, "messages": []string{"Unit added."}})
}

func (s *Server) updateTemplateUnit(w http.ResponseWriter, r *http.Request) {
	var req unitLinePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	t, err := s.store.GetArmyTemplate(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	line, ok := templateUnitByID(t, r.PathValue("unitID"))
	if !ok {
		writeError(w, http.StatusInternalServerError, sql.ErrNoRows)
		return
	}
	moniker := line.DefaultMoniker
	if req.Moniker != nil {
		moniker = *req.Moniker
	}
	miniCount := line.MiniCount
	if req.MiniCount != nil {
		miniCount = *req.MiniCount
	}
	t, err = s.store.UpdateTemplateUnit(r.PathValue("id"), r.PathValue("unitID"), moniker, miniCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "template": t, "messages": []string{"Unit updated."}})
}

func (s *Server) deleteTemplateUnit(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.DeleteTemplateUnit(r.PathValue("id"), r.PathValue("unitID"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "template": t, "messages": []string{"Unit removed."}})
}

func (s *Server) listArmies(w http.ResponseWriter, r *http.Request) {
	armies, err := s.store.ListArmies()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "armies": armies, "messages": []string{}})
}

func (s *Server) createArmy(w http.ResponseWriter, r *http.Request) {
	var req armyNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	army, err := s.store.CreateArmy(req.Name, req.TargetPoints)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "army": army, "messages": []string{"Army created."}})
}

func (s *Server) createArmyFromTemplate(w http.ResponseWriter, r *http.Request) {
	var req fromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	army, err := s.store.CreateArmyFromTemplate(req.TemplateID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "army": army, "messages": []string{"Army roster created."}})
}

func (s *Server) getArmy(w http.ResponseWriter, r *http.Request) {
	army, err := s.store.GetArmy(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "army": army, "messages": []string{}})
}

func (s *Server) updateArmy(w http.ResponseWriter, r *http.Request) {
	var req armyNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	army, err := s.store.UpdateArmy(r.PathValue("id"), req.Name, req.TargetPoints)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "army": army, "messages": []string{"Army updated."}})
}

func (s *Server) addArmyUnit(w http.ResponseWriter, r *http.Request) {
	var req unitLineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	army, err := s.store.AddArmyUnit(r.PathValue("id"), req.CatalogUnitID, req.Moniker, req.MiniCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "army": army, "messages": []string{"Unit added."}})
}

func (s *Server) updateArmyUnit(w http.ResponseWriter, r *http.Request) {
	var req unitLinePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	army, err := s.store.GetArmy(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	line, ok := armyUnitByID(army, r.PathValue("unitID"))
	if !ok {
		writeError(w, http.StatusInternalServerError, sql.ErrNoRows)
		return
	}
	moniker := line.Moniker
	if req.Moniker != nil {
		moniker = *req.Moniker
	}
	miniCount := line.MiniCount
	if req.MiniCount != nil {
		miniCount = *req.MiniCount
	}
	currentHealth := line.CurrentHealth
	if req.CurrentHealth != nil {
		currentHealth = *req.CurrentHealth
	}
	army, err = s.store.UpdateArmyUnit(r.PathValue("id"), r.PathValue("unitID"), moniker, miniCount, currentHealth)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "army": army, "messages": []string{"Unit updated."}})
}

func (s *Server) deleteArmyUnit(w http.ResponseWriter, r *http.Request) {
	army, err := s.store.DeleteArmyUnit(r.PathValue("id"), r.PathValue("unitID"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "army": army, "messages": []string{"Unit removed."}})
}

type armyNameRequest struct {
	Name         string `json:"name"`
	TargetPoints int    `json:"targetPoints"`
}

type unitLineRequest struct {
	CatalogUnitID string `json:"catalogUnitId"`
	Moniker       string `json:"moniker"`
	MiniCount     int    `json:"miniCount"`
	CurrentHealth int    `json:"currentHealth"`
}

type unitLinePatchRequest struct {
	Moniker       *string `json:"moniker"`
	MiniCount     *int    `json:"miniCount"`
	CurrentHealth *int    `json:"currentHealth"`
}

func templateUnitByID(template store.ArmyTemplate, id string) (store.ArmyTemplateUnit, bool) {
	for _, unit := range template.Units {
		if unit.ID == id {
			return unit, true
		}
	}
	return store.ArmyTemplateUnit{}, false
}

func armyUnitByID(army store.Army, id string) (store.ArmyUnit, bool) {
	for _, unit := range army.Units {
		if unit.ID == id {
			return unit, true
		}
	}
	return store.ArmyUnit{}, false
}

type fromTemplateRequest struct {
	TemplateID string `json:"templateId"`
	Name       string `json:"name"`
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
