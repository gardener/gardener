// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// Builder is an object that builds Seed objects.
type Builder struct {
	seedObjectFunc func(context.Context) (*gardencorev1beta1.Seed, error)
}

// Seed is an object containing information about a Seed cluster.
type Seed struct {
	info      atomic.Value
	infoMutex sync.Mutex

	KubernetesVersion *semver.Version
}
