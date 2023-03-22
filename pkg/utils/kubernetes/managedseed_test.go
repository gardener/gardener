// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
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
			_, err := GetManagedSeedWithReader(ctx, failingListReader{fakeClient}, namespace, name)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#GetManagedSeedByName", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			seedName = "foo"
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return nil since the ManagedSeed is not found", func() {
			c.EXPECT().Get(ctx, Key("garden", seedName), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			managedSeed, err := GetManagedSeedByName(ctx, c, seedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(managedSeed).To(BeNil())
		})

		It("should return an error since reading the ManagedSeed failed", func() {
			fakeErr := fmt.Errorf("fake")

			c.EXPECT().Get(ctx, Key("garden", seedName), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).Return(fakeErr)

			managedSeed, err := GetManagedSeedByName(ctx, c, seedName)
			Expect(err).To(MatchError(fakeErr))
			Expect(managedSeed).To(BeNil())
		})

		It("should return the ManagedSeed since reading it succeeded", func() {
			expected := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: seedName}}

			c.EXPECT().Get(ctx, Key("garden", seedName), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *seedmanagementv1alpha1.ManagedSeed, _ ...client.GetOption) error {
				expected.DeepCopyInto(obj)
				return nil
			})

			managedSeed, err := GetManagedSeedByName(ctx, c, seedName)
			Expect(err).NotTo(HaveOccurred())
			Expect(managedSeed).To(Equal(expected))
		})
	})
})

type failingListReader struct {
	client.Reader
}

func (failingListReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return fmt.Errorf("fake")
}
