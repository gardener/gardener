// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	name      = "foo"
	namespace = "garden"
)

var _ = Describe("managedseed", func() {
	var (
		ctx         context.Context
		managedSeed *seedmanagementv1alpha1.ManagedSeed
		fakeErr     = errors.New("fake")
	)

	BeforeEach(func() {
		ctx = context.Background()

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: &seedmanagementv1alpha1.Shoot{
					Name: name,
				},
			},
		}
	})

	Describe("#GetManagedSeedWithReader", func() {
		var (
			fakeClient client.Client
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithObjects(managedSeed).
				WithIndex(&seedmanagementv1alpha1.ManagedSeed{}, seedmanagement.ManagedSeedShootName, indexer.ManagedSeedShootNameIndexerFunc).
				Build()
		})

		It("should return the ManagedSeed for the given shoot namespace and name, if it exists", func() {
			result, err := GetManagedSeedWithReader(ctx, fakeClient, namespace, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(managedSeed))
		})

		It("should return nil if a ManagedSeed does not exist", func() {
			Expect(fakeClient.Delete(ctx, managedSeed)).To(Succeed())

			result, err := GetManagedSeedWithReader(ctx, fakeClient, namespace, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if more than one ManagedSeeds exist", func() {
			managedSeed2 := managedSeed.DeepCopy()
			managedSeed2.ResourceVersion = ""
			managedSeed2.Name += "2"
			Expect(fakeClient.Create(ctx, managedSeed2)).To(Succeed())

			_, err := GetManagedSeedWithReader(ctx, fakeClient, namespace, name)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if listing the ManagedSeeds fails", func() {
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return fakeErr
					},
				}).
				Build()

			_, err := GetManagedSeedWithReader(ctx, fakeClient, namespace, name)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#GetManagedSeedByName", func() {
		var (
			seedName   = "foo"
			fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetes.GardenScheme).
					Build()
		)

		It("should return nil since the ManagedSeed is not found", func() {
			managedSeed, err := GetManagedSeedByName(ctx, fakeClient, seedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(managedSeed).To(BeNil())
		})

		It("should return an error since reading the ManagedSeed failed", func() {
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return fakeErr
					},
				}).
				Build()

			managedSeed, err := GetManagedSeedByName(ctx, fakeClient, seedName)
			Expect(err).To(MatchError(fakeErr))
			Expect(managedSeed).To(BeNil())
		})

		It("should return the ManagedSeed since reading it succeeded", func() {
			expected := &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      seedName,
					Namespace: "garden",
				},
			}
			Expect(fakeClient.Create(ctx, expected)).To(Succeed())

			managedSeed, err := GetManagedSeedByName(ctx, fakeClient, seedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(managedSeed).To(Equal(expected))
		})
	})
})
