// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("validation", func() {
	var (
		controllerRegistration *core.ControllerRegistration
		ctrlResource           core.ControllerResource
	)

	BeforeEach(func() {
		ctrlResource = core.ControllerResource{
			Kind: extensionsv1alpha1.OperatingSystemConfigResource,
			Type: "my-os",
		}
		controllerRegistration = &core.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "extension-abc",
			},
			Spec: core.ControllerRegistrationSpec{
				Resources: []core.ControllerResource{
					ctrlResource,
				},
				Deployment: &core.ControllerRegistrationDeployment{},
			},
		}
	})

	Describe("#ValidateControllerRegistration", func() {
		DescribeTable("ControllerRegistration metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				controllerRegistration.ObjectMeta = objectMeta

				errorList := ValidateControllerRegistration(controllerRegistration)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid ControllerRegistration with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ControllerRegistration with empty name",
				metav1.ObjectMeta{Name: ""},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ControllerRegistration with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "extension-abc.test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ControllerRegistration with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "extension-abc_test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid empty ControllerRegistration resources", func() {
			errorList := ValidateControllerRegistration(&core.ControllerRegistration{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid empty values in a given resource", func() {
			controllerRegistration.Spec.Resources[0].Type = ""

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.resources[0].type"),
			}))))
		})

		It("should forbid duplicates in given resources", func() {
			controllerRegistration.Spec.Resources = append(controllerRegistration.Spec.Resources, controllerRegistration.Spec.Resources[0])

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("spec.resources[1]"),
			}))))
		})

		It("should allow all known extension kinds", func() {
			controllerRegistration.Spec.Resources = make([]core.ControllerResource, 0, len(SupportedExtensionKinds))
			for kind := range SupportedExtensionKinds {
				controllerRegistration.Spec.Resources = append(controllerRegistration.Spec.Resources,
					core.ControllerResource{
						Kind: kind,
						Type: "foo",
					},
				)
			}

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid unknown extension kinds", func() {
			controllerRegistration.Spec.Resources = []core.ControllerResource{
				{
					Kind: extensionsv1alpha1.BackupBucketResource,
					Type: "my-os",
				},
				{
					Kind: "foo",
					Type: "my-os",
				},
			}

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.resources[1].kind"),
			}))))
		})

		It("should allow specifying no resources", func() {
			controllerRegistration.Spec.Resources = nil

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow valid ControllerRegistration resources", func() {
			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow to set required field for kind Extension", func() {
			strat := core.BeforeKubeAPIServer
			resource := core.ControllerResource{
				Kind:             extensionsv1alpha1.ExtensionResource,
				Type:             "arbitrary",
				GloballyEnabled:  pointer.Bool(true),
				ReconcileTimeout: makeDurationPointer(10 * time.Second),
				Lifecycle: &core.ControllerResourceLifecycle{
					Reconcile: &strat,
				},
			}

			controllerRegistration.Spec.Resources = []core.ControllerResource{resource}
			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid to set certain fields for kind != Extension", func() {
			strat := core.BeforeKubeAPIServer
			ctrlResource.GloballyEnabled = pointer.Bool(true)
			ctrlResource.ReconcileTimeout = makeDurationPointer(10 * time.Second)
			ctrlResource.Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &strat,
			}
			controllerRegistration.Spec.Resources = []core.ControllerResource{ctrlResource}

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.resources[0].globallyEnabled"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.resources[0].reconcileTimeout"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.resources[0].lifecycle"),
			}))))
		})

		It("should allow setting the BeforeKubeAPIServer lifecycle strategy", func() {
			beforeStrat := core.BeforeKubeAPIServer
			controllerRegistration.Spec.Resources[0].Kind = "Extension"
			controllerRegistration.Spec.Resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &beforeStrat,
				Delete:    &beforeStrat,
				Migrate:   &beforeStrat,
			}

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow setting the AfterKubeAPIServer lifecycle strategy", func() {
			afterStrat := core.AfterKubeAPIServer
			controllerRegistration.Spec.Resources[0].Kind = "Extension"
			controllerRegistration.Spec.Resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &afterStrat,
				Delete:    &afterStrat,
				Migrate:   &afterStrat,
			}

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should not allow setting invalid lifecycle strategies", func() {
			one := core.ControllerResourceLifecycleStrategy("one")
			two := core.ControllerResourceLifecycleStrategy("two")
			three := core.ControllerResourceLifecycleStrategy("three")
			controllerRegistration.Spec.Resources[0].Kind = "Extension"
			controllerRegistration.Spec.Resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &one,
				Delete:    &two,
				Migrate:   &three,
			}

			errorList := ValidateControllerRegistration(controllerRegistration)
			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.resources[0].lifecycle.reconcile"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.resources[0].lifecycle.delete"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.resources[0].lifecycle.migrate"),
			}))))
		})

		It("should allow setting the OnDemand policy", func() {
			policy := core.ControllerDeploymentPolicyOnDemand
			controllerRegistration.Spec.Deployment.Policy = &policy

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow setting the Always policy", func() {
			policy := core.ControllerDeploymentPolicyAlways
			controllerRegistration.Spec.Deployment.Policy = &policy

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow setting the AlwaysExceptNoShoots policy", func() {
			policy := core.ControllerDeploymentPolicyAlwaysExceptNoShoots
			controllerRegistration.Spec.Deployment.Policy = &policy

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid to set unsupported deployment policies", func() {
			policy := core.ControllerDeploymentPolicy("foo")
			controllerRegistration.Spec.Deployment.Policy = &policy

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.deployment.policy"),
			}))))
		})

		It("should forbid to set seed selectors if it controls a resource primarily", func() {
			controllerRegistration.Spec.Deployment.SeedSelector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			}

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.deployment.seedSelector"),
			}))))
		})

		It("should forbid to set unsupported seed selectors", func() {
			controllerRegistration.Spec.Resources[0].Primary = pointer.Bool(false)
			controllerRegistration.Spec.Deployment.SeedSelector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "no/slash/allowed",
				},
			}

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.deployment.seedSelector.matchLabels"),
			}))))
		})

		It("should forbid specifying more than one reference to a ControllerDeployment", func() {
			controllerRegistration.Spec.Deployment.DeploymentRefs = []core.DeploymentRef{
				{Name: "foo"},
				{Name: "bar"},
			}
			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.deployment.deploymentRefs"),
			}))))
		})

		It("should forbid specifying a ControllerDeployment reference w/ an empty name", func() {
			controllerRegistration.Spec.Deployment.DeploymentRefs = []core.DeploymentRef{
				{Name: ""},
			}
			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.deploymentRefs[0].name"),
			}))))
		})
	})

	Describe("#ValidateControllerRegistrationUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()

			newControllerRegistration := prepareControllerRegistrationForUpdate(controllerRegistration)
			controllerRegistration.DeletionTimestamp = &now
			newControllerRegistration.DeletionTimestamp = &now
			newControllerRegistration.Spec.Resources[0].Type = "another-os"

			errorList := ValidateControllerRegistrationUpdate(newControllerRegistration, controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))))
		})

		It("should prevent changing the primary field of a resource", func() {
			newControllerRegistration := prepareControllerRegistrationForUpdate(controllerRegistration)
			newControllerRegistration.Spec.Resources[0].Primary = pointer.Bool(false)

			errorList := ValidateControllerRegistrationUpdate(newControllerRegistration, controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})
	})
})

func prepareControllerRegistrationForUpdate(obj *core.ControllerRegistration) *core.ControllerRegistration {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
