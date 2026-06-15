---
date: 2026-06-10
topic: battlemap-zoom-editor
---

# Battlemap Zoom and Editor Requirements

## Summary

Miniswar will add a reusable battlemap library with a first-pass editor for rectangular terrain zones, and the arena SVG will gain camera controls that can zoom from a 200mm minimum visible side out to the full saved map. Games will choose a saved battlemap and preserve the exact battlemap definition they started with.

---

## Problem Frame

The current arena works because maps are small and fixed-size, but battlemaps need to become authored scenario assets. Larger maps require more than visual scaling: placement, movement, terrain overlap, and AI automation all need the same map dimensions and terrain definitions that the player sees.

The project already treats the arena as an SVG world measured in millimeters and requires game actions to be available through structured feedback for automation. Battlemap editing and zooming should extend that model instead of introducing UI-only state.

---

## Key Decisions

- **Reusable battlemaps.** Battlemaps are managed as reusable assets, not as one-off settings buried inside new-game setup. This keeps scenario preparation separate from play.
- **Rectangular terrain first.** The first editor supports rectangular terrain zones with type, label, position, and dimensions. This matches the current terrain behavior and avoids introducing a general drawing tool before the rules need it.
- **SVG viewBox camera.** Zoom and pan change the visible map area in SVG world coordinates. CSS-only scaling is not enough because click coordinates, placement previews, and automation feedback must remain meaningful in millimeters.
- **Game-local battlemap copy.** A game preserves the battlemap definition it started with. Later edits to the reusable battlemap do not mutate saved or in-progress games.

---

## Actors

- A1. **Scenario author.** Creates and edits reusable battlemaps before games are started.
- A2. **Player.** Chooses a battlemap for a new game and uses zoom/pan while placing and moving units.
- A3. **Automation client.** Lists, creates, validates, and selects battlemaps through structured API behavior.
- A4. **Rules engine.** Validates movement, placement, and terrain effects against the selected battlemap dimensions and zones.

---

## Requirements

**Battlemap Library**

- R1. The app must provide a reusable battlemap library where users can list, create, rename, edit, and delete saved battlemaps.
- R2. Each saved battlemap must have a name, dimensions in millimeters, and a collection of terrain zones.
- R3. The new-game flow must allow users to choose from saved battlemaps.
- R4. Starting a game must copy the selected battlemap definition into game state so the game remains stable if the reusable battlemap changes later.
- R5. Built-in starter battlemaps should remain available or be migrated into the reusable library without breaking existing games.

**Battlemap Editor**

- R6. The first editor must support rectangular terrain zones only.
- R7. Each terrain zone must capture type, label, x/y position, width, and height in millimeters.
- R8. The editor must let users add, select, move, resize, update, and delete terrain zones.
- R9. The editor must validate map dimensions and terrain geometry before saving.
- R10. The editor must preview the battlemap in the same SVG millimeter coordinate system used during play.
- R11. Terrain editing feedback must be useful outside the browser UI, including structured validation errors for invalid dimensions, invalid terrain types, or out-of-bounds zones.

**Zoom and Navigation**

- R12. The arena must be able to zoom out to show the full battlemap on screen regardless of the battlemap dimensions.
- R13. The maximum zoom-in level must keep the shorter visible side at least 200mm while preserving the SVG aspect ratio.
- R14. Zooming and panning must preserve meaningful millimeter coordinates for clicks, placement previews, and editor interactions.
- R15. The UI must provide basic camera controls for fit-to-map, zoom in, zoom out, and pan.
- R16. The camera must clamp panning so users cannot lose the battlemap entirely off screen.
- R17. Camera state must be view state, not game history, unless a future workflow explicitly makes camera position shareable or replayable.

**Game Rules and Automation**

- R18. Placement, movement, combat alignment, pushback, withdraw, and terrain overlap checks must use the active battlemap dimensions instead of a fixed global arena size.
- R19. Game responses must expose the active battlemap dimensions and terrain zones so AI and scripted clients can reason about legal actions.
- R20. Battlemap management must be available through structured server behavior, not only DOM interactions.
- R21. Rewind must continue to restore the game-local battlemap copy along with the rest of game state.

