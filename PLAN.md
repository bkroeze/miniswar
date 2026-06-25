# Miniswar App

## Summary

Miniswar is a Go web app with SQLite-backed game state, JSON APIs for all game actions, army and roster management, reusable battlemaps, and an SVG arena UI driven by Alpine.js. The current app supports catalog-backed armies, templates, multi-unit setup, activation rolls, alternating activations, movement and shooting actions, move-into-combat resolution, morale, pushback choices, action history, rewind, and URL-addressable game steps.

## Key Changes

- `cmd/miniswar` starts the HTTP server with `-addr` and `-db` flags. Local runs default to `miniswar.sqlite`; the production container passes `-db /storage/miniswar.sqlite` for Single Server persistent storage.
- `internal/version/VERSION` stores the base app version. Local `just` builds and runs pass the current branch through ldflags, Docker builds accept `APP_VERSION`, `APP_BRANCH`, and `APP_DEFAULT_BRANCH`, and branch builds display a sanitized `base-branch` suffix while default-branch builds display the bare base version.
- `.github/workflows/bump-version.yml` increments the minor decimal base version after merged PRs, skipping the initial `fm/miniswar-version-f9` versioning PR.
- `internal/game` owns rules, state transitions, layouts, activation, movement, shooting, combat, morale, legal actions, and rewind snapshots.
- `internal/store` persists games, snapshots, the imported unit catalog, army templates, and army rosters in SQLite. Ordinary filesystem database paths create missing parent directories before opening; `:memory:` and `file:` SQLite DSNs skip parent directory handling.
- `web` serves the landing page, management pages, CSS, Alpine.js, and SVG rendering.
- The store imports `data/units.json` into `catalog_units` and `catalog_unit_terrains` when it opens.
- The arena is rendered entirely in SVG using millimeter coordinates. Minis are rectangles sized by base dimensions, with unit/player color, facing indicator, mini key, officer marking, status styling, engagement styling, and contextual unit-adjacent controls.
- During play, the browser uses a top gameplay banner plus SVG controls near the active unit instead of a right-side action form. The active unit's move control opens clickable forward and backward movement bars, and eligible shooting units can open server-provided clickable target crosses. Feedback and rewindable action history live in the left bar with unit details, and the URL tracks the current game and action step.
- Battlemaps are saved in SQLite with dimensions and rectangular terrain zones. The browser includes a battlemap editor, and active games copy the chosen map definition so later library edits do not mutate saved or rewindable game state.
- Setup can use saved army rosters for either player, or fall back to manual units when no roster is selected.
- Unit layout uses stable mini keys like `p1-u1-m01`; the officer defaults to one of the center positions in the front rank.

## Public Interfaces

- Browser routes:
  - `GET /` renders the landing page and game shell.
  - `GET /games/{id}` renders the game shell and loads the latest saved state for that game.
  - `GET /games/{id}/steps/{step}` renders the game shell and loads a specific action-history step for that game.
  - `GET /armies` renders the army template and roster manager.
  - `GET /battlemaps` renders the reusable battlemap library and rectangular terrain editor.
  - `GET /static/*` serves CSS, JavaScript, and Alpine.js.
- The landing page footer renders the display version and `Copyright (c) 2026 Bruce Kroeze`.
- Catalog API:
  - `GET /api/catalog/units?nation=&terrain=` returns catalog units, optionally filtered by exact nation and terrain.
  - `GET /api/catalog/filters` returns available `nations` and `terrains`.
- Army template API:
  - `GET /api/army-templates` lists templates.
  - `POST /api/army-templates` creates a template from `{ "name": string, "targetPoints": number }`.
  - `GET /api/army-templates/{id}` returns a template with units.
  - `PATCH /api/army-templates/{id}` updates template name and target points.
  - `POST /api/army-templates/{id}/units` adds a catalog unit from `{ "catalogUnitId": string, "moniker": string, "miniCount": number }`.
  - `PATCH /api/army-templates/{id}/units/{unitID}` updates default moniker and mini count.
  - `DELETE /api/army-templates/{id}/units/{unitID}` removes a template unit.
- Army roster API:
  - `GET /api/armies` lists rosters.
  - `POST /api/armies` creates a roster from `{ "name": string, "targetPoints": number }`.
  - `POST /api/armies/from-template` creates a roster from `{ "templateId": string, "name": string }`.
  - `GET /api/armies/{id}` returns a roster with units.
  - `PATCH /api/armies/{id}` updates roster name and target points.
  - `POST /api/armies/{id}/units` adds a catalog unit from `{ "catalogUnitId": string, "moniker": string, "miniCount": number }`.
  - `PATCH /api/armies/{id}/units/{unitID}` updates moniker, mini count, and current health.
  - `DELETE /api/armies/{id}/units/{unitID}` removes a roster unit.
