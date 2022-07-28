// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"

	"github.com/Masterminds/semver"
	"k8s.io/apimachinery/pkg/version"
)

// VersionMajorMinor extracts and returns the major and the minor part of the given version (input must be a semantic version).
func VersionMajorMinor(version string) (string, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", fmt.Errorf("Invalid version string '%s': %w", version, err)
	}
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor()), nil
}

// VersionInfo converts the given version string to version.Info (input must be a semantic version).
func VersionInfo(vs string) (*version.Info, error) {
	v, err := semver.NewVersion(vs)
	if err != nil {
		return nil, fmt.Errorf("Invalid version string '%s': %w", vs, err)
	}
	return &version.Info{
		Major:      fmt.Sprintf("%d", v.Major()),
		Minor:      fmt.Sprintf("%d", v.Minor()),
		GitVersion: fmt.Sprintf("v%d.%d.%d", v.Major(), v.Minor(), v.Patch()),
	}, nil
}
