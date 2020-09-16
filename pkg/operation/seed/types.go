// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	corev1 "k8s.io/api/core/v1"
)

// Builder is an object that builds Seed objects.
type Builder struct {
	seedObjectFunc func() (*gardencorev1beta1.Seed, error)
	seedSecretFunc func(*corev1.SecretReference) (*corev1.Secret, error)
}

// Seed is an object containing information about a Seed cluster.
type Seed struct {
	Info                           *gardencorev1beta1.Seed
	Secret                         *corev1.Secret
	LoadBalancerServiceAnnotations map[string]string
}
