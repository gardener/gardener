// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/care"
)

var _ = Describe("Add", func() {
	Describe("#MapManagedResourceToExtension", func() {
		var (
			ctx            context.Context
			careReconciler *Reconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			careReconciler = &Reconciler{
				GardenNamespace: "garden",
			}
		})

		DescribeTable("map a managed resource to an extension",
			func(namespace, name, expectedExtension string) {
				managedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
				}

				result := careReconciler.MapManagedResourceToExtension(ctx, managedResource)
				if expectedExtension != "" {
					Expect(result).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: expectedExtension}}))
				} else {
					Expect(result).To(BeEmpty())
				}
			},

			Entry("non-extension managed resource in garden namespace", "garden", "foobar", ""),

			Entry("managed resource for extension in non-garden namespace", "goo", "extension-foobar-garden", ""),

			Entry("managed resource for extension", "garden", "extension-foobar-garden", "foobar"),

			Entry("managed resource for runtime resources of extension admission", "garden", "extension-admission-runtime-foobar", "foobar"),

			Entry("managed resource for virtual resources of extension admission", "garden", "extension-admission-virtual-foobar", "foobar"),
		)
	})
})
