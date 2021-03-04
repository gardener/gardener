// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed_test

import (
	"context"
	"errors"

	"github.com/gardener/gardener/pkg/apis/core"
	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	corefake "github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	seedmanagementfake "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/managedseed"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/testing"
)

const (
	name      = "foo"
	namespace = "garden"
)

var _ = Describe("ManagedSeed", func() {
	Describe("#Validate", func() {
		var (
			shoot                *core.Shoot
			managedSeed          *seedmanagementv1alpha1.ManagedSeed
			coreClient           *corefake.Clientset
			seedManagementClient *seedmanagementfake.Clientset
			admissionHandler     *ManagedSeed
		)

		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

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

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreClient = &corefake.Clientset{}
			admissionHandler.SetInternalCoreClientset(coreClient)

			seedManagementClient = &seedmanagementfake.Clientset{}
			admissionHandler.SetSeedManagementClientset(seedManagementClient)
		})

		Context("delete", func() {
			It("should do nothing if the resource is not a Shoot", func() {
				attrs := admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should forbid the Shoot deletion if a ManagedSeed referencing the Shoot exists", func() {
				seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed}}, nil
				})

				err := admissionHandler.Validate(context.TODO(), getShootAttributes(shoot), nil)
				Expect(err).To(BeForbiddenError())
			})

			It("should allow the Shoot deletion if a ManagedSeed referencing the Shoot does not exist", func() {
				seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{}}, nil
				})

				err := admissionHandler.Validate(context.TODO(), getShootAttributes(shoot), nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail with an error different from Forbidden if retrieving the ManagedSeed fails with an error different from NotFound", func() {
				seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewInternalError(errors.New("Internal Server Error"))
				})

				err := admissionHandler.Validate(context.TODO(), getShootAttributes(shoot), nil)
				Expect(err).To(HaveOccurred())
				Expect(err).ToNot(BeForbiddenError())
			})
		})

		Context("delete collection", func() {
			var (
				anotherShoot *core.Shoot
			)

			BeforeEach(func() {
				anotherShoot = &core.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar",
						Namespace: "garden",
					},
				}
			})

			It("should forbid multiple Shoots deletion if a ManagedSeed referencing any of the Shoots exists", func() {
				coreClient.AddReactor("list", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &core.ShootList{Items: []core.Shoot{*shoot, *anotherShoot}}, nil
				})
				seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{*managedSeed}}, nil
				})

				err := admissionHandler.Validate(context.TODO(), getAllShootsAttributes(shoot.Namespace), nil)
				Expect(err).To(BeForbiddenError())
			})

			It("should allow multiple Shoots deletion if no ManagedSeeds referencing the Shoots exist", func() {
				coreClient.AddReactor("list", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &core.ShootList{Items: []core.Shoot{*shoot, *anotherShoot}}, nil
				})
				seedManagementClient.AddReactor("list", "managedseeds", func(action testing.Action) (bool, runtime.Object, error) {
					return true, &seedmanagementv1alpha1.ManagedSeedList{Items: []seedmanagementv1alpha1.ManagedSeed{}}, nil
				})

				err := admissionHandler.Validate(context.TODO(), getAllShootsAttributes(shoot.Namespace), nil)
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
			Expect(registered).To(ContainElement(PluginName))
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
			admissionHandler.SetInternalCoreClientset(&corefake.Clientset{})
			admissionHandler.SetSeedManagementClientset(&seedmanagementfake.Clientset{})

			err := admissionHandler.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func getShootAttributes(shoot *core.Shoot) admission.Attributes {
	return admission.NewAttributesRecord(shoot, nil, corev1beta1.Kind("Shoot").WithVersion("v1beta1"), shoot.Namespace, shoot.Name, corev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
}

func getAllShootsAttributes(namespace string) admission.Attributes {
	return admission.NewAttributesRecord(nil, nil, corev1beta1.Kind("Shoot").WithVersion("v1beta1"), namespace, "", corev1beta1.Resource("shoots").WithVersion("v1beta1"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
}
