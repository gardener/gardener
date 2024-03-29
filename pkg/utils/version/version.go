// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	// ConstraintK8sGreaterEqual126 is a version constraint for versions >= 1.26.
	ConstraintK8sGreaterEqual126 *semver.Constraints
	// ConstraintK8sLess126 is a version constraint for versions < 1.26.
	ConstraintK8sLess126 *semver.Constraints
	// ConstraintK8sGreaterEqual127 is a version constraint for versions >= 1.27.
	ConstraintK8sGreaterEqual127 *semver.Constraints
	// ConstraintK8sLess127 is a version constraint for versions < 1.27.
	ConstraintK8sLess127 *semver.Constraints
	// ConstraintK8sGreaterEqual128 is a version constraint for versions >= 1.28.
	ConstraintK8sGreaterEqual128 *semver.Constraints
	// ConstraintK8sEqual128 is a version constraint for versions == 1.28.
	ConstraintK8sEqual128 *semver.Constraints
	// ConstraintK8sGreaterEqual129 is a version constraint for versions >= 1.29.
	ConstraintK8sGreaterEqual129 *semver.Constraints
)

func init() {
	var err error
	ConstraintK8sGreaterEqual126, err = semver.NewConstraint(">= 1.26-0")
	utilruntime.Must(err)
	ConstraintK8sLess126, err = semver.NewConstraint("< 1.26-0")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual127, err = semver.NewConstraint(">= 1.27-0")
	utilruntime.Must(err)
	ConstraintK8sLess127, err = semver.NewConstraint("< 1.27-0")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual128, err = semver.NewConstraint(">= 1.28-0")
	utilruntime.Must(err)
	ConstraintK8sEqual128, err = semver.NewConstraint("~ 1.28.x-0")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual129, err = semver.NewConstraint(">= 1.29-0")
	utilruntime.Must(err)
}

// CompareVersions returns true if the constraint <version1> compared by <operator> to <version2>
// returns true, and false otherwise.
// The comparison is based on semantic versions, i.e. <version1> and <version2> will be converted
// if needed.
func CompareVersions(version1, operator, version2 string) (bool, error) {
	var (
		v1 = normalize(version1)
		v2 = normalize(version2)
	)

	return CheckVersionMeetsConstraint(v1, fmt.Sprintf("%s %s", operator, v2))
}

// CheckVersionMeetsConstraint returns true if the <version> meets the <constraint>.
func CheckVersionMeetsConstraint(version, constraint string) (bool, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, err
	}

	v, err := semver.NewVersion(normalize(version))
	if err != nil {
		return false, err
	}

	return c.Check(v), nil
}

func normalize(version string) string {
	v := strings.Replace(version, "v", "", -1)
	idx := strings.IndexAny(v, "-+")
	if idx != -1 {
		v = v[:idx]
	}

	return v
}

// VersionRange represents a version range of type [AddedInVersion, RemovedInVersion).
type VersionRange struct {
	AddedInVersion   string
	RemovedInVersion string
}

// Contains returns true if the range contains the given version, false otherwise.
// The range contains the given version only if it's greater or equal than AddedInVersion (always true if AddedInVersion is empty),
// and less than RemovedInVersion (always true if RemovedInVersion is empty).
func (r *VersionRange) Contains(version string) (bool, error) {
	var constraint string

	switch {
	case r.AddedInVersion != "" && r.RemovedInVersion == "":
		constraint = ">= " + r.AddedInVersion
	case r.AddedInVersion == "" && r.RemovedInVersion != "":
		constraint = "< " + r.RemovedInVersion
	case r.AddedInVersion != "" && r.RemovedInVersion != "":
		constraint = fmt.Sprintf(">= %s, < %s", r.AddedInVersion, r.RemovedInVersion)
	default:
		constraint = "*"
	}

	return CheckVersionMeetsConstraint(version, constraint)
}

// SupportedVersionRange returns the supported version range for the given API.
func (r *VersionRange) SupportedVersionRange() string {
	switch {
	case r.AddedInVersion != "" && r.RemovedInVersion == "":
		return "versions >= " + r.AddedInVersion
	case r.AddedInVersion == "" && r.RemovedInVersion != "":
		return "versions < " + r.RemovedInVersion
	case r.AddedInVersion != "" && r.RemovedInVersion != "":
		return fmt.Sprintf("versions >= %s, < %s", r.AddedInVersion, r.RemovedInVersion)
	default:
		return "all kubernetes versions"
	}
}
