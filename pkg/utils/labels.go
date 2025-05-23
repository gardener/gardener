// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// MustNewRequirement creates a labels.Requirement with the given values and panics if there is an error.
func MustNewRequirement(key string, op selection.Operator, vals ...string) labels.Requirement {
	req, err := labels.NewRequirement(key, op, vals)
	utilruntime.Must(err)
	return *req
}
