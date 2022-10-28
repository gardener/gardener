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

package version

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	// ConstraintK8sGreaterEqual118 is a version constraint for versions >= 1.18.
	ConstraintK8sGreaterEqual118 *semver.Constraints
	// ConstraintK8sEqual118 is a version constraint for versions == 1.18.
	ConstraintK8sEqual118 *semver.Constraints
	// ConstraintK8sGreaterEqual119 is a version constraint for versions >= 1.19.
	ConstraintK8sGreaterEqual119 *semver.Constraints
	// ConstraintK8sLess119 is a version constraint for versions < 1.19.
	ConstraintK8sLess119 *semver.Constraints
	// ConstraintK8sLess120 is a version constraint for versions < 1.20.
	ConstraintK8sLess120 *semver.Constraints
	// ConstraintK8sEqual119 is a version constraint for versions == 1.19.
	ConstraintK8sEqual119 *semver.Constraints
	// ConstraintK8sGreaterEqual120 is a version constraint for versions >= 1.20.
	ConstraintK8sGreaterEqual120 *semver.Constraints
	// ConstraintK8sEqual120 is a version constraint for versions == 1.20.
	ConstraintK8sEqual120 *semver.Constraints
	// ConstraintK8sLessEqual121 is a version constraint for versions <= 1.21.
	ConstraintK8sLessEqual121 *semver.Constraints
	// ConstraintK8sEqual121 is a version constraint for versions == 1.21.
	ConstraintK8sEqual121 *semver.Constraints
	// ConstraintK8sGreaterEqual121 is a version constraint for versions >= 1.21.
	ConstraintK8sGreaterEqual121 *semver.Constraints
	// ConstraintK8sLessEqual122 is a version constraint for versions <= 1.22.
	ConstraintK8sLessEqual122 *semver.Constraints
	// ConstraintK8sEqual122 is a version constraint for versions == 1.22.
	ConstraintK8sEqual122 *semver.Constraints
	// ConstraintK8sGreaterEqual122 is a version constraint for versions >= 1.22.
	ConstraintK8sGreaterEqual122 *semver.Constraints
	// ConstraintK8sEqual123 is a version constraint for versions == 1.23.
	ConstraintK8sEqual123 *semver.Constraints
	// ConstraintK8sGreaterEqual123 is a version constraint for versions >= 1.23.
	ConstraintK8sGreaterEqual123 *semver.Constraints
	// ConstraintK8sEqual124 is a version constraint for versions == 1.24.
	ConstraintK8sEqual124 *semver.Constraints
	// ConstraintK8sLess124 is a version constraint for versions < 1.24.
	ConstraintK8sLess124 *semver.Constraints
	// ConstraintK8sGreaterEqual125 is a version constraint for versions >= 1.25.
	ConstraintK8sGreaterEqual125 *semver.Constraints
	// ConstraintK8sLess125 is a version constraint for versions < 1.25.
	ConstraintK8sLess125 *semver.Constraints
	// ConstraintK8sGreaterEqual126 is a version constraint for versions >= 1.26.
	ConstraintK8sGreaterEqual126 *semver.Constraints
)

func init() {
	var err error

	ConstraintK8sGreaterEqual118, err = semver.NewConstraint(">= 1.18")
	utilruntime.Must(err)
	ConstraintK8sEqual118, err = semver.NewConstraint("1.18.x")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual119, err = semver.NewConstraint(">= 1.19")
	utilruntime.Must(err)
	ConstraintK8sEqual119, err = semver.NewConstraint("1.19.x")
	utilruntime.Must(err)
	ConstraintK8sLess119, err = semver.NewConstraint("< 1.19")
	utilruntime.Must(err)
	ConstraintK8sLess120, err = semver.NewConstraint("< 1.20")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual120, err = semver.NewConstraint(">= 1.20")
	utilruntime.Must(err)
	ConstraintK8sEqual120, err = semver.NewConstraint("1.20.x")
	utilruntime.Must(err)
	ConstraintK8sLessEqual121, err = semver.NewConstraint("<= 1.21.x")
	utilruntime.Must(err)
	ConstraintK8sEqual121, err = semver.NewConstraint("1.21.x")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual121, err = semver.NewConstraint(">= 1.21")
	utilruntime.Must(err)
	ConstraintK8sLessEqual122, err = semver.NewConstraint("<= 1.22.x")
	utilruntime.Must(err)
	ConstraintK8sEqual122, err = semver.NewConstraint("1.22.x")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual122, err = semver.NewConstraint(">= 1.22")
	utilruntime.Must(err)
	ConstraintK8sEqual123, err = semver.NewConstraint("1.23.x")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual123, err = semver.NewConstraint(">= 1.23")
	utilruntime.Must(err)
	ConstraintK8sEqual124, err = semver.NewConstraint("1.24.x")
	utilruntime.Must(err)
	ConstraintK8sLess124, err = semver.NewConstraint("< 1.24")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual125, err = semver.NewConstraint(">= 1.25")
	utilruntime.Must(err)
	ConstraintK8sLess125, err = semver.NewConstraint("< 1.25")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual126, err = semver.NewConstraint(">= 1.26")
	utilruntime.Must(err)
}

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

	v, err := semver.NewVersion(normalizeVersion(version))
	if err != nil {
		return false, err
	}

	return c.Check(v), nil
}
