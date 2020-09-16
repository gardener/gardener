// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"
	"runtime"
	"strings"

	apimachineryversion "k8s.io/apimachinery/pkg/version"
)

var (
	gitVersion   = "v0.0.0-dev"
	gitCommit    string
	gitTreeState string
	buildDate    = "1970-01-01T00:00:00Z"

	version *apimachineryversion.Info
)

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
// These variables typically come from -ldflags settings and in
// their absence fallback to the settings in pkg/version/version.go
func Get() apimachineryversion.Info {
	return *version
}

func init() {
	var (
		versionParts = strings.Split(gitVersion, ".")
		gitMajor     string
		gitMinor     string
	)

	if len(versionParts) >= 2 {
		gitMajor = strings.TrimPrefix(versionParts[0], "v")
		gitMinor = versionParts[1]
	}

	version = &apimachineryversion.Info{
		Major:        gitMajor,
		Minor:        gitMinor,
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
