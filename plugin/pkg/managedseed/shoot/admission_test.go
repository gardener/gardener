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

package shoot_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	fakeseedmanagement "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/managedseed/shoot"
)

const (
	name      = "foo"
	shootName = "test"
	namespace = "garden"
)

var _ = Describe("Shoot", func() {
	Describe("#Validate", func() {
		var (
			managedSeed          *seedmanagementv1alpha1.ManagedSeed
			shoot                *core.Shoot
			coreInformerFactory  gardencoreinformers.SharedInformerFactory
			seedManagementClient *fakeseedmanagement.Clientset
			admissionHandler     *Shoot
		)

		BeforeEach(func() {
			managedSeed = &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					SeedName: ptr.To(name),
				},
			}

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)

			seedManagementClient = &fakeseedmanagement.Clientset{}
			admissionHandler.SetSeedManagementClientset(seedManagementClient)
		})

		Context("delete", func() {
			It("should do nothing if the resource is not a ManagedSeed", func() {
				attrs := admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), managedSeed.Namespace, managedSeed.Name, core.Resource("foos").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should forbid the ManagedSeed deletion if a Shoot scheduled on its Seed exists", func() {
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

				err := admissionHandler.Validate(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).To(BeForbiddenError())
			})

			It("should allow the ManagedSeed deletion if a Shoot scheduled on its Seed does not exists", func() {
				err := admissionHandler.Validate(context.TODO(), getManagedSeedAttributes(managedSeed), nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("delete collection", func() {
			var (
				anotherManagedSeed *seedmanagementv1alpha1.ManagedSeed
			)

			BeforeEach(func() {
				anotherManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar",
						Namespace: namespace,
					},
				}
			})

			It("should forbid multiple ManagedSeed deletion if a Shoots scheduled on any of their Seeds exists", func() {
				seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed, *anotherManagedSeed}}, nil
				})
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())

				err := admissionHandler.Validate(context.TODO(), getAllManagedSeedAttributes(managedSeed.Namespace), nil)
				Expect(err).To(BeForbiddenError())
			})

			It("should allow multiple ManagedSeed deletion if no Shoots scheduled on any of their Seeds exist", func() {
				seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed, *anotherManagedSeed}}, nil
				})

				err := admissionHandler.Validate(context.TODO(), getAllManagedSeedAttributes(managedSeed.Namespace), nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ManagedSeedShoot"))
		})
	})

	Describe("#New", func() {
		It("should only handle DELETE operations", func() {
			admissionHandler, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(admissionHandler.Handles(admission.Create)).NotTo(BeTrue())
			Expect(admissionHandler.Handles(admission.Update)).NotTo(BeTrue())
			Expect(admissionHandler.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(admissionHandler.Handles(admission.Delete)).To(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should fail if the required clients are not set", func() {
			admissionHandler, _ := New()

			err := admissionHandler.ValidateInitialization()
			Expect(err).To(HaveOccurred())
		})

		It("should not fail if the required clients are set", func() {
			admissionHandler, _ := New()
			admissionHandler.SetInternalCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			admissionHandler.SetSeedManagementClientset(&fakeseedmanagement.Clientset{})

			err := admissionHandler.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func getManagedSeedAttributes(managedSeed *seedmanagementv1alpha1.ManagedSeed) admission.Attributes {
	return admission.NewAttributesRecord(managedSeed, nil, seedmanagementv1alpha1.Kind("ManagedSeed").WithVersion("v1alpha1"), managedSeed.Namespace, managedSeed.Name, seedmanagementv1alpha1.Resource("managedseeds").WithVersion("v1alpha1"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
}

func getAllManagedSeedAttributes(namespace string) admission.Attributes {
	return admission.NewAttributesRecord(nil, nil, seedmanagementv1alpha1.Kind("ManagedSeed").WithVersion("v1alpha1"), namespace, "", seedmanagementv1alpha1.Resource("managedseeds").WithVersion("v1alpha1"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
}
