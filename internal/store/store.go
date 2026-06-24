package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"miniswar/internal/game"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

var memoryStoreID uint64

func Open(path string) (*Store, error) {
	if err := ensureSQLiteParentDir(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`pragma foreign_keys = on`); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.SeedBattlemaps(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.ImportCatalog(projectPath("data/units.json")); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func ensureSQLiteParentDir(path string) error {
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func sqliteDSN(path string) string {
	if strings.HasPrefix(path, "file:") {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		return path + sep + "_pragma=foreign_keys(1)"
	}
	if path == ":memory:" {
		return fmt.Sprintf("file:miniswar-memory-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)", atomic.AddUint64(&memoryStoreID, 1))
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Add("_pragma", "foreign_keys(1)")
	u.RawQuery = q.Encode()
	return u.String()
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
);
create table if not exists battlemaps (
  id text primary key,
  name text not null,
  width_mm real not null,
  height_mm real not null,
  terrain_json text not null,
  is_builtin integer not null default 0,
  created_at text not null,
  updated_at text not null
);
create table if not exists catalog_units (
  id text primary key,
  unit_name text not null,
  nation text not null,
  a integer not null,
  m integer not null,
  f integer not null,
  s integer not null,
  d integer not null,
  cd integer not null,
  h integer not null,
  pts integer not null,
  base text not null,
  base_width_mm integer not null,
  base_depth_mm integer not null,
  special_json text not null,
  equipment_json text not null,
  source_hash text not null,
  created_at text not null,
  updated_at text not null
);
create table if not exists catalog_unit_terrains (
  unit_id text not null references catalog_units(id) on delete cascade,
  terrain text not null,
  primary key (unit_id, terrain)
);
create table if not exists army_templates (
  id text primary key,
  name text not null,
  target_points integer not null default 0,
  created_at text not null,
  updated_at text not null
);
create table if not exists army_template_units (
  id text primary key,
  template_id text not null references army_templates(id) on delete cascade,
  catalog_unit_id text not null references catalog_units(id),
  default_moniker text not null,
  mini_count integer not null,
  sort_order integer not null,
  created_at text not null,
  updated_at text not null
);
create table if not exists armies (
  id text primary key,
  template_id text references army_templates(id),
  name text not null,
  target_points integer not null default 0,
  created_at text not null,
  updated_at text not null
);
create table if not exists army_units (
  id text primary key,
  army_id text not null references armies(id) on delete cascade,
  catalog_unit_id text not null references catalog_units(id),
  moniker text not null,
  mini_count integer not null,
  max_health integer not null,
  current_health integer not null,
  sort_order integer not null,
  created_at text not null,
  updated_at text not null
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
	game.NormalizeGame(&g)
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
		game.NormalizeGame(&g)
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

func projectPath(path string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return path
	}
	return filepath.Join(filepath.Dir(file), "..", "..", path)
}

func (s *Store) SaveSnapshot(gameID string, actionIndex int, state string) error {
	_, err := s.db.Exec(`
insert or replace into snapshots(game_id, action_index, state_json, created_at)
values (?, ?, ?, ?)`, gameID, actionIndex, state, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) DeleteSnapshotsAfter(gameID string, actionIndex int) error {
	_, err := s.db.Exec(`delete from snapshots where game_id = ? and action_index > ?`, gameID, actionIndex)
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