- Battlemap API:
  - `GET /api/battlemaps` lists saved battlemaps.
  - `POST /api/battlemaps` creates a battlemap from `{ "name": string, "widthMm": number, "heightMm": number, "terrains": [] }`; omitted dimensions default to `760x520mm` and omitted terrain defaults to an empty list.
  - `GET /api/battlemaps/{id}` returns one battlemap.
  - `PATCH /api/battlemaps/{id}` updates custom battlemap name, dimensions, and terrain.
  - `DELETE /api/battlemaps/{id}` deletes custom battlemaps. Built-in starter maps are protected from update and delete.
- Game API:
  - `POST /api/games` creates a game. The request can provide `player1Units` and `player2Units`, legacy `player1` and `player2`, or `player1ArmyId` and `player2ArmyId` to load roster units. Manual units use `baseWidthMm`, `baseDepthMm`, `count`, optional `name`, optional catalog/army identity fields, optional `stats`, optional `special`, optional `equipment`, and optional health fields.
  - `GET /api/games` lists saved games.
  - `GET /api/games/{id}` returns full game state.
  - `GET /api/games/{id}/steps/{step}` returns the game state at an action-history step. Step `0` is the initial saved setup state, the current step is writable, and earlier historical steps include `readOnly: true`.
  - `POST /api/games/{id}/placements` places the next setup unit from `{ "playerId": number, "unitId": string, "x": number, "y": number, "facingDeg": number }`.
  - `POST /api/games/{id}/activate` activates a unit from `{ "playerId": number, "unitId": string }`, rolls `2d10`, records success/failure, may resolve engagement combat, and returns available actions with legal target details when shooting is available.
  - `POST /api/games/{id}/actions` applies `move`, `pivot`, `about_face`, `shoot`, `skip`, or `combat_pushback` from an action request with `playerId`, `unitId`, `type`, and type-specific fields such as `direction`, `distanceMm`, `facingDeg`, `anchorKey`, `targetUnitId`, or `combatChoice`.
  - `GET /api/games/{id}/actions` returns action history with machine-readable results.
  - `POST /api/games/{id}/rewind` rewinds to `{ "actionIndex": number }` and deletes later snapshots. Missing snapshots return `400` with `game snapshot not found`.
- Game mutation and step responses use `APIResponse` where practical: `ok`, `game`, `action`, `roll`, `legalActions`, `legalActionDetails`, `readOnly`, `messages`, and `errors`.
- Army and catalog responses use the same `ok`, domain object, and `messages` pattern.
- `battlemapId` defaults to a saved starter map. Game creation resolves the ID through the battlemap store, copies the map definition into the game, and rejects unknown IDs.
- Adding template or roster units with `miniCount` omitted or `0` uses one full rank for that base size, and counts above the base maximum are clamped. Roster `currentHealth` is clamped between `0` and the unit's max health.

## Game Behavior

- On round 1, randomly choose the first player and record the seed and opening initiative.
- Players alternate activations. Each unbroken unit activates once per round, then a new round begins.
- Activation roll: roll two ten-sided dice. If either die is greater than or equal to the unit activation number, activation succeeds.
- Player 1 unit activation number is `5`; player 2 unit activation number is `4`.
- Disordered units activate at activation number +1. If a disordered unit succeeds, disorder clears and that activation is limited to one simple action.
- Successful activation grants two actions. Failed activation grants one simple action.
- Legal active actions are:
  - `move`: straight forward up to movement limit, backward up to half. A second move in the same activation has half movement.
  - `pivot`: rotate unit around the officer by default, or around a selected mini key anchor when supplied.
  - `about_face`: reverse facing and reorganize ranks according to the base layout rules.
  - `shoot`: attack one legal enemy target with a listed shooting weapon.
  - `skip`: end the activation.
