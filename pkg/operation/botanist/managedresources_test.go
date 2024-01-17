// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var _ = Describe("ManagedResources", func() {
	var (
		fakeClient                                                                    client.Client
		botanist                                                                      *Botanist
		namespace                                                                     *corev1.Namespace
		ctx                                                                           = context.TODO()
		seedManagedResource, shootManagedResouceZeroClass, shootManagedResouceNoClass *resourcesv1alpha1.ManagedResource

		deleteManagedResoucesWithDelay = func(ctx context.Context, delay time.Duration, managedResources ...*resourcesv1alpha1.ManagedResource) {
			defer GinkgoRecover()
			time.Sleep(delay)
			for _, mr := range managedResources {
				Expect(fakeClient.Delete(ctx, mr)).To(Succeed())
			}
		}
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{Operation: &operation.Operation{}}
		k8sSeedClient := kubernetesfake.NewClientSetBuilder().WithClient(fakeClient).Build()
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		botanist.SeedClientSet = k8sSeedClient
		botanist.Shoot = &shootpkg.Shoot{
			SeedNamespace: namespace.Name,
		}
		seedManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: "seed", Namespace: namespace.Name},
			Spec:       resourcesv1alpha1.ManagedResourceSpec{Class: ptr.To("seed")},
		}
		shootManagedResouceZeroClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: "shoot-zero-class", Namespace: namespace.Name},
			Spec:       resourcesv1alpha1.ManagedResourceSpec{Class: ptr.To("")},
		}
		shootManagedResouceNoClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{Name: "shoot-no-class", Namespace: namespace.Name},
		}
	})

	Describe("#WaitUntilShootManagedResourcesDeleted", func() {
		It("should wait for all managed resources referring the shoot to be deleted", func() {
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootManagedResouceZeroClass)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootManagedResouceNoClass)).To(Succeed())
			Expect(fakeClient.Create(ctx, seedManagedResource)).To(Succeed())

			go deleteManagedResoucesWithDelay(ctx, time.Second*3, shootManagedResouceZeroClass, shootManagedResouceNoClass)

			timeoutContext, cancel := context.WithTimeout(ctx, time.Second*30)
			defer cancel()
			Expect(botanist.WaitUntilShootManagedResourcesDeleted(timeoutContext)).To(Succeed())
			mrList := &metav1.PartialObjectMetadataList{}
			mrList.SetGroupVersionKind(resourcesv1alpha1.SchemeGroupVersion.WithKind("ManagedResourceList"))
			Expect(fakeClient.List(ctx, mrList, client.InNamespace(namespace.Name))).To(Succeed())
			Expect(mrList.Items).To(HaveLen(1))
		})

		It("should timeout because not all managed resources referring the shoot are deleted", func() {
			Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootManagedResouceZeroClass)).To(Succeed())
			Expect(fakeClient.Create(ctx, shootManagedResouceNoClass)).To(Succeed())
			Expect(fakeClient.Create(ctx, seedManagedResource)).To(Succeed())

			go deleteManagedResoucesWithDelay(ctx, time.Second*1, shootManagedResouceNoClass)

			timeoutContext, cancel := context.WithTimeout(ctx, time.Second*6)
			defer cancel()
			err := botanist.WaitUntilShootManagedResourcesDeleted(timeoutContext)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&retry.Error{}))
			multiError := errors.Unwrap(err)
			Expect(multiError).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(multiError.(*multierror.Error).Errors).To(ConsistOf(fmt.Errorf("shoot managed resource %s/%s still exists", namespace.Name, shootManagedResouceZeroClass.Name)))
		})
	})
})
