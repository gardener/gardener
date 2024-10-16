// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	// ConstraintK8sGreaterEqual128 is a version constraint for versions >= 1.28.
	ConstraintK8sGreaterEqual128 *semver.Constraints
	// ConstraintK8sGreaterEqual129 is a version constraint for versions >= 1.29.
	ConstraintK8sGreaterEqual129 *semver.Constraints
	// ConstraintK8sLess130 is a version constraint for versions < 1.30.
	ConstraintK8sLess130 *semver.Constraints
	// ConstraintK8sGreaterEqual130 is a version constraint for versions >= 1.30.
	ConstraintK8sGreaterEqual130 *semver.Constraints
	// ConstraintK8sLess131 is a version constraint for versions < 1.31.
	ConstraintK8sLess131 *semver.Constraints
	// ConstraintK8sGreaterEqual131 is a version constraint for versions >= 1.31.
	ConstraintK8sGreaterEqual131 *semver.Constraints
	// ConstraintK8sLess132 is a version constraint for versions < 1.32.
	ConstraintK8sLess132 *semver.Constraints
	// ConstraintK8sGreaterEqual132 is a version constraint for versions >= 1.32.
	ConstraintK8sGreaterEqual132 *semver.Constraints
)

func init() {
	var err error
	ConstraintK8sGreaterEqual128, err = semver.NewConstraint(">= 1.28-0")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual129, err = semver.NewConstraint(">= 1.29-0")
	utilruntime.Must(err)
	ConstraintK8sLess130, err = semver.NewConstraint("< 1.30-0")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual130, err = semver.NewConstraint(">= 1.30-0")
	utilruntime.Must(err)
	ConstraintK8sLess131, err = semver.NewConstraint("< 1.31-0")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual131, err = semver.NewConstraint(">= 1.31-0")
	utilruntime.Must(err)
	ConstraintK8sLess132, err = semver.NewConstraint("< 1.32-0")
	utilruntime.Must(err)
	ConstraintK8sGreaterEqual132, err = semver.NewConstraint(">= 1.32-0")
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
