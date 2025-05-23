// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Bastions", func() {
	var (
		fakeClient         client.Client
		botanist           *Botanist
		namespace          *corev1.Namespace
		ctx                = context.TODO()
		bastion1, bastion2 *extensionsv1alpha1.Bastion
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{Operation: &operation.Operation{}}
		k8sSeedClient := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		botanist.SeedClientSet = k8sSeedClient
		botanist.Shoot = &shootpkg.Shoot{
			ControlPlaneNamespace: namespace.Name,
		}

		bastion1 = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{Name: "bastion1", Namespace: namespace.Name},
		}
		bastion2 = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{Name: "bastion2", Namespace: namespace.Name},
		}
	})

	Describe("#DeleteBastions", func() {
		It("should delete all bastions", func() {
			Expect(fakeClient.Create(ctx, bastion1)).To(Succeed())
			Expect(fakeClient.Create(ctx, bastion2)).To(Succeed())

			Expect(botanist.DeleteBastions(ctx)).To(Succeed())

			bastionList := &metav1.PartialObjectMetadataList{}
			bastionList.SetGroupVersionKind(extensionsv1alpha1.SchemeGroupVersion.WithKind("BastionList"))
			Expect(fakeClient.List(ctx, bastionList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(bastionList.Items).To(BeEmpty())
		})
	})
})
