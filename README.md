# mahin-cli-v1

`mahin` is a Go CLI for managing movies and TV shows with local file scanning, TMDb metadata enrichment, and basic file organization tools.

## Highlights

- Scan folders for video files and extract clean titles
- Enrich media with TMDb metadata (ratings, genres, cast, posters)
- Search TMDb directly and save results into local database
- View interactive library lists with sorting and paging
- Move, undo move, rename, and play media files
- Keep a local SQLite database with scan and move history

## Requirements

- Go 1.24+
- Git (required for `self-update`)
- Network access for TMDb-based commands (`scan`, `search`, `info` by title, `suggest`)

## Build and Run

```bash
go test ./...
go build -o mahin .
./mahin --help
```

Optional install to your PATH:

```bash
cp mahin ~/bin/mahin
```

## Quick Start

```bash
# 1) Check current config
mahin movie config

# 2) Scan a folder
mahin movie scan ~/Downloads --verbose

# 3) Browse local library entries
mahin movie ls --sort rating --limit 20

# 4) Fetch details by local database ID
mahin movie info 1

# 5) Fetch by title from TMDb and save (if new)
mahin movie info "Inception"
```

## Command Reference

### Root Commands

- `mahin hello` - print greeting and running version
- `mahin version` - print full build version info
- `mahin self-update` (alias: `mahin update`) - pull latest changes from the current cloned git repository
- `mahin movie ...` - movie/TV management commands

### Movie Commands

- `mahin movie config`
- `mahin movie config get <key>`
- `mahin movie config set <key> <value>`
- `mahin movie scan [folder] [--verbose|-v]`
- `mahin movie ls [--limit N] [--sort title|rating|year]`
- `mahin movie search "<name>"`
- `mahin movie info <id-or-title>`
- `mahin movie suggest [N] [--random] [--type auto|movie|tv]`
- `mahin movie move <id-or-title> <destination>`
- `mahin movie undo`
- `mahin movie rename [folder] [--dry-run]`
- `mahin movie play <id-or-title>`
- `mahin movie stats`

## Behavior Notes

### `movie scan`

- Scans a folder for supported video files.
- Handles files in the scan directory and video files found one level inside subfolders.
- Cleans filename noise (release tags, codecs, etc.) and attempts TMDb match.
- Inserts media into database and stores thumbnails under `~/movie-cli-output/thumbnails/...`.

### `movie ls`

- Shows only local media entries that point to real video files.
- Interactive controls: `N` (next), `P` (previous), `Q` (quit), or enter item number for details.

### `movie search <name>`

- Searches TMDb first.
- Saves the best TMDb result into the local database.
- If result already exists (same TMDb ID), it updates existing data.
- If no TMDb result is found, nothing is saved.
- Stores JSON metadata under:
	- `~/Movies/.mahin-metadata/...` for movie results
	- `~/TVShows/.mahin-metadata/...` for TV results

### `movie info <id-or-title>`

- Numeric input: reads directly from local database by ID.
- Non-numeric input: searches TMDb and saves only if not already present.
- Always writes metadata JSON for title-based TMDb lookups.
- If already in database, it does not create duplicate records.

### `movie move` and `movie undo`

- `movie move` supports direct mode (`<id-or-title> <destination>`) and interactive mode (no args).
- Move actions are logged in `move_history`.
- `movie undo` reverts the most recent non-undone move.

### `movie rename`

- Renames video files in one folder (non-recursive) to clean names like `Title (Year).ext`.
- `--dry-run` previews changes without renaming.

### `movie suggest`

- Uses TMDb recommendations/trending data.
- `--type auto` chooses movie or TV based on your library counts.
- `--random` returns random picks from trending content.

### `movie stats`

- Shows totals, top genres, and average ratings from database records.

## Configuration

Available keys:

- `movies_dir` (default: `~/Movies`)
- `tv_dir` (default: `~/TVShows`)
- `archive_dir` (default: `~/Archive`)
- `scan_dir` (default: `~/Downloads`)
- `tmdb_api_key` (default value is seeded)
- `page_size` (default: `20`)

Examples:

```bash
mahin movie config
mahin movie config get scan_dir
mahin movie config set scan_dir ~/Media
mahin movie config set page_size 30
```

## Data and Storage Layout

On first run, the CLI creates:

- Database: `~/movie-cli-output/mahin.db`
- JSON folders: `~/movie-cli-output/json/movie`, `~/movie-cli-output/json/tv`, `~/movie-cli-output/json/history`
- Thumbnails: `~/movie-cli-output/thumbnails`

Main SQLite tables:

- `media`
- `move_history`
- `config`
- `scan_history`
- `tags`

## Self-Update Details

`mahin self-update` performs:

1. `git rev-parse --show-toplevel`
2. `git status --porcelain` (must be clean)
3. `git pull --ff-only`

If the repository has local changes, update is blocked until you commit or stash them.

## Project Layout

```text
mahin-cli-v1/
	cmd/          # Cobra commands
	cleaner/      # filename cleaning/parsing
	db/           # SQLite schema and queries
	tmdb/         # TMDb API client
	updater/      # self-update logic
	version/      # build/version metadata
	main.go
	go.mod
	go.sum
```
