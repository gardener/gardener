/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

The utility functions in this file were copied from the kubernetes/client-go project
https://github.com/kubernetes/client-go/blob/master/util/certificate/certificate_manager.go

Modifications Copyright 2024 SAP SE or an SAP affiliate company and Gardener contributors
*/

package certificate

import (
	"crypto/tls"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// nextRotationDeadline returns a value for the threshold at which the
// current certificate should be rotated, 80%+/-10% of the expiration of the
// certificate.
func nextRotationDeadline(certificate tls.Certificate, validityConfig *gardenletconfigv1alpha1.KubeconfigValidity) time.Time {
	notAfter := certificate.Leaf.NotAfter
	totalDuration := float64(notAfter.Sub(certificate.Leaf.NotBefore))
	return certificate.Leaf.NotBefore.Add(jitteryDuration(totalDuration, validityConfig.AutoRotationJitterPercentageMin, validityConfig.AutoRotationJitterPercentageMax))
}

// jitteryDuration uses some jitter to set the rotation threshold so each gardenlet
// will rotate at approximately 70-90% of the total lifetime of the
// certificate.  With jitter, if a number of gardenlets are added to a garden cluster at
// approximately the same time, they won't all
// try to rotate certificates at the same time for the rest of the lifetime
func jitteryDuration(totalDuration float64, minPercentage, maxPercentage *int32) time.Duration {
	var (
		min = ptr.Deref(minPercentage, 70)
		max = ptr.Deref(maxPercentage, 90)

		minFactor = 1 - float64(min)/100
		maxFactor = float64(max-min) / 100
	)

	return wait.Jitter(time.Duration(totalDuration), maxFactor) - time.Duration(totalDuration*minFactor)
}
