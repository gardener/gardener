// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/version"
)

// VersionMajorMinor extracts and returns the major and the minor part of the given version (input must be a semantic version).
func VersionMajorMinor(version string) (string, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", fmt.Errorf("invalid version string '%s': %w", version, err)
	}
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor()), nil
}

// VersionInfo converts the given version string to version.Info (input must be a semantic version).
func VersionInfo(vs string) (*version.Info, error) {
	v, err := semver.NewVersion(vs)
	if err != nil {
		return nil, fmt.Errorf("invalid version string '%s': %w", vs, err)
	}
	return &version.Info{
		Major:      fmt.Sprintf("%d", v.Major()),
		Minor:      fmt.Sprintf("%d", v.Minor()),
		GitVersion: fmt.Sprintf("v%d.%d.%d", v.Major(), v.Minor(), v.Patch()),
	}, nil
}
