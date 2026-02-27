// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	// ConstraintK8sLess131 is a version constraint for versions < 1.31.
	ConstraintK8sLess131 *Constraints
	// ConstraintK8sEqual131 is a version constraint for versions == 1.31.
	ConstraintK8sEqual131 *Constraints
	// ConstraintK8sGreaterEqual131 is a version constraint for versions >= 1.31.
	ConstraintK8sGreaterEqual131 *Constraints
	// ConstraintK8sLess132 is a version constraint for versions < 1.32.
	ConstraintK8sLess132 *Constraints
	// ConstraintK8sGreaterEqual132 is a version constraint for versions >= 1.32.
	ConstraintK8sGreaterEqual132 *Constraints
	// ConstraintK8sLess133 is a version constraint for versions < 1.33.
	ConstraintK8sLess133 *Constraints
	// ConstraintK8sGreaterEqual133 is a version constraint for versions >= 1.33.
	ConstraintK8sGreaterEqual133 *Constraints
	// ConstraintK8sLess134 is a version constraint for versions < 1.34.
	ConstraintK8sLess134 *Constraints
	// ConstraintK8sGreaterEqual134 is a version constraint for versions >= 1.34.
	ConstraintK8sGreaterEqual134 *Constraints
	// ConstraintK8sLess135 is a version constraint for versions < 1.35.
	ConstraintK8sLess135 *Constraints
	// ConstraintK8sGreaterEqual135 is a version constraint for versions >= 1.35.
	ConstraintK8sGreaterEqual135 *Constraints
)

func init() {
	ConstraintK8sLess131 = MustNewConstraint("< 1.31-0")
	ConstraintK8sEqual131 = MustNewConstraint("~ 1.31.x-0")
	ConstraintK8sGreaterEqual131 = MustNewConstraint(">= 1.31-0")
	ConstraintK8sLess132 = MustNewConstraint("< 1.32-0")
	ConstraintK8sGreaterEqual132 = MustNewConstraint(">= 1.32-0")
	ConstraintK8sLess133 = MustNewConstraint("< 1.33-0")
	ConstraintK8sGreaterEqual133 = MustNewConstraint(">= 1.33-0")
	ConstraintK8sLess134 = MustNewConstraint("< 1.34-0")
	ConstraintK8sGreaterEqual134 = MustNewConstraint(">= 1.34-0")
	ConstraintK8sLess135 = MustNewConstraint("< 1.35-0")
	ConstraintK8sGreaterEqual135 = MustNewConstraint(">= 1.35-0")
}

// CompareVersions returns true if the constraint <version1> compared by <operator> to <version2>
// returns true, and false otherwise.
// The comparison is based on semantic versions, i.e. <version1> and <version2> will be converted
// if needed.
func CompareVersions(version1, operator, version2 string) (bool, error) {
	var (
		v1 = Normalize(version1)
		v2 = Normalize(version2)
	)

	return CheckVersionMeetsConstraint(v1, fmt.Sprintf("%s %s", operator, v2))
}

// CheckVersionMeetsConstraint returns true if the <version> meets the <constraint>.
func CheckVersionMeetsConstraint(version, constraint string) (bool, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, err
	}

	v, err := semver.NewVersion(Normalize(version))
	if err != nil {
		return false, err
	}

	return c.Check(v), nil
}

// Normalize returns the normalized version string by removing the leading 'v' and any suffixes like '-rc1', '-beta2', etc.
func Normalize(version string) string {
	v := strings.ReplaceAll(version, "v", "")
	idx := strings.IndexAny(v, "-+~")
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

// CheckIfMinorVersionUpdate checks if the new version is a minor version update to the old version.
func CheckIfMinorVersionUpdate(old, new string) (bool, error) {
	oldVersion, err := semver.NewVersion(Normalize(old))
	if err != nil {
		return false, fmt.Errorf("failed to parse old version %s: %w", old, err)
	}
	newVersion, err := semver.NewVersion(Normalize(new))
	if err != nil {
		return false, fmt.Errorf("failed to parse new version %s: %w", new, err)
	}

	return oldVersion.Minor() != newVersion.Minor(), nil
}
