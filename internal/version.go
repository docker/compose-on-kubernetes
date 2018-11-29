package internal

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

const (
	unknownGitCommit = "unknown-commit"
)

// Default build-time variable.
// These values are overridden via ldflags
var (
	Version   = "unknown-version"
	GitCommit = unknownGitCommit
	BuildTime = "unknown-buildtime"
)

// FullVersion returns the completion version informations of the project
func FullVersion() string {
	infos := []string{
		fmt.Sprintf("Version:    %s", Version),
		fmt.Sprintf("Git commit: %s", GitCommit),
		fmt.Sprintf("OS/Arch:    %s/%s", runtime.GOOS, runtime.GOARCH),
	}

	if t, err := time.Parse(time.RFC3339Nano, BuildTime); err == nil {
		infos = append(infos, fmt.Sprintf("Built:      %s", t.Format(time.ANSIC)))
	}

	return strings.Join(infos, "\n")
}
