// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	fakeseedmanagement "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	. "github.com/gardener/gardener/plugin/pkg/utils"
)

var _ = Describe("ManagedSeed utils", func() {
	const (
		name      = "foo"
		namespace = "garden"
	)

	var (
		ctx         context.Context
		managedSeed *seedmanagementv1alpha1.ManagedSeed
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

	Describe("#GetManagedSeed", func() {
		var seedManagementClient *fakeseedmanagement.Clientset

		BeforeEach(func() {
			seedManagementClient = &fakeseedmanagement.Clientset{}
		})

		It("should return the ManagedSeed for the given shoot namespace and name, if it exists", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(_ testing.Action) (bool, runtime.Object, error) {
				return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed}}, nil
			})

			result, err := GetManagedSeed(ctx, seedManagementClient, namespace, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(managedSeed))
		})

		It("should return nil if a ManagedSeed does not exist", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(_ testing.Action) (bool, runtime.Object, error) {
				return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{}}, nil
			})

			result, err := GetManagedSeed(ctx, seedManagementClient, namespace, name)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if more than one ManagedSeeds exist", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(_ testing.Action) (bool, runtime.Object, error) {
				return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed, *managedSeed}}, nil
			})

			_, err := GetManagedSeed(ctx, seedManagementClient, namespace, name)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if listing the ManagedSeeds fails", func() {
			seedManagementClient.AddReactor("list", "managedseeds", func(_ testing.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("error")
			})

			_, err := GetManagedSeed(ctx, seedManagementClient, namespace, name)
			Expect(err).To(HaveOccurred())
		})
	})
})
