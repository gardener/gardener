// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Fast deletion cleanup", func() {
	var (
		ctx        context.Context
		seedClient client.Client
		cleaner    *cleaner
	)

	BeforeEach(func() {
		ctx = context.Background()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		cleaner = NewCleaner(logr.Discard(), seedClient, nil, "shoot-namespace")
	})

	Describe("#StopShootResourceManager", func() {
		It("should tolerate a missing deployment", func() {
			Expect(cleaner.StopShootResourceManager(ctx)).To(Succeed())
		})

		It("should scale down without depending on healthy replicas or Pods in other namespaces", func() {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shoot-namespace", Name: v1beta1constants.DeploymentNameGardenerResourceManager},
				Spec:       appsv1.DeploymentSpec{Replicas: new(int32(2))},
			}
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())
			Expect(seedClient.Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: "other-namespace", Name: "resource-manager",
				Labels: map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager},
			}})).To(Succeed())

			Expect(cleaner.StopShootResourceManager(ctx)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(Equal(new(int32(0))))
		})

		It("should not proceed while a Shoot resource-manager Pod still exists", func() {
			Expect(seedClient.Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: "shoot-namespace", Name: "resource-manager",
				Labels: map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager},
			}})).To(Succeed())
			timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
			defer cancel()
			Expect(cleaner.StopShootResourceManager(timeoutCtx)).NotTo(Succeed())
		})
	})

	Describe("#DeleteShootManagedResources", func() {
		It("should finalize all Shoot-targeted MRs but leave Seed MRs and other namespaces untouched", func() {
			shootMR := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{
				Namespace: "shoot-namespace", Name: "shoot", Finalizers: []string{"resource-manager", "another-finalizer"},
				Labels: map[string]string{"origin": "gardener"},
			}}
			extensionMR := shootMR.DeepCopy()
			extensionMR.Name = "extension"
			extensionMR.Labels = nil
			extensionMR.Spec.Class = new("")
			seedMR := shootMR.DeepCopy()
			seedMR.Name = "seed"
			seedMR.Spec.Class = new("seed")
			otherMR := shootMR.DeepCopy()
			otherMR.Namespace = "other-namespace"
			unfinalizedMR := shootMR.DeepCopy()
			unfinalizedMR.Name = "no-finalizers"
			unfinalizedMR.Finalizers = nil
			for _, mr := range []*resourcesv1alpha1.ManagedResource{shootMR, extensionMR, seedMR, otherMR, unfinalizedMR} {
				Expect(seedClient.Create(ctx, mr)).To(Succeed())
			}
			// Already-terminating MRs must also be finalized when switching from normal to fast deletion.
			Expect(seedClient.Delete(ctx, extensionMR)).To(Succeed())

			Expect(cleaner.DeleteShootManagedResources(ctx)).To(Succeed())
			for _, mr := range []*resourcesv1alpha1.ManagedResource{shootMR, extensionMR, unfinalizedMR} {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(BeNotFoundError())
			}
			for _, mr := range []*resourcesv1alpha1.ManagedResource{seedMR, otherMR} {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
				Expect(mr.DeletionTimestamp).To(BeNil())
				Expect(mr.Finalizers).To(ConsistOf("resource-manager", "another-finalizer"))
			}

			// Repeated cleanup catches MRs created by extensions during teardown and is safe to retry.
			lateMR := shootMR.DeepCopy()
			lateMR.Name = "late"
			lateMR.ResourceVersion = ""
			Expect(seedClient.Create(ctx, lateMR)).To(Succeed())
			Expect(cleaner.DeleteShootManagedResources(ctx)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(lateMR), lateMR)).To(BeNotFoundError())
			Expect(cleaner.DeleteShootManagedResources(ctx)).To(Succeed())
		})
	})
})
