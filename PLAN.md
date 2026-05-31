# Phase 1 Miniswar App

## Summary

Build the first playable scaffold from `IDEA.md` and `RULES.md`: a Go web app with SQLite-backed game state, JSON APIs for all game actions, and an SVG arena UI driven by Alpine.js. Phase 1 supports setup, two players, one unit per player, activation rolls, alternating activations, movement-oriented actions, action history, and rewind.

## Key Changes

- Initialize a Go module with a small layered structure:
  - `cmd/miniswar` starts the HTTP server.
  - `internal/game` owns rules, state transitions, layouts, activation, movement, and rewind snapshots.
  - `internal/store` persists games, turns, units, minis, actions, and snapshots in SQLite.
  - `web` serves HTML, CSS, Alpine.js, and SVG rendering.
- Use Go stdlib templates plus Alpine.js for the browser UI. No React, no canvas.
- Render the arena entirely in SVG using millimeter coordinates. Minis are rectangles sized by base dimensions, with unit/player color, facing indicator, mini key, and officer marking.
- Implement setup controls for unit sizes and base sizes, constrained by `RULES.md`.
- Create deterministic unit layout logic with stable mini keys like `p1-u1-m01`; officer defaults to one of the two center positions in the front rank.

## Public Interfaces

- Browser routes:
  - `GET /` renders the app shell.
  - `GET /static/*` serves CSS and JavaScript.
- JSON API:
  - `POST /api/games` creates a game from setup options and returns full state.
  - `GET /api/games/{id}` returns full game state.
  - `POST /api/games/{id}/activate` activates a unit, rolls `2d10`, records success/failure, and returns available actions.
  - `POST /api/games/{id}/actions` applies `move`, `pivot`, or `about_face`.
  - `POST /api/games/{id}/rewind` rewinds to a prior action index or snapshot id and returns full state.
  - `GET /api/games/{id}/actions` returns action history with machine-readable results.
- Every action response includes `ok`, `game`, `action`, `roll`, `legalActions`, `messages`, and `errors`.
- Do not implement shooting, combat resolution, damage, scenery interaction, sockets, or multiplayer auth in phase 1.

## Game Behavior

- On round 1, randomly choose the first player and record the seed/roll metadata.
- Players alternate activations. Each unit activates once per round, then a new round begins.
- Activation roll: roll two ten-sided dice. If either die is greater than or equal to the unit activation number, activation succeeds.
- Player 1 unit activation number is `5`; player 2 unit activation number is `4`.
- Successful activation grants two actions. Failed activation grants one simple action.
- Phase 1 actions:
  - `move`: straight forward up to movement limit, backward up to half. A second move in the same activation has half movement.
  - `pivot`: rotate unit around the officer by default, or around a selected mini key anchor when supplied.
  - `about_face`: reverse facing and reorganize ranks according to the base layout rules.
- Use a conservative default movement limit of `100mm` unless a later rule document specifies otherwise.
- Store pre-action snapshots so rewind works during active or completed games.

## Test Plan

- Unit tests for base-size validation, max unit size, rank layout, officer placement, and mini key stability.
- Unit tests for activation success/failure with injectable RNG.
- Unit tests for action legality: turn ownership, remaining actions, second-move distance reduction, and failed-activation simple action limit.
- Unit tests for rewind restoring game state and action history position.
- HTTP tests for create game, get game, activate, apply action, list actions, and rewind.
- Manual browser check: create a game with different unit sizes, activate each side, move/pivot/about-face, verify SVG updates and action feedback.

## Assumptions

- Use Go 1.26.2 already available in the environment.
- Use `modernc.org/sqlite` to avoid CGO requirements.
- Use Alpine.js for the phase 1 UI, with the app still fully operable through JSON APIs.
