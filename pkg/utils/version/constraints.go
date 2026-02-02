// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"github.com/Masterminds/semver/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Constraints is a wrapper around semver.Constraints.
type Constraints struct {
	*semver.Constraints
}

// CheckVersion checks whether the given version string satisfies the constraints.
// Please ensure the passed version is a valid semantic version (errors related to parsing the version will result in `false` being returned - the error is omitted).
func (c *Constraints) CheckVersion(version string) bool {
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	return c.Check(v)
}

// MustNewConstraint creates a new Constraints object from the given constraint string.
// The function panics if the passed constraint is invalid.
func MustNewConstraint(constraintStr string) *Constraints {
	constraint, err := semver.NewConstraint(constraintStr)
	utilruntime.Must(err)
	return &Constraints{Constraints: constraint}
}
