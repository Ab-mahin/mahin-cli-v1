# mahin-cli-v1

A self-updating CLI tool with GitHub Releases integration.

## Project Structure

```text
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
```
## Available Commands

```bash
mahin hello    # Print a greeting
mahin version  # Show current version, commit, and build date
mahin update   # Check GitHub for a newer release and self-update
```

## Verification

### 1. Static Analysis

```bash
# Compile all packages
go build -v ./...

# Run static analysis
go vet ./...

# Build the binary
go build -o mahin .
```

### 2. Test All Commands

```bash
# Test version command
./mahin version

# Test hello command
./mahin hello

# Test update help
./mahin update --help
```

### 3. GitHub API Validation

```bash
# Check if release endpoint is accessible
curl -s https://api.github.com/repos/Ab-mahin/mahin-cli-v1/releases/latest | python3 -c "import sys, json; r = json.load(sys.stdin); print(f'Tag: {r.get(\"tag_name\")}'); print(f'Assets: {len(r.get(\"assets\", []))} files')"
```

### 4. Platform Detection

```bash
# Check OS and architecture
go env GOOS GOARCH

# Verify expected asset name matches
echo "mahin-$(go env GOOS)-$(go env GOARCH)"
```

### 5. Checksum Verification

```bash
# Generate checksum for a binary
shasum -a 256 dist/mahin-darwin-arm64

# Verify checksum matches
shasum -c dist/mahin-darwin-arm64.sha256
```

### 6. End-to-End Update Test

```bash
# Build an older version binary
go build -ldflags "-X github.com/mahin/mahin-cli-v1/version.Version=v0.0.1" -o /tmp/mahin-test .

# Check initial version
/tmp/mahin-test version

# Run update
/tmp/mahin-test update

# Wait for child process to complete
sleep 10

# Verify updated version
/tmp/mahin-test version
```

### 7. Complete Verification Flow

```bash
echo "=== BEFORE UPDATE ==="
./mahin version

echo ""
echo "=== RUNNING UPDATE ==="
./mahin update

echo ""
echo "=== WAITING FOR CHILD PROCESS ==="
sleep 8

echo ""
echo "=== AFTER UPDATE ==="
./mahin version

echo ""
echo "=== TEST ALL COMMANDS ==="
./mahin hello
./mahin version
./mahin update  # Should show "Already up to date"
```

## Publishing a Release

### Prerequisites

```bash
# Install GitHub CLI (macOS)
brew install gh

# Authenticate
gh auth login
```

### Build Release Assets

```bash
# Set version and build metadata
VERSION="v0.0.2"
BUILD_DATE="$(date -u +%Y-%m-%d)"

# Create dist directory
mkdir -p dist

# Build for macOS ARM64
GOOS=darwin GOARCH=arm64 go build -ldflags "\
  -X github.com/mahin/mahin-cli-v1/version.Version=${VERSION} \
  -X github.com/mahin/mahin-cli-v1/version.Commit=$(git rev-parse --short HEAD) \
  -X github.com/mahin/mahin-cli-v1/version.BuildDate=${BUILD_DATE}" \
  -o dist/mahin-darwin-arm64 .

# Generate checksum
shasum -a 256 dist/mahin-darwin-arm64 > dist/mahin-darwin-arm64.sha256

# Verify binary
./dist/mahin-darwin-arm64 version
```

### Build for Multiple Platforms

```bash
# macOS Intel
GOOS=darwin GOARCH=amd64 go build -ldflags "..." -o dist/mahin-darwin-amd64 .
shasum -a 256 dist/mahin-darwin-amd64 > dist/mahin-darwin-amd64.sha256

# Linux AMD64
GOOS=linux GOARCH=amd64 go build -ldflags "..." -o dist/mahin-linux-amd64 .
shasum -a 256 dist/mahin-linux-amd64 > dist/mahin-linux-amd64.sha256

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags "..." -o dist/mahin-linux-arm64 .
shasum -a 256 dist/mahin-linux-arm64 > dist/mahin-linux-arm64.sha256

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -ldflags "..." -o dist/mahin-windows-amd64.exe .
shasum -a 256 dist/mahin-windows-amd64.exe > dist/mahin-windows-amd64.exe.sha256
```

### Create and Publish Release

```bash
# Create release with assets
gh release create v0.0.2 \
  dist/mahin-darwin-arm64 \
  dist/mahin-darwin-arm64.sha256 \
  dist/mahin-darwin-amd64 \
  dist/mahin-darwin-amd64.sha256 \
  dist/mahin-linux-amd64 \
  dist/mahin-linux-amd64.sha256 \
  dist/mahin-linux-arm64 \
  dist/mahin-linux-arm64.sha256 \
  dist/mahin-windows-amd64.exe \
  dist/mahin-windows-amd64.exe.sha256 \
  --title "v0.0.2" \
  --notes "Release notes here"

# Verify release is published
curl -s https://api.github.com/repos/Ab-mahin/mahin-cli-v1/releases/latest | grep tag_name
```

## Configuration

Edit `config/config.go` to customize:

```go
const (
    GitHubOwner = "Ab-mahin"           // Your GitHub username
    GitHubRepo  = "mahin-cli-v1"       // Your repository name
    BinaryName  = "mahin"              // Base name for assets
)
```

## How Self-Update Works

1. **Parent Process** (`mahin update`):
   - Checks current version
   - Queries GitHub Releases API for latest version
   - Compares versions using semver
   - Detects OS/architecture
   - Finds matching binary asset
   - Spawns child updater process
   - Exits immediately (releases file lock)

2. **Child Process** (`mahin --internal-updater ...`):
   - Waits for parent to exit
   - Downloads new binary to temp directory
   - Downloads and verifies SHA-256 checksum
   - Verifies new binary runs correctly
   - Atomically replaces old binary with new one
   - Cleans up temp files
   - Exits

This approach works around OS file-locking (especially Windows) by ensuring the running binary is not active when it gets replaced.

