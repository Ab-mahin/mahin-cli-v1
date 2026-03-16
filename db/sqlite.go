// Package db manages the SQLite database for the movie CLI.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const dbDir = "movie-cli-output"
const dbFile = "mahin.db"

// DB wraps the sql.DB connection.
type DB struct {
	*sql.DB
	BasePath string // path to movie-cli-output
}

// Open opens (or creates) the SQLite database and runs migrations.
func Open() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}

	base := filepath.Join(home, dbDir)
	dirs := []string{
		base,
		filepath.Join(base, "json", "movie"),
		filepath.Join(base, "json", "tv"),
		filepath.Join(base, "json", "history"),
		filepath.Join(base, "thumbnails"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("cannot create directory %s: %w", d, err)
		}
	}

	dbPath := filepath.Join(base, dbFile)
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("cannot set WAL mode: %w", err)
	}

	d := &DB{DB: conn, BasePath: base}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return d, nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS media (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		title            TEXT NOT NULL,
		clean_title      TEXT NOT NULL,
		year             INTEGER,
		type             TEXT CHECK(type IN ('movie', 'tv')) NOT NULL,
		tmdb_id          INTEGER UNIQUE,
		imdb_id          TEXT,
		description      TEXT,
		imdb_rating      REAL,
		tmdb_rating      REAL,
		popularity       REAL,
		genre            TEXT,
		director         TEXT,
		cast_list        TEXT,
		thumbnail_path   TEXT,
		original_file_name TEXT,
		original_file_path TEXT,
		current_file_path  TEXT,
		file_extension   TEXT,
		file_size        INTEGER,
		scanned_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS move_history (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		media_id         INTEGER NOT NULL,
		from_path        TEXT NOT NULL,
		to_path          TEXT NOT NULL,
		original_file_name TEXT,
		new_file_name    TEXT,
		moved_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
		undone           INTEGER DEFAULT 0,
		FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS config (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS scan_history (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		folder_path   TEXT NOT NULL,
		total_files   INTEGER DEFAULT 0,
		movies_found  INTEGER DEFAULT 0,
		tv_found      INTEGER DEFAULT 0,
		scanned_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tags (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		media_id   INTEGER NOT NULL,
		tag        TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
		UNIQUE(media_id, tag)
	);

	CREATE INDEX IF NOT EXISTS idx_media_type       ON media(type);
	CREATE INDEX IF NOT EXISTS idx_media_title      ON media(clean_title);
	CREATE INDEX IF NOT EXISTS idx_media_year       ON media(year);
	CREATE INDEX IF NOT EXISTS idx_media_tmdb       ON media(tmdb_id);
	CREATE INDEX IF NOT EXISTS idx_move_history_media ON move_history(media_id);
	CREATE INDEX IF NOT EXISTS idx_move_history_undone ON move_history(undone);
	CREATE INDEX IF NOT EXISTS idx_tags_media       ON tags(media_id);

	INSERT OR IGNORE INTO config (key, value) VALUES ('movies_dir',  '~/Movies');
	INSERT OR IGNORE INTO config (key, value) VALUES ('tv_dir',      '~/TVShows');
	INSERT OR IGNORE INTO config (key, value) VALUES ('archive_dir', '~/Archive');
	INSERT OR IGNORE INTO config (key, value) VALUES ('scan_dir',    '~/Downloads');
	INSERT OR IGNORE INTO config (key, value) VALUES ('tmdb_api_key', '88636f9b250e2fd85a84a8580cb9e2ff');
	INSERT OR IGNORE INTO config (key, value) VALUES ('page_size',   '20');
	`
	_, err := d.Exec(schema)
	return err
}

// --- Helper methods ---

// Media represents a row in the media table.
type Media struct {
	ID               int64
	Title            string
	CleanTitle       string
	Year             int
	Type             string // "movie" or "tv"
	TmdbID           int
	ImdbID           string
	Description      string
	ImdbRating       float64
	TmdbRating       float64
	Popularity       float64
	Genre            string
	Director         string
	CastList         string
	ThumbnailPath    string
	OriginalFileName string
	OriginalFilePath string
	CurrentFilePath  string
	FileExtension    string
	FileSize         int64
}

// InsertMedia inserts a new media record and returns the ID.
func (d *DB) InsertMedia(m *Media) (int64, error) {
	var tmdbID interface{}
	if m.TmdbID > 0 {
		tmdbID = m.TmdbID
	}

	res, err := d.Exec(`
		INSERT INTO media (title, clean_title, year, type, tmdb_id, imdb_id,
			description, imdb_rating, tmdb_rating, popularity, genre, director,
			cast_list, thumbnail_path, original_file_name, original_file_path,
			current_file_path, file_extension, file_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Title, m.CleanTitle, m.Year, m.Type, tmdbID, m.ImdbID,
		m.Description, m.ImdbRating, m.TmdbRating, m.Popularity, m.Genre, m.Director,
		m.CastList, m.ThumbnailPath, m.OriginalFileName, m.OriginalFilePath,
		m.CurrentFilePath, m.FileExtension, m.FileSize,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateMediaByTmdbID updates an existing record matched by tmdb_id.
func (d *DB) UpdateMediaByTmdbID(m *Media) error {
	_, err := d.Exec(`
		UPDATE media SET title=?, clean_title=?, year=?, type=?, imdb_id=?,
			description=?, imdb_rating=?, tmdb_rating=?, popularity=?, genre=?,
			director=?, cast_list=?, thumbnail_path=?, current_file_path=?,
			file_extension=?, file_size=?, updated_at=CURRENT_TIMESTAMP
		WHERE tmdb_id=?`,
		m.Title, m.CleanTitle, m.Year, m.Type, m.ImdbID,
		m.Description, m.ImdbRating, m.TmdbRating, m.Popularity, m.Genre,
		m.Director, m.CastList, m.ThumbnailPath, m.CurrentFilePath,
		m.FileExtension, m.FileSize, m.TmdbID,
	)
	return err
}

// ListMedia returns paginated media records.
func (d *DB) ListMedia(offset, limit int) ([]Media, error) {
	rows, err := d.Query(`
		SELECT id, title, clean_title, year, type, tmdb_id, imdb_id,
			description, imdb_rating, tmdb_rating, popularity, genre,
			director, cast_list, thumbnail_path, original_file_name,
			original_file_path, current_file_path, file_extension, file_size
		FROM media ORDER BY clean_title ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMediaRows(rows)
}

// SearchMedia searches by title (fuzzy via LIKE).
func (d *DB) SearchMedia(query string) ([]Media, error) {
	rows, err := d.Query(`
		SELECT id, title, clean_title, year, type, tmdb_id, imdb_id,
			description, imdb_rating, tmdb_rating, popularity, genre,
			director, cast_list, thumbnail_path, original_file_name,
			original_file_path, current_file_path, file_extension, file_size
		FROM media WHERE clean_title LIKE ? OR title LIKE ?
		ORDER BY popularity DESC LIMIT 20`, "%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMediaRows(rows)
}

// GetMediaByID returns a single media record.
func (d *DB) GetMediaByID(id int64) (*Media, error) {
	row := d.QueryRow(`
		SELECT id, title, clean_title, year, type, tmdb_id, imdb_id,
			description, imdb_rating, tmdb_rating, popularity, genre,
			director, cast_list, thumbnail_path, original_file_name,
			original_file_path, current_file_path, file_extension, file_size
		FROM media WHERE id = ?`, id)
	m := &Media{}
	var tmdbID sql.NullInt64
	err := row.Scan(&m.ID, &m.Title, &m.CleanTitle, &m.Year, &m.Type,
		&tmdbID, &m.ImdbID, &m.Description, &m.ImdbRating, &m.TmdbRating,
		&m.Popularity, &m.Genre, &m.Director, &m.CastList, &m.ThumbnailPath,
		&m.OriginalFileName, &m.OriginalFilePath, &m.CurrentFilePath,
		&m.FileExtension, &m.FileSize)
	if err != nil {
		return nil, err
	}
	if tmdbID.Valid {
		m.TmdbID = int(tmdbID.Int64)
	}
	return m, nil
}

// GetMediaByTmdbID returns a single media record by TMDb ID.
func (d *DB) GetMediaByTmdbID(tmdbID int) (*Media, error) {
	row := d.QueryRow(`
		SELECT id, title, clean_title, year, type, tmdb_id, imdb_id,
			description, imdb_rating, tmdb_rating, popularity, genre,
			director, cast_list, thumbnail_path, original_file_name,
			original_file_path, current_file_path, file_extension, file_size
		FROM media WHERE tmdb_id = ? LIMIT 1`, tmdbID)

	m := &Media{}
	var nullableTMDBID sql.NullInt64
	err := row.Scan(&m.ID, &m.Title, &m.CleanTitle, &m.Year, &m.Type,
		&nullableTMDBID, &m.ImdbID, &m.Description, &m.ImdbRating, &m.TmdbRating,
		&m.Popularity, &m.Genre, &m.Director, &m.CastList, &m.ThumbnailPath,
		&m.OriginalFileName, &m.OriginalFilePath, &m.CurrentFilePath,
		&m.FileExtension, &m.FileSize)
	if err != nil {
		return nil, err
	}

	if nullableTMDBID.Valid {
		m.TmdbID = int(nullableTMDBID.Int64)
	}

	return m, nil
}

// CountMedia returns total count, optionally filtered by type.
func (d *DB) CountMedia(mediaType string) (int, error) {
	var count int
	var err error
	if mediaType == "" {
		err = d.QueryRow("SELECT COUNT(*) FROM media").Scan(&count)
	} else {
		err = d.QueryRow("SELECT COUNT(*) FROM media WHERE type = ?", mediaType).Scan(&count)
	}
	return count, err
}

// GetConfig returns a config value by key.
func (d *DB) GetConfig(key string) (string, error) {
	var val string
	err := d.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&val)
	return val, err
}

// SetConfig sets a config value.
func (d *DB) SetConfig(key, value string) error {
	_, err := d.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", key, value)
	return err
}

// InsertMoveHistory logs a move operation.
func (d *DB) InsertMoveHistory(mediaID int64, fromPath, toPath, origName, newName string) error {
	_, err := d.Exec(`
		INSERT INTO move_history (media_id, from_path, to_path, original_file_name, new_file_name)
		VALUES (?, ?, ?, ?, ?)`, mediaID, fromPath, toPath, origName, newName)
	return err
}

// MoveRecord represents a row in move_history.
type MoveRecord struct {
	ID               int64
	MediaID          int64
	FromPath         string
	ToPath           string
	OriginalFileName string
	NewFileName      string
	Undone           bool
}

// GetLastMove returns the latest un-undone move.
func (d *DB) GetLastMove() (*MoveRecord, error) {
	row := d.QueryRow(`
		SELECT id, media_id, from_path, to_path, original_file_name, new_file_name, undone
		FROM move_history WHERE undone = 0 ORDER BY moved_at DESC LIMIT 1`)
	r := &MoveRecord{}
	err := row.Scan(&r.ID, &r.MediaID, &r.FromPath, &r.ToPath, &r.OriginalFileName, &r.NewFileName, &r.Undone)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// MarkMoveUndone marks a move_history record as undone.
func (d *DB) MarkMoveUndone(id int64) error {
	_, err := d.Exec("UPDATE move_history SET undone = 1 WHERE id = ?", id)
	return err
}

// UpdateMediaPath updates the current file path.
func (d *DB) UpdateMediaPath(mediaID int64, newPath string) error {
	_, err := d.Exec("UPDATE media SET current_file_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newPath, mediaID)
	return err
}

// TopGenres returns genres sorted by frequency.
func (d *DB) TopGenres(limit int) (map[string]int, error) {
	rows, err := d.Query("SELECT genre FROM media WHERE genre != ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var genre string
		if err := rows.Scan(&genre); err != nil {
			continue
		}
		// genres are comma-separated
		for _, g := range splitCSV(genre) {
			counts[g]++
		}
	}
	return counts, nil
}

// InsertScanHistory logs a scan operation.
func (d *DB) InsertScanHistory(folder string, total, movies, tv int) error {
	_, err := d.Exec(`
		INSERT INTO scan_history (folder_path, total_files, movies_found, tv_found)
		VALUES (?, ?, ?, ?)`, folder, total, movies, tv)
	return err
}

// MediaByType returns media filtered by type.
func (d *DB) MediaByType(mediaType string, limit int) ([]Media, error) {
	rows, err := d.Query(`
		SELECT id, title, clean_title, year, type, tmdb_id, imdb_id,
			description, imdb_rating, tmdb_rating, popularity, genre,
			director, cast_list, thumbnail_path, original_file_name,
			original_file_path, current_file_path, file_extension, file_size
		FROM media WHERE type = ? ORDER BY popularity DESC LIMIT ?`, mediaType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMediaRows(rows)
}

// --- internal helpers ---

func scanMediaRows(rows *sql.Rows) ([]Media, error) {
	var list []Media
	for rows.Next() {
		var m Media
		var tmdbID sql.NullInt64
		if err := rows.Scan(&m.ID, &m.Title, &m.CleanTitle, &m.Year, &m.Type,
			&tmdbID, &m.ImdbID, &m.Description, &m.ImdbRating, &m.TmdbRating,
			&m.Popularity, &m.Genre, &m.Director, &m.CastList, &m.ThumbnailPath,
			&m.OriginalFileName, &m.OriginalFilePath, &m.CurrentFilePath,
			&m.FileExtension, &m.FileSize); err != nil {
			return nil, err
		}
		if tmdbID.Valid {
			m.TmdbID = int(tmdbID.Int64)
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func splitCSV(s string) []string {
	var result []string
	for _, part := range split(s, ",") {
		t := trim(part)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func split(s, sep string) []string {
	// simple split to avoid importing strings in this file
	var parts []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			parts = append(parts, s)
			break
		}
		parts = append(parts, s[:i])
		s = s[i+len(sep):]
	}
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
