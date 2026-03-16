# mahin-cli-v1

Simple CLI with four commands:

1. `mahin hello`
2. `mahin version`
3. `mahin self-update`
4. `mahin help`

mahin hello
mahin version
mahin self-update
mahin movie
mahin help
mahin [any-command] --help
Movie commands

mahin movie config
mahin movie config get <key>
mahin movie config set <key> <value>
mahin movie scan [folder]
mahin movie scan [folder] --verbose
mahin movie ls
mahin movie ls --limit <number>
mahin movie ls --sort title
mahin movie ls --sort rating
mahin movie ls --sort year
mahin movie search "<name>"
mahin movie info <id-or-title>
mahin movie suggest
mahin movie suggest --random
mahin movie suggest --type auto
mahin movie suggest --type movie
mahin movie suggest --type tv
mahin movie move <id-or-title> <destination>
mahin movie undo
mahin movie rename <folder>
mahin movie rename <folder> --dry-run
mahin movie play <id-or-title>
mahin movie stats

## How `self-update` works

`mahin self-update` pulls the latest files from the current cloned git repository.

It runs:

1. `git rev-parse --show-toplevel`
2. `git status --porcelain` (repository must be clean)
3. `git pull --ff-only`

This updates your local clone files from remote.

## Project Structure

```text
mahin-cli-v1/
├── cmd/
│   ├── hello.go
│   ├── root.go
│   ├── update.go      (self-update command)
│   └── version.go
├── updater/
│   └── updater.go
├── version/
│   └── version.go
├── main.go
├── go.mod
└── go.sum
```