- `shoot` is legal only for a successful, non-simple activation when the unit has actions remaining, has not already shot this activation, is not in combat, has a listed shooting weapon or matching special ability, and has at least one enemy target in range and line of sight. Units in active combat cannot be shooting targets.
- Shooting weapons are matched from unit equipment, special abilities, or name. Current ranges are Bow `500mm`, Elf Bow `550mm`, Sling `300mm`, Light Catapult `800mm`, Heavy Catapult `1000mm`, Ballista `750mm`, and Fire Breath `300mm`.
- Legal shooting target details are returned in `legalActionDetails` as `shoot` targets with weapon, range, range limit, line-of-sight result, and target center for browser or automation targeting.
- Shooting action results include target unit, weapon, range, line of sight, dice count, target number, modifiers, rolls, hits, casualties, morale tests, broken units, and target removal.
- Shooting line of sight is traced from the officer base through the attacker's front arc. `Indirect Fire` bypasses line of sight, target `Large` or `Enormous` changes which blockers matter, `Shielding` reduces shooting dice by one with a minimum of one, and terrain cover/obscuring hooks currently return no modifier or blocker until richer terrain data exists.
- A unit with an `M` stat moves `M * 25mm`; units without stats use `100mm`.
- Rough terrain doubles movement cost only for the overlapping portion of a move. Impassable terrain blocks placement, movement, pivot, about-face, combat alignment, and pushback/withdraw movement. Path terrain is currently visual only.
- Unit placement, movement, combat alignment, pushback, and withdrawal use the active battlemap bounds instead of a fixed arena size.
- The browser camera uses the SVG `viewBox`: Fit shows the entire active map regardless of size, zoom clamps at a 200mm minimum visible side, and pan controls keep the view inside map bounds.
- Units may pass through friendly units only if the move fully clears them; otherwise movement backs up to the last clear position. Enemy contact during forward or backward movement triggers combat.
- Moving into an enemy creates an engagement, snaps the attacker flush to the defender face when possible, accepts a small geometry tolerance for angled contact, and resolves a combat round.
- Activating a unit already engaged with an enemy also resolves combat before ordinary actions continue.
- Combat records dice counts, target numbers, modifiers, rolls, hits, casualties, morale tests, broken units, winners, and pending pushback choices.
- Shooting records dice counts, target numbers, modifiers, rolls, hits, casualties, morale tests caused by shooting, broken units, and removed targets.
- While a pending combat choice exists, legal actions are limited to `combat_pushback` with one of `pushback_25`, `pushback_75`, `withdraw_25`, or `decline`.
- Passable-obstacle terrain does not block movement, but it marks a defender as fortified when the attacker crosses or contacts it while moving into combat.
- Roster health is copied into each mini when a game starts. Units with zero current health start removed, are skipped during placement and activation, and can immediately determine a win or draw after setup.
- When only one player has units left on the battlefield, that player wins and the game phase becomes `complete`. If no player has active units left, the game completes as a draw.
- Store pre-action snapshots so rewind works during active, pending-combat, and completed games.
- Browser play URLs use `/games/{id}/steps/{actionCount}`. The browser replaces the URL as play progresses, saved-game links open the current step, and historical step URLs load read-only state without rewinding the current saved game.

## Test Plan

- Unit tests for base-size validation, max unit size, rank layout, officer placement, mini key stability, and catalog-derived movement/health.
- Unit tests for activation success/failure, disordered activation, legal action gating, movement limits, shooting eligibility, line of sight, shooting resolution, and failed-activation simple action limits.
- Unit tests for combat alignment, dice, target numbers, hit allocation, officer-safe casualties, morale, broken cascades, pushback/withdraw/decline, and win completion.
- Store tests for catalog import, filters, template CRUD, roster CRUD, battlemap CRUD, validation, army-to-game setup conversion, filesystem parent directory creation, and idempotent reopen behavior that preserves user-created rows.
- Version tests for base/default-branch display, branch suffix derivation, and branch-name sanitization.
- HTTP tests for catalog endpoints, army endpoints, battlemap endpoints, valid and invalid game creation, copied game battlemaps, place units, activate, apply actions including shooting, pivot, about-face, and combat pushback, list actions, persistence, rewind including invalid rewind requests, full multi-unit round progression, read-only historical step lookup, and the landing footer version/copyright text.
- UI wiring tests ensure Alpine template event handlers reference methods implemented by `web/static/app.js`.
- Manual browser check: manage templates/rosters/battlemaps, create a game from rosters, place units, zoom/pan/fit the SVG arena, activate with the unit-adjacent `+`, open the move control and click forward/backward movement bars, use shoot controls and target crosses when eligible, reload the saved game and repeat movement, pivot/about-face from battlemap clicks, skip with `~`, enter combat, resolve pushback, rewind, open a saved game URL at the latest step, open an earlier step URL, and verify SVG updates, banner text, left-bar history, URL tracking, read-only historical state, landing footer, and action feedback.

## Assumptions

- Use Go 1.26.2 already available in the environment.
- Use `modernc.org/sqlite` to avoid CGO requirements.
- Use Alpine.js for the UI, with the app still fully operable through JSON APIs.
- Wheel movement, multiplayer sockets, authentication, and special ability execution beyond shooting-related hooks remain deferred.
