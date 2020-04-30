// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootstate

import (
	"context"
	"errors"

	"github.com/gardener/gardener/pkg/apis/core"
	corev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	externalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"

	. "github.com/gardener/gardener/test/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/testing"
)

var _ = Describe("Validate ShootState deletion", func() {
	var (
		shoot                             core.Shoot
		shootState                        corev1alpha1.ShootState
		gardenExternalCoreInformerFactory externalcoreinformers.SharedInformerFactory
		gardenClient                      *fake.Clientset
		admissionHandler                  *ValidateShootStateDeletion
	)

	BeforeEach(func() {
		shootState = corev1alpha1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "garden-foo",
			},
		}

		shoot = core.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "garden-foo",
			},
		}

		admissionHandler, _ = New()
		admissionHandler.AssignReadyFunc(func() bool { return true })

		gardenExternalCoreInformerFactory = externalcoreinformers.NewSharedInformerFactory(nil, 0)
		admissionHandler.SetExternalCoreInformerFactory(gardenExternalCoreInformerFactory)

		gardenClient = &fake.Clientset{}
		admissionHandler.SetInternalCoreClientset(gardenClient)
	})

	It("should do nothing because the resource is not ShootState", func() {
		attrs := admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

		err := admissionHandler.Validate(context.TODO(), attrs, nil)

		Expect(err).NotTo(HaveOccurred())
	})

	It("should forbid ShootState deletion because shoot object exists ", func() {
		attrs := admission.NewAttributesRecord(&shootState, nil, corev1alpha1.Kind("ShootState").WithVersion("v1alpha1"), shootState.Namespace, shootState.Name, corev1alpha1.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

		gardenClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
			return true, &shoot, nil
		})

		err := admissionHandler.Validate(context.TODO(), attrs, nil)

		Expect(err).To(BeForbiddenError())
	})

	It("should return an error which is not Forbidden if retrieving the shoot fails with an error different from NotFound", func() {
		attrs := admission.NewAttributesRecord(&shootState, nil, corev1alpha1.Kind("ShootState").WithVersion("v1alpha1"), shootState.Namespace, shootState.Name, corev1alpha1.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

		gardenClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewInternalError(errors.New("Internal Server Error"))
		})

		err := admissionHandler.Validate(context.TODO(), attrs, nil)
		Expect(err).To(HaveOccurred())
		Expect(err).ToNot(BeForbiddenError())
	})

	It("should allow ShootState deletion because shoot object does not exist", func() {
		attrs := admission.NewAttributesRecord(&shootState, nil, corev1alpha1.Kind("ShootState").WithVersion("v1alpha1"), shootState.Namespace, shootState.Name, corev1alpha1.Resource("shootstates").WithVersion("v1alpha1"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

		gardenClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(core.Resource("shoot"), "foo")
		})

		err := admissionHandler.Validate(context.TODO(), attrs, nil)

		Expect(err).NotTo(HaveOccurred())
	})

	Context("Delete collection", func() {
		It("should allow deletion because no corresponding Shoot objects exists for the ShootStates", func() {
			secondShootState := shootState.DeepCopy()
			secondShootState.Name = "bar"

			Expect(gardenExternalCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Add(&shootState)).NotTo(HaveOccurred())
			Expect(gardenExternalCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Add(secondShootState)).NotTo(HaveOccurred())

			attrs := admission.NewAttributesRecord(nil, nil, corev1alpha1.Kind("ShootState").WithVersion("v1alpha1"), shootState.Namespace, "", corev1alpha1.Resource("shootstates").WithVersion("valpha1"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

			gardenClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewNotFound(core.Resource("shoot"), "")
			})

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should forbid ShootState deletion", func() {
			secondShootState := shootState.DeepCopy()
			secondShootState.Name = "bar"

			Expect(gardenExternalCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Add(&shootState)).NotTo(HaveOccurred())
			Expect(gardenExternalCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Add(secondShootState)).NotTo(HaveOccurred())

			attrs := admission.NewAttributesRecord(nil, nil, corev1alpha1.Kind("ShootState").WithVersion("v1alpha1"), shootState.Namespace, "", corev1alpha1.Resource("shootstates").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

			gardenClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &shoot, nil
			})

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(BeForbiddenError())
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

	Describe("#NewFactory", func() {
		It("should create a new PluginFactory", func() {
			f, err := NewFactory(nil)

			Expect(f).NotTo(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#New", func() {
		It("should only handle DELETE operations", func() {
			dr, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Update)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Delete)).To(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no ShootStateLister is set", func() {
			dr, _ := New()

			err := dr.ValidateInitialization()

			Expect(err).To(HaveOccurred())
		})

		It("should not return error if ShootStateLister and core client is set", func() {
			dr, _ := New()
			dr.SetExternalCoreInformerFactory(externalcoreinformers.NewSharedInformerFactory(nil, 0))
			dr.SetInternalCoreClientset(&fake.Clientset{})
			err := dr.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
