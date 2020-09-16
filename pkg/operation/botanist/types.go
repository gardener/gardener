// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"sync"

	"github.com/gardener/gardener/pkg/operation"

	corev1 "k8s.io/api/core/v1"
)

// Botanist is a struct which has methods that perform cloud-independent operations for a Shoot cluster.
type Botanist struct {
	*operation.Operation
	DefaultDomainSecret *corev1.Secret
	mutex               sync.RWMutex
}
