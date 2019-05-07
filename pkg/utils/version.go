// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
)

// CompareVersions returns true if the constraint <version1> compared by <operator> to <version2>
// returns true, and false otherwise.
// The comparison is based on semantic versions, i.e. <version1> and <version2> will be converted
// if needed.
func CompareVersions(version1, operator, version2 string) (bool, error) {
	var (
		v1 = normalizeVersion(version1)
		v2 = normalizeVersion(version2)
	)

	return CheckVersionMeetsConstraint(v1, fmt.Sprintf("%s %s", operator, v2))
}

func normalizeVersion(version string) string {
	v := strings.Replace(version, "v", "", -1)
	idx := strings.IndexAny(v, "-+")
	if idx != -1 {
		v = v[:idx]
	}
	return v
}

// CheckVersionMeetsConstraint returns true if the <version> meets the <constraint>.
func CheckVersionMeetsConstraint(version, constraint string) (bool, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, err
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return false, err
	}

	return c.Check(v), nil
}
