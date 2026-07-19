package releaseinfo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = ""
)

var (
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	commitPattern  = regexp.MustCompile(`^[a-f0-9]{40}$`)
)

// Info identifies one immutable application build. Values are injected by the
// release build and intentionally contain no deployment credentials or URLs.
type Info struct {
	Version string
	Commit  string
	BuiltAt time.Time
}

// PublicInfo is safe to expose from unauthenticated health endpoints.
type PublicInfo struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

func Current() Info {
	info := Info{
		Version: strings.TrimSpace(Version),
		Commit:  strings.ToLower(strings.TrimSpace(Commit)),
	}
	if value := strings.TrimSpace(BuiltAt); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			info.BuiltAt = parsed.UTC()
		}
	}
	return info
}

func (info Info) Validate(requireImmutable bool) error {
	if !versionPattern.MatchString(info.Version) {
		return fmt.Errorf("release version %q is invalid", info.Version)
	}
	if info.Commit != "unknown" && !commitPattern.MatchString(info.Commit) {
		return fmt.Errorf("release commit %q must be a full lowercase Git SHA", info.Commit)
	}
	if requireImmutable {
		if info.Version == "dev" {
			return errors.New("production release version cannot be dev")
		}
		if !commitPattern.MatchString(info.Commit) {
			return errors.New("production release requires a full Git commit SHA")
		}
		if info.BuiltAt.IsZero() {
			return errors.New("production release requires a valid RFC3339 build timestamp")
		}
	}
	return nil
}

func (info Info) Public() PublicInfo {
	revision := info.Commit
	if len(revision) > 12 {
		revision = revision[:12]
	}
	return PublicInfo{Version: info.Version, Revision: revision}
}
