// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/care"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GarbageCollection", func() {
	var (
		ctx                          = context.Background()
		fakeSeedClient               client.Client
		fakeShootClient              client.Client
		fakeSeedKubernetesInterface  kubernetes.Interface
		fakeShootKubernetesInterface kubernetes.Interface

		op *operation.Operation
		gc *GarbageCollection
	)

	BeforeEach(func() {
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeShootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeSeedKubernetesInterface = fakekubernetes.NewClientSetBuilder().WithClient(fakeSeedClient).Build()
		fakeShootKubernetesInterface = fakekubernetes.NewClientSetBuilder().WithClient(fakeShootClient).Build()

		op = &operation.Operation{
			Logger:        logr.Discard(),
			SeedClientSet: fakeSeedKubernetesInterface,
			Shoot:         &shoot.Shoot{ControlPlaneNamespace: "some-namespace"},
		}
		op.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

		gc = NewGarbageCollection(op, func() (kubernetes.Interface, bool, error) { return fakeShootKubernetesInterface, true, nil })
	})

	Describe("#Collect", func() {
		Context("shoot cluster", func() {
			Context("orphaned node leases", func() {
				var (
					node1, node2   *corev1.Node
					lease1, lease2 *coordinationv1.Lease
				)

				BeforeEach(func() {
					node1 = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}}
					node2 = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2"}}
					lease1 = &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: node1.Name, Namespace: "kube-node-lease"}}
					lease2 = &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: node2.Name, Namespace: "kube-node-lease"}}

					// only lease1 gets a proper owner ref to a node
					Expect(controllerutil.SetControllerReference(node1, lease1, kubernetes.ShootScheme)).To(Succeed())
				})

				It("should do nothing because there are no orphaned objects", func() {
					Expect(fakeShootClient.Create(ctx, node1)).To(Succeed())
					Expect(fakeShootClient.Create(ctx, lease1)).To(Succeed())

					gc.Collect(ctx)

					Expect(fakeShootClient.Get(ctx, client.ObjectKeyFromObject(lease1), lease1)).To(Succeed())
				})

				It("should do nothing because node still exists despite missing owner reference", func() {
					Expect(fakeShootClient.Create(ctx, node1)).To(Succeed())
					Expect(fakeShootClient.Create(ctx, lease1)).To(Succeed())

					Expect(fakeShootClient.Create(ctx, node2)).To(Succeed())
					Expect(fakeShootClient.Create(ctx, lease2)).To(Succeed())

					gc.Collect(ctx)

					Expect(fakeShootClient.Get(ctx, client.ObjectKeyFromObject(lease1), lease1)).To(Succeed())
					Expect(fakeShootClient.Get(ctx, client.ObjectKeyFromObject(lease2), lease2)).To(Succeed())
				})

				It("should clean up orphaned node Lease objects", func() {
					Expect(fakeShootClient.Create(ctx, node1)).To(Succeed())
					Expect(fakeShootClient.Create(ctx, lease1)).To(Succeed())

					Expect(fakeShootClient.Create(ctx, lease2)).To(Succeed())

					gc.Collect(ctx)

					Expect(fakeShootClient.Get(ctx, client.ObjectKeyFromObject(lease1), lease1)).To(Succeed())
					Expect(fakeShootClient.Get(ctx, client.ObjectKeyFromObject(lease2), lease2)).To(BeNotFoundError())
				})
			})
		})
	})
})