---

## Key Flows

- F1. **Create a reusable battlemap**
  - **Actors:** A1, A3
  - **Steps:** The author creates a battlemap, sets its dimensions, adds rectangular terrain zones, resolves validation feedback, and saves it to the library.
  - **Outcome:** The battlemap appears as a selectable reusable asset.
  - **Covered by:** R1, R2, R6, R7, R8, R9, R11

- F2. **Start a game from a saved battlemap**
  - **Actors:** A2, A4
  - **Steps:** The player opens new-game setup, selects a saved battlemap, chooses armies or manual units, and creates the game.
  - **Outcome:** The game stores its own copy of the selected battlemap and uses it for rendering and rules.
  - **Covered by:** R3, R4, R18, R19, R21

- F3. **Navigate a large battlemap**
  - **Actors:** A2
  - **Steps:** The player fits the full map to screen, zooms in toward a tactical area, pans within map bounds, and places or selects units using SVG coordinates.
  - **Outcome:** The visible area changes without corrupting the millimeter-based action model.
  - **Covered by:** R12, R13, R14, R15, R16, R17

---

## Acceptance Examples

- AE1. **Covers R12, R13.** Given a battlemap much larger than the viewport, when the user chooses fit-to-map, then the full map is visible. When the user zooms in repeatedly, then the shorter visible side stops at 200mm or more.
- AE2. **Covers R4, R21.** Given a game created from a saved battlemap, when the reusable battlemap is later edited, then the existing game and its rewind snapshots still use the original copied definition.
- AE3. **Covers R14, R18.** Given the arena is zoomed and panned, when the player clicks to place a unit, then the server receives millimeter coordinates in the battlemap world and validates them against the active map bounds.
- AE4. **Covers R9, R11.** Given an author creates a terrain rectangle extending outside the map, when they save, then the save is rejected with structured feedback identifying the invalid zone.

---

## Scope Boundaries

Deferred for later:

- Freeform terrain, polygons, brush drawing, imported images, and map art layers.
- Deployment zones, scenario objectives, scripted setup rules, and scenario notes.
- Minimap navigation and shareable camera bookmarks.
- Battlemap versioning UX beyond preserving the game-local copy.

Outside this first pass:

- Canvas rendering for the play area.
- React-based battlemap editing.
- UI-only battlemap behavior that cannot be exercised through structured server feedback.

---

## Dependencies / Assumptions

- Saved battlemaps will be persisted in SQLite alongside games, snapshots, army templates, and rosters.
- The existing SVG arena remains the primary visual surface for play and editing.
- Existing built-in maps should keep working during the transition to saved battlemaps.
- Camera state does not need to be rewindable because it does not change game state.

---

## Success Criteria

- Users can create a saved rectangular-terrain battlemap and select it when starting a game.
- Very large battlemaps can be fit fully on screen and zoomed in until the shorter visible side reaches 200mm.
- Placement and movement validation use the selected battlemap dimensions.
- Automation clients can manage battlemaps and receive actionable validation feedback.
- Existing game rewind behavior remains intact.

---

## Sources / Research

- `AGENTS.md` establishes SQLite persistence, SVG-only arena rendering, millimeter coordinates, automation-friendly actions, and rewind as project constraints.
- `PLAN.md` describes the current game state, JSON APIs, SVG arena, saved games, snapshots, and fixed built-in battlemaps.
- `internal/game/types.go` currently defines battlemap and terrain data structures plus fixed arena dimensions.
- `internal/game/engine.go` currently provides built-in battlemaps and uses fixed arena dimensions for rule validation.
- `web/templates/index.html` currently renders the arena with a fixed SVG viewBox and hard-coded battlemap options.
- `web/static/app.js` currently converts clicks through SVG coordinates and renders terrain zones from game state.
