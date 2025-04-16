// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/test/e2e/gardener"
)

// ItShouldAnnotateSeed sets the given annotation within the seed metadata to the specified value and patches the seed object
func ItShouldAnnotateSeed(s *SeedContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate Seed", func(ctx SpecContext) {
		patch := client.MergeFrom(s.Seed.DeepCopy())

		for key, value := range annotations {
			s.Log.Info("Setting annotation", "annotation", key, "value", value)
			metav1.SetMetaDataAnnotation(&s.Seed.ObjectMeta, key, value)
		}

		Eventually(ctx, func() error {
			return s.GardenClient.Patch(ctx, s.Seed, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
