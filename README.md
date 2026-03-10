mahin-cli-v1/
│
├── main.go                   Detects --internal-updater flag -> routes to child or cobra
│
├── config/
│   └── config.go             3 constants: GitHubOwner, GitHubRepo, BinaryName
│
├── version/
│   └── version.go            3 build-time vars (Version/Commit/BuildDate) + Full()/Short()
│
├── cmd/                      One file per command, zero business logic
│   ├── root.go               Registers all subcommands, exposes Execute()
│   ├── hello.go              `mahin hello`
│   ├── version.go            `mahin version`
│   └── update.go             `mahin update` -> calls updater.Run()
│
└── updater/                  All update logic, split by responsibility
    ├── updater.go            Orchestrator: parent flow + child flow, semver compare
    ├── github.go             GitHub API: fetch latest release, find asset URLs
    ├── platform.go           OS/arch detection at runtime -> builds asset filename
    ├── download.go           Download file to disk + SHA-256 checksum verify
    ├── replace.go            Verify binary + atomic swap of old -> new executable
    ├── process.go            Builds the OS-specific detached child command
    ├── process_unix.go       Unix: no extra flags needed
    └── process_windows.go    Windows: CREATE_NEW_PROCESS_GROUP to detach child