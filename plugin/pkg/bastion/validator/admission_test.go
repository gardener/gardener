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

package validator_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	corev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	corefake "github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/bastion/validator"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
)

const (
	bastionName = "foo"
	shootName   = "foo"
	seedName    = "foo"
	namespace   = "garden"
	provider    = "foo-provider"
	region      = "foo-region"
	userName    = "ginkgo"
)

var _ = Describe("Bastion", func() {
	Describe("#Admit", func() {
		var (
			bastion          *core.Bastion
			shoot            *core.Shoot
			coreClient       *corefake.Clientset
			admissionHandler *Bastion
		)

		BeforeEach(func() {
			bastion = &core.Bastion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bastionName,
					Namespace: namespace,
				},
				Spec: core.BastionSpec{
					ShootRef: corev1.LocalObjectReference{
						Name: shootName,
					},
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					SeedName: pointer.StringPtr(seedName),
					Provider: core.Provider{
						Type: provider,
					},
					Region: region,
				},
			}

			var err error
			admissionHandler, err = New()
			Expect(err).ToNot(HaveOccurred())
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreClient = &corefake.Clientset{}
			admissionHandler.SetInternalCoreClientset(coreClient)
		})

		It("should do nothing if the resource is not a Bastion", func() {
			attrs := admission.NewAttributesRecord(nil, nil, core.Kind(bastionName).WithVersion("version"), bastion.Namespace, bastion.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)
			Expect(err).To(Succeed())
		})

		It("should allow Bastion creation if all fields are set correctly", func() {
			coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
				return true, shoot, nil
			})

			err := admissionHandler.Admit(context.TODO(), getBastionAttributes(bastion), nil)
			Expect(err).To(Succeed())
		})

		It("should mutate Bastion with Shoot information", func() {
			coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
				return true, shoot, nil
			})

			err := admissionHandler.Admit(context.TODO(), getBastionAttributes(bastion), nil)
			Expect(err).To(Succeed())
			Expect(bastion.Spec.SeedName).To(PointTo(Equal(seedName)))
			Expect(bastion.Spec.ProviderType).To(PointTo(Equal(provider)))
		})

		It("should mutate Bastion with creator information", func() {
			coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
				return true, shoot, nil
			})

			err := admissionHandler.Admit(context.TODO(), getBastionAttributes(bastion), nil)
			Expect(err).To(Succeed())
			Expect(bastion.Annotations[v1alpha1constants.GardenerCreatedBy]).To(Equal(userName))
		})

		It("should always keep the creator annotation", func() {
			coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
				return true, shoot, nil
			})

			bastion.Annotations = map[string]string{
				v1alpha1constants.GardenerCreatedBy: "not-" + userName,
			}

			err := admissionHandler.Admit(context.TODO(), getBastionAttributes(bastion), nil)
			Expect(err).To(Succeed())
			Expect(bastion.Annotations[v1alpha1constants.GardenerCreatedBy]).To(Equal(userName))
		})

		It("should forbid the Bastion creation if a Shoot name is not specified", func() {
			bastion.Spec.ShootRef.Name = ""

			err := admissionHandler.Admit(context.TODO(), getBastionAttributes(bastion), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.shootRef.name"),
				})),
			))
		})

		It("should forbid the Bastion creation if the Shoot does not exist", func() {
			bastion.Spec.ShootRef.Name = "does-not-exist"

			err := admissionHandler.Admit(context.TODO(), getBastionAttributes(bastion), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootRef.name"),
				})),
			))
		})

		It("should forbid the Bastion creation if the Shoot does not specify a Seed", func() {
			shoot.Spec.SeedName = nil

			coreClient.AddReactor("get", "shoots", func(action testing.Action) (bool, runtime.Object, error) {
				return true, shoot, nil
			})

			err := admissionHandler.Admit(context.TODO(), getBastionAttributes(bastion), nil)
			Expect(err).To(BeInvalidError())
			Expect(getErrorList(err)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootRef.name"),
				})),
			))
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
		It("should only handle CREATE and UPDATE operations", func() {
			admissionHandler, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(admissionHandler.Handles(admission.Create)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Update)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Connect)).To(BeFalse())
			Expect(admissionHandler.Handles(admission.Delete)).To(BeFalse())
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

			err := admissionHandler.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func getBastionAttributes(bastion *core.Bastion) admission.Attributes {
	return admission.NewAttributesRecord(bastion,
		nil,
		corev1alpha1.Kind("Bastion").WithVersion("v1alpha1"),
		bastion.Namespace,
		bastion.Name,
		corev1alpha1.Resource("bastions").WithVersion("v1alpha1"),
		"",
		admission.Create,
		&metav1.CreateOptions{},
		false,
		&user.DefaultInfo{
			Name: userName,
		},
	)
}

func getErrorList(err error) field.ErrorList {
	statusError, ok := err.(*apierrors.StatusError)
	if !ok {
		return field.ErrorList{}
	}
	var errs field.ErrorList
	for _, cause := range statusError.ErrStatus.Details.Causes {
		errs = append(errs, &field.Error{
			Type:   field.ErrorType(cause.Type),
			Field:  cause.Field,
			Detail: cause.Message,
		})
	}
	return errs
}
