// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// EventuallyNotHaveOperationAnnotation checks if the given object does not have the gardener operation annotation set.
func EventuallyNotHaveOperationAnnotation(ctx context.Context, komega komega.Komega, obj client.Object) {
	GinkgoHelper()

	Eventually(ctx, komega.Object(obj)).WithPolling(2 * time.Second).Should(
		HaveField("ObjectMeta.Annotations", Not(HaveKey(v1beta1constants.GardenerOperation))))
}

// ItShouldEventuallyNotHaveOperationAnnotation checks if the given object does not have the gardener operation annotation set.
//
// Deprecated: Instead, use the `EventuallyNotHaveOperationAnnotation` function. For more details, see https://github.com/gardener/gardener/issues/13134.
func ItShouldEventuallyNotHaveOperationAnnotation(komega komega.Komega, obj client.Object) {
	GinkgoHelper()
	It("Should not have operation annotation", func(ctx SpecContext) {
		EventuallyNotHaveOperationAnnotation(ctx, komega, obj)
	}, SpecTimeout(time.Minute))
}
