package version

// Version identifies the running build. It is overridden at build time via
// -ldflags "-X github.com/Be1zebub/sing-box-tray-runner/internal/version.Version=vX.Y.Z"
// (see scripts/build.sh, scripts/build.ps1, and .github/workflows/release.yml).
// The default "dev" means "no release tag embedded" — the self-updater treats
// that as always older than the latest GitHub release.
var Version = "dev"
