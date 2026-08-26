// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package security

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Object is a security object resource.
type Object interface {
	metav1.Object
}
