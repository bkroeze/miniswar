package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"miniswar/internal/game"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
create table if not exists games (
  id text primary key,
  state_json text not null,
  created_at text not null,
  updated_at text not null
);
create table if not exists snapshots (
  game_id text not null,
  action_index integer not null,
  state_json text not null,
  created_at text not null,
  primary key (game_id, action_index)
);`)
	return err
}

func (s *Store) SaveGame(g *game.Game) error {
	b, err := json.Marshal(g)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(`
insert into games(id, state_json, created_at, updated_at)
values (?, ?, ?, ?)
on conflict(id) do update set state_json = excluded.state_json, updated_at = excluded.updated_at`,
		g.ID, string(b), g.CreatedAt.Format(time.RFC3339Nano), now)
	return err
}

func (s *Store) GetGame(id string) (*game.Game, error) {
	var state string
	err := s.db.QueryRow(`select state_json from games where id = ?`, id).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	var g game.Game
	if err := json.Unmarshal([]byte(state), &g); err != nil {
		return nil, err
	}
	snapshots, err := s.Snapshots(id)
	if err != nil {
		return nil, err
	}
	g.Snapshots = snapshots
	return &g, nil
}

func (s *Store) ListGames() ([]game.GameSummary, error) {
	rows, err := s.db.Query(`
select g.id, g.state_json, g.created_at, g.updated_at, count(s.action_index)
from games g
left join snapshots s on s.game_id = g.id
group by g.id, g.state_json, g.created_at, g.updated_at
order by g.updated_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []game.GameSummary
	for rows.Next() {
		var id, state, created, updated string
		var snapshots int
		if err := rows.Scan(&id, &state, &created, &updated, &snapshots); err != nil {
			return nil, err
		}
		var g game.Game
		if err := json.Unmarshal([]byte(state), &g); err != nil {
			return nil, err
		}
		out = append(out, game.GameSummary{
			ID:            id,
			CreatedAt:     created,
			UpdatedAt:     updated,
			Round:         g.Round,
			Phase:         g.Phase,
			ActivePlayer:  g.ActivePlayer,
			BattlemapID:   g.Battlemap.ID,
			Battlemap:     g.Battlemap.Name,
			ActionCount:   len(g.ActionHistory),
			SnapshotCount: snapshots,
		})
	}
	return out, rows.Err()
}

func (s *Store) SaveSnapshot(gameID string, actionIndex int, state string) error {
	_, err := s.db.Exec(`
insert or replace into snapshots(game_id, action_index, state_json, created_at)
values (?, ?, ?, ?)`, gameID, actionIndex, state, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) Snapshot(gameID string, actionIndex int) (string, error) {
	var state string
	err := s.db.QueryRow(`select state_json from snapshots where game_id = ? and action_index = ?`, gameID, actionIndex).Scan(&state)
	return state, err
}

func (s *Store) Snapshots(gameID string) ([]game.SnapshotRecord, error) {
	rows, err := s.db.Query(`select action_index, state_json, created_at from snapshots where game_id = ? order by action_index`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []game.SnapshotRecord
	for rows.Next() {
		var rec game.SnapshotRecord
		var created string
		if err := rows.Scan(&rec.Index, &rec.GameJSON, &created); err != nil {
			return nil, err
		}
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, rec)
	}
	return out, rows.Err()
}
