// Package config holds the static constants that identify this CLI's
// GitHub repository and binary name.
//
// This is the ONLY file you need to change when forking or renaming the CLI.
package config

const (
	// GitHubOwner is your GitHub username or organisation.
	GitHubOwner = "Ab-mahin"

	// GitHubRepo is the name of the repository that publishes releases.
	GitHubRepo = "mahin-cli-v1"

	// BinaryName is the base name used when naming release assets.
	// Release assets must follow: <BinaryName>-<os>-<arch>[.exe]
	// e.g.  mahin-linux-amd64
	//       mahin-darwin-arm64
	//       mahin-windows-amd64.exe
	BinaryName = "mahin"
)