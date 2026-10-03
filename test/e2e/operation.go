// SPDX-FileCopyrightText: Contributors to the Gardener project
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

// EventuallyNotHaveOperationAnnotation waits for the gardener operation annotation to be removed from the given object.
// It must be called from within a spec, e.g.:
//
//	It("Should start the reconciliation", func(ctx SpecContext) {
//	  EventuallyNotHaveOperationAnnotation(ctx, tc.GardenKomega, tc.Seed)
//	}, SpecTimeout(2*time.Minute))
func EventuallyNotHaveOperationAnnotation(ctx context.Context, komega komega.Komega, obj client.Object) {
	GinkgoHelper()

	Eventually(ctx, komega.Object(obj)).WithPolling(2 * time.Second).Should(
		HaveField("ObjectMeta.Annotations", Not(HaveKey(v1beta1constants.GardenerOperation))))
}

// ItShouldEventuallyNotHaveOperationAnnotation checks if the given object does not have the gardener operation annotation set
func ItShouldEventuallyNotHaveOperationAnnotation(komega komega.Komega, obj client.Object) {
	GinkgoHelper()
	It("Should not have operation annotation", func(ctx SpecContext) {
		EventuallyNotHaveOperationAnnotation(ctx, komega, obj)
	}, SpecTimeout(2*time.Minute))
}
