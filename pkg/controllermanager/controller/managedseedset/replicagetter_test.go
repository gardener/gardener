// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
)

var _ = Describe("ReplicaGetter", func() {
	var (
		c client.Client

		replicaGetter ReplicaGetter

		ctx context.Context

		managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet
		shoots         []gardencorev1beta1.Shoot
		managedSeeds   []seedmanagementv1alpha1.ManagedSeed
		seeds          []gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctx = context.TODO()

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.Shoot{}, gardencore.ShootSeedName, func(obj client.Object) []string {
				shoot, ok := obj.(*gardencorev1beta1.Shoot)
				if !ok {
					return nil
				}
				if shoot.Spec.SeedName != nil {
					return []string{*shoot.Spec.SeedName}
				}
				return nil
			}).Build()

		managedSeedSet = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": name,
					},
				},
			},
		}
		shoots = []gardencorev1beta1.Shoot{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-0",
					Namespace: namespace,
					Labels:    map[string]string{"name": name},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-1",
					Namespace: namespace,
					Labels:    map[string]string{"name": name},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-2",
					Namespace: namespace,
					Labels:    map[string]string{"name": name},
				},
			},
		}
		managedSeeds = []seedmanagementv1alpha1.ManagedSeed{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-0",
					Namespace: namespace,
					Labels:    map[string]string{"name": name},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-1",
					Namespace: namespace,
					Labels:    map[string]string{"name": name},
				},
			},
		}
		seeds = []gardencorev1beta1.Seed{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name + "-0",
					Labels: map[string]string{"name": name},
				},
			},
		}
	})

	Describe("#GetReplicas", func() {
		It("should return all existing replicas", func() {
			scheduledShoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: &seeds[0].Name,
				},
			}

			Expect(c.Create(ctx, scheduledShoot.DeepCopy())).To(Succeed())

			for _, s := range shoots {
				Expect(c.Create(ctx, s.DeepCopy())).To(Succeed())
			}
			for _, ms := range managedSeeds {
				Expect(c.Create(ctx, ms.DeepCopy())).To(Succeed())
			}
			for _, sd := range seeds {
				Expect(c.Create(ctx, sd.DeepCopy())).To(Succeed())
			}

			replicaGetter = NewReplicaGetter(c, c, ReplicaFactoryFunc(NewReplica))

			result, err := replicaGetter.GetReplicas(ctx, managedSeedSet)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(3))

			// Verify structure: first shoot has managed seed and seed (with scheduled shoots)
			Expect(result[0].GetName()).To(Equal(name + "-0"))
			Expect(result[0].IsDeletable()).To(BeFalse()) // has scheduled shoots

			// Second shoot has managed seed but no seed
			Expect(result[1].GetName()).To(Equal(name + "-1"))
			Expect(result[1].GetStatus()).To(Equal(StatusManagedSeedPreparing))

			// Third shoot has neither managed seed nor seed
			Expect(result[2].GetName()).To(Equal(name + "-2"))
			Expect(result[2].GetStatus()).To(Equal(StatusShootReconciling))
		})
	})
})
