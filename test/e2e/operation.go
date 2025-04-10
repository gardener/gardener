// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ItShouldEventuallyNotHaveOperationAnnotation checks if the given object does not have the gardener operation annotation set
func ItShouldEventuallyNotHaveOperationAnnotation(komega komega.Komega, obj client.Object) {
	GinkgoHelper()
	It("Should not have operation annotation", func(ctx SpecContext) {
		Eventually(ctx, komega.Object(obj)).WithPolling(2 * time.Second).Should(
			HaveField("ObjectMeta.Annotations", Not(HaveKey(v1beta1constants.GardenerOperation))))
	}, SpecTimeout(time.Minute))
}
