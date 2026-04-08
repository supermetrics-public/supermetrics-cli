package buildcfg

// Build-time values injected via ldflags. Example:
//
//	go build -ldflags "-X .../internal/buildcfg.Version=v1.0.0 -X .../internal/buildcfg.OAuthClientID=..."
//
// During development, the Makefile reads values from .env and injects them automatically.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"

	OAuthClientID string
	OAuthScopes   string

	DefaultDomain = "supermetrics.com"
)
