// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

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

		It("should allow valid ControllerRegistration resources", func() {
			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
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
			controllerRegistration.Spec.Resources[0].Primary = ptr.To(false)
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
		It("should forbid updating anything if deletion time stamp is set", func() {
			now := metav1.Now()

			newControllerRegistration := prepareControllerRegistrationForUpdate(controllerRegistration)
			controllerRegistration.DeletionTimestamp = &now
			newControllerRegistration.DeletionTimestamp = &now
			newControllerRegistration.Spec.Resources[0].Type = "another-os"

			errorList := ValidateControllerRegistrationUpdate(newControllerRegistration, controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update controller registration spec if deletion timestamp is set. Requested changes: Resources.slice[0].Type: another-os != my-os"),
			}))))
		})

		It("should forbid changing the primary field of a resource", func() {
			newControllerRegistration := prepareControllerRegistrationForUpdate(controllerRegistration)
			newControllerRegistration.Spec.Resources[0].Primary = ptr.To(false)

			errorList := ValidateControllerRegistrationUpdate(newControllerRegistration, controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})
	})

	Describe("#ValidateControllerResources", func() {
		var (
			resources  []core.ControllerResource
			validModes []core.ClusterType
			fldPath    *field.Path
		)

		BeforeEach(func() {
			resources = []core.ControllerResource{ctrlResource}
			validModes = []core.ClusterType{core.ClusterTypeShoot, core.ClusterTypeSeed}
			fldPath = field.NewPath("resources")
		})

		It("should allow specifying no resources", func() {
			resources = nil

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid an unset type in a given resource", func() {
			resources[0].Type = ""

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("resources[0].type"),
			}))))
		})

		It("should forbid duplicates in given resources", func() {
			resources = append(resources, resources[0])

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("resources[1]"),
			}))))
		})

		It("should allow all known extension kinds", func() {
			resources = make([]core.ControllerResource, 0, len(extensionsv1alpha1.AllExtensionKinds))
			for kind := range extensionsv1alpha1.AllExtensionKinds {
				resources = append(resources,
					core.ControllerResource{
						Kind: kind,
						Type: "foo",
					},
				)
			}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid unknown extension kinds", func() {
			resources = []core.ControllerResource{
				{
					Kind: extensionsv1alpha1.BackupBucketResource,
					Type: "my-os",
				},
				{
					Kind: "foo",
					Type: "my-os",
				},
			}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[1].kind"),
			}))))
		})

		It("should allow to set required field for kind Extension", func() {
			strategy := core.BeforeKubeAPIServer
			resource := core.ControllerResource{
				Kind:                 extensionsv1alpha1.ExtensionResource,
				Type:                 "arbitrary",
				GloballyEnabled:      ptr.To(true),
				AutoEnable:           []core.ClusterType{core.ClusterTypeShoot},
				ClusterCompatibility: []core.ClusterType{core.ClusterTypeShoot},
				ReconcileTimeout:     &metav1.Duration{Duration: 10 * time.Second},
				Lifecycle: &core.ControllerResourceLifecycle{
					Reconcile: &strategy,
				},
			}

			resources = []core.ControllerResource{resource}
			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid to set certain fields for kind != Extension", func() {
			strategy := core.BeforeKubeAPIServer
			ctrlResource.GloballyEnabled = ptr.To(true)
			ctrlResource.AutoEnable = []core.ClusterType{core.ClusterTypeShoot}
			ctrlResource.ReconcileTimeout = &metav1.Duration{Duration: 10 * time.Second}
			ctrlResource.Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &strategy,
			}
			resources = []core.ControllerResource{ctrlResource}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("resources[0].globallyEnabled"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("resources[0].autoEnable"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("resources[0].reconcileTimeout"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("resources[0].lifecycle"),
			}))))
		})

		It("should allow setting valid autoEnable modes", func() {
			resources[0].Kind = "Extension"
			resources[0].AutoEnable = []core.ClusterType{core.ClusterTypeShoot, core.ClusterTypeSeed}
			resources[0].ClusterCompatibility = []core.ClusterType{core.ClusterTypeShoot, core.ClusterTypeSeed}

			Expect(ValidateControllerResources(resources, validModes, fldPath)).To(BeEmpty())
		})

		It("should forbid setting an invalid autoEnable mode", func() {
			resources[0].Kind = "Extension"
			resources[0].AutoEnable = append(resources[0].AutoEnable, "foo")

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].autoEnable[0]"),
			}))))
		})

		It("should forbid setting an duplicate autoEnable", func() {
			resources[0].Kind = "Extension"
			resources[0].AutoEnable = []core.ClusterType{"shoot", "shoot"}
			resources[0].ClusterCompatibility = []core.ClusterType{core.ClusterTypeShoot, core.ClusterTypeSeed}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("resources[0].autoEnable[1]"),
			}))))
		})

		It("should allow setting a compatible cluster type for autoEnable", func() {
			resources[0].Kind = "Extension"
			resources[0].AutoEnable = []core.ClusterType{core.ClusterTypeShoot}
			resources[0].ClusterCompatibility = []core.ClusterType{core.ClusterTypeShoot}

			Expect(ValidateControllerResources(resources, validModes, fldPath)).To(BeEmpty())
		})

		It("should forbid setting an incompatible cluster type for autoEnable", func() {
			resources[0].Kind = "Extension"
			resources[0].AutoEnable = []core.ClusterType{core.ClusterTypeShoot}
			resources[0].ClusterCompatibility = []core.ClusterType{core.ClusterTypeSeed}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("resources[0].autoEnable[0]"),
			}))))
		})

		It("should allow setting valid clusterCompatibility modes", func() {
			resources[0].Kind = "Extension"
			resources[0].ClusterCompatibility = []core.ClusterType{"shoot", "seed"}

			Expect(ValidateControllerResources(resources, validModes, fldPath)).To(BeEmpty())
		})

		It("should forbid setting an invalid clusterCompatibility mode", func() {
			resources[0].Kind = "Extension"
			resources[0].ClusterCompatibility = append(resources[0].ClusterCompatibility, "foo")

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].clusterCompatibility[0]"),
			}))))
		})

		It("should forbid setting an duplicate clusterCompatibility", func() {
			resources[0].Kind = "Extension"
			resources[0].ClusterCompatibility = []core.ClusterType{"shoot", "shoot"}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("resources[0].clusterCompatibility[1]"),
			}))))
		})

		It("should allow setting the BeforeKubeAPIServer lifecycle strategy", func() {
			beforeStrategy := core.BeforeKubeAPIServer
			resources[0].Kind = "Extension"
			resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &beforeStrategy,
				Delete:    &beforeStrategy,
				Migrate:   &beforeStrategy,
			}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow setting the AfterKubeAPIServer lifecycle strategy", func() {
			afterStrategy := core.AfterKubeAPIServer
			resources[0].Kind = "Extension"
			resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &afterStrategy,
				Delete:    &afterStrategy,
				Migrate:   &afterStrategy,
			}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow setting the AfterWorker lifecycle strategy on reconcile", func() {
			afterStrategy := core.AfterWorker
			resources[0].Kind = "Extension"
			resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &afterStrategy,
			}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should not allow setting AfterWorker lifecycle strategy on migrate or delete", func() {
			afterStrategy := core.AfterWorker
			resources[0].Kind = "Extension"
			resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Migrate: &afterStrategy,
				Delete:  &afterStrategy,
			}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].lifecycle.migrate"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].lifecycle.delete"),
			}))))
		})

		It("should not allow setting invalid lifecycle strategies", func() {
			one := core.ControllerResourceLifecycleStrategy("one")
			two := core.ControllerResourceLifecycleStrategy("two")
			three := core.ControllerResourceLifecycleStrategy("three")
			resources[0].Kind = "Extension"
			resources[0].Lifecycle = &core.ControllerResourceLifecycle{
				Reconcile: &one,
				Delete:    &two,
				Migrate:   &three,
			}

			errorList := ValidateControllerResources(resources, validModes, fldPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].lifecycle.reconcile"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].lifecycle.delete"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].lifecycle.migrate"),
			}))))
		})
	})

	Describe("#ValidateControllerResourcesUpdate", func() {
		var resources []core.ControllerResource

		BeforeEach(func() {
			resources = []core.ControllerResource{
				{
					Kind: extensionsv1alpha1.InfrastructureResource,
					Type: "foo",
				},
				{
					Kind: extensionsv1alpha1.ExtensionResource,
					Type: "bar",
				},
			}
		})

		It("should forbid changing the primary field of a resource", func() {
			newResources := slices.Clone(resources)
			newResources[0].Primary = ptr.To(false)

			errorList := ValidateControllerResourcesUpdate(newResources, resources, field.NewPath("spec.resources"))

			Expect(errorList).To(ContainElement(MatchError(ContainSubstring("field is immutable"))))
		})

		It("should allow adding a new resource", func() {
			newResources := slices.Clone(resources)
			newResources = append(newResources, core.ControllerResource{
				Kind:    extensionsv1alpha1.ExtensionResource,
				Type:    "baz",
				Primary: ptr.To(true),
			})

			errorList := ValidateControllerResourcesUpdate(newResources, resources, field.NewPath("spec.resources"))

			Expect(errorList).To(BeEmpty())
		})

		It("should allow update without changes", func() {
			newResources := slices.Clone(resources)

			errorList := ValidateControllerResourcesUpdate(newResources, resources, field.NewPath("spec.resources"))

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareControllerRegistrationForUpdate(obj *core.ControllerRegistration) *core.ControllerRegistration {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
