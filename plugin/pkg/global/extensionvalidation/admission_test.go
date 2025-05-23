// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionvalidation_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/global/extensionvalidation"
)

var _ = Describe("ExtensionValidator", func() {
	var (
		coreInformerFactory gardencoreinformers.SharedInformerFactory
		admissionHandler    *ExtensionValidator
	)

	BeforeEach(func() {
		admissionHandler, _ = New()
		admissionHandler.AssignReadyFunc(func() bool { return true })

		coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		admissionHandler.SetCoreInformerFactory(coreInformerFactory)
	})

	It("should do nothing because the resource is not BackupBucket, BackupEntry, Seed, or Shoot", func() {
		attrs := admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), "", "", core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

		err := admissionHandler.Validate(context.TODO(), attrs, nil)

		Expect(err).NotTo(HaveOccurred())
	})

	Context("BackupBucket", func() {
		var backupBucket = &core.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bb",
			},
			Spec: core.BackupBucketSpec{
				Provider: core.BackupBucketProvider{
					Type: "foo",
				},
			},
		}

		It("should allow to create the object", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupBucketResource, backupBucket.Spec.Provider.Type, true, nil)
			Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because no primary extension is registered for type", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupBucketResource, backupBucket.Spec.Provider.Type, false, nil)
			Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupBucketResource, "some-other-type", true, nil)
			Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should do nothing because the spec has not changed", func() {
			attrs := admission.NewAttributesRecord(backupBucket, backupBucket, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
		})
	})

	Context("BackupEntry", func() {
		var (
			backupBucket = &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bb",
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider: gardencorev1beta1.BackupBucketProvider{
						Type: "foo",
					},
				},
			}
			backupEntry = &core.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "be",
				},
				Spec: core.BackupEntrySpec{
					BucketName: backupBucket.Name,
				},
			}
		)

		It("should allow to create the object", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupEntryResource, backupBucket.Spec.Provider.Type, true, nil)
			Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().BackupBuckets().Informer().GetStore().Add(backupBucket)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because no primary extension is registered for type", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupEntryResource, backupBucket.Spec.Provider.Type, false, nil)
			Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().BackupBuckets().Informer().GetStore().Add(backupBucket)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupEntryResource, "some-other-type", true, nil)
			Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should do nothing because the spec has not changed", func() {
			attrs := admission.NewAttributesRecord(backupEntry, backupEntry, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
		})
	})

	Context("Seed", func() {
		var (
			seed = &core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seed",
				},
				Spec: core.SeedSpec{
					Provider: core.SeedProvider{
						Type: "foo",
					},
					Backup: &core.Backup{
						Provider: "bar",
					},
					Ingress: &core.Ingress{},
					DNS: core.SeedDNS{
						Provider: &core.SeedDNSProvider{
							Type: "baz",
						},
					},
					Extensions: []core.Extension{
						{Type: "foo1"},
					},
				},
			}
		)

		var (
			kindToTypes = []struct {
				extensionKind, extensionType string
				clusterCompatibility         []gardencorev1beta1.ClusterType
			}{
				{extensionsv1alpha1.ControlPlaneResource, seed.Spec.Provider.Type, nil},
				{extensionsv1alpha1.BackupBucketResource, seed.Spec.Backup.Provider, nil},
				{extensionsv1alpha1.BackupEntryResource, seed.Spec.Backup.Provider, nil},
				{extensionsv1alpha1.DNSRecordResource, seed.Spec.DNS.Provider.Type, nil},
				{extensionsv1alpha1.ExtensionResource, seed.Spec.Extensions[0].Type, []gardencorev1beta1.ClusterType{"seed"}},
				{extensionsv1alpha1.ExtensionResource, "foo2", []gardencorev1beta1.ClusterType{"shoot"}},
			}
			registerAllExtensions = func() {
				for _, registration := range kindToTypes {
					controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true, nil, registration.clusterCompatibility...)
					Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
				}
			}
		)

		It("should allow to create the object", func() {
			registerAllExtensions()

			attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because some extension is not registered", func() {
			for _, registration := range kindToTypes {
				if !sets.New(registration.clusterCompatibility...).Has("seed") {
					continue
				}

				registerAllExtensions()

				controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true, nil)
				Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Delete(controllerRegistration)).To(Succeed())

				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred(), fmt.Sprintf("expected that extension %s is not registered", controllerRegistration.Name))
				Expect(err.Error()).To(ContainSubstring(registration.extensionType))
			}
		})

		It("should prevent the object from being created because extension is not compatible with seed", func() {
			seed.Spec.Extensions = append(seed.Spec.Extensions, core.Extension{Type: "foo2"})
			registerAllExtensions()

			attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(MatchError(ContainSubstring(`Seed uses non-registered extension type: spec.extensions[1].type ("foo2")`)))
		})

		It("should prevent the object from being created because no primary extension is registered for some type", func() {
			for _, registration := range kindToTypes {
				registerAllExtensions()

				controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, false, nil)
				Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Update(controllerRegistration)).To(Succeed())

				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred(), fmt.Sprintf("expected that extension %s is not registered", controllerRegistration.Name))
				Expect(err.Error()).To(ContainSubstring(registration.extensionType))
			}
		})

		It("should prevent the object from being created because no extension type is registered", func() {
			attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should do nothing because the spec has not changed", func() {
			attrs := admission.NewAttributesRecord(seed, seed, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
		})
	})

	Context("Shoot", func() {
		var shoot = &core.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name: "shoot",
			},
			Spec: core.ShootSpec{
				DNS: &core.DNS{
					Providers: []core.DNSProvider{
						{Type: ptr.To("foo-1")},
						{Type: ptr.To("foo0")},
						{Type: ptr.To("unmanaged")},
					},
				},
				Extensions: []core.Extension{
					{Type: "foo1"},
					{Type: "foo2"},
				},
				Networking: &core.Networking{
					Type: ptr.To("foo3"),
				},
				Provider: core.Provider{
					Type: "foo4",
					Workers: []core.Worker{
						{
							Machine: core.Machine{
								Image: &core.ShootMachineImage{
									Name: "foo5",
								},
							},
						},
						{
							CRI: &core.CRI{Name: "cri",
								ContainerRuntimes: []core.ContainerRuntime{{Type: "cr1"}, {Type: "cr2"}}},
							Machine: core.Machine{
								Image: &core.ShootMachineImage{
									Name: "foo6",
								},
							},
						},
					},
				},
			},
		}

		var (
			kindToTypes = []struct {
				extensionKind, extensionType string
				clusterCompatibility         []gardencorev1beta1.ClusterType
			}{
				{extensionsv1alpha1.ControlPlaneResource, shoot.Spec.Provider.Type, nil},
				{extensionsv1alpha1.ExtensionResource, shoot.Spec.Extensions[0].Type, []gardencorev1beta1.ClusterType{"shoot"}},
				{extensionsv1alpha1.ExtensionResource, shoot.Spec.Extensions[1].Type, []gardencorev1beta1.ClusterType{"shoot", "seed"}},
				{extensionsv1alpha1.ExtensionResource, "foo3", []gardencorev1beta1.ClusterType{"seed"}},
				{extensionsv1alpha1.InfrastructureResource, shoot.Spec.Provider.Type, nil},
				{extensionsv1alpha1.NetworkResource, *shoot.Spec.Networking.Type, nil},
				{extensionsv1alpha1.OperatingSystemConfigResource, shoot.Spec.Provider.Workers[0].Machine.Image.Name, nil},
				{extensionsv1alpha1.OperatingSystemConfigResource, shoot.Spec.Provider.Workers[1].Machine.Image.Name, nil},
				{extensionsv1alpha1.WorkerResource, shoot.Spec.Provider.Type, nil},
				{extensionsv1alpha1.ContainerRuntimeResource, shoot.Spec.Provider.Workers[1].CRI.ContainerRuntimes[0].Type, nil},
				{extensionsv1alpha1.ContainerRuntimeResource, shoot.Spec.Provider.Workers[1].CRI.ContainerRuntimes[1].Type, nil},
			}
			registerAllExtensions = func() {
				for _, registration := range kindToTypes {
					controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true, nil, registration.clusterCompatibility...)
					Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
				}
			}
		)

		It("should allow to create the object", func() {
			registerAllExtensions()

			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because some extension is not registered", func() {
			for _, registration := range kindToTypes {
				if !sets.New(registration.clusterCompatibility...).Has("shoot") {
					continue
				}

				registerAllExtensions()

				controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true, nil)
				Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Delete(controllerRegistration)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred(), fmt.Sprintf("expected that extension %s is not registered", controllerRegistration.Name))
				Expect(err.Error()).To(ContainSubstring(registration.extensionType))
			}
		})

		It("should prevent the object from being created because extension is not compatible with shoot", func() {
			shoot.Spec.Extensions = append(shoot.Spec.Extensions, core.Extension{Type: "foo3"})
			registerAllExtensions()

			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(MatchError(ContainSubstring(`Shoot uses non-registered extension type: spec.extensions[2].type ("foo3")`)))
		})

		It("should prevent the object from being created because no extension type is registered", func() {
			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should do nothing because the spec has not changed", func() {
			attrs := admission.NewAttributesRecord(shoot, shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
		})

		Context("Workerless Shoot", func() {
			It("should prevent the object from being created because the extension type doesn't support workerless Shoots", func() {
				var (
					nonSupportedType      = "non-supported"
					nonSupportedExtension = createControllerRegistrationForKindType(extensionsv1alpha1.ExtensionResource, nonSupportedType, true, ptr.To(false))

					shoot = &core.Shoot{
						Spec: core.ShootSpec{
							Extensions: []core.Extension{
								{
									Type: nonSupportedType,
								},
							},
						},
					}
				)

				Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(nonSupportedExtension)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("given Shoot is workerless and uses non-supported extension type: spec.extensions[0].type (%q)", nonSupportedType)))
			})

			It("should prevent the object from being created because the extension type doesn't specify WorkerlessSupported field for workerless Shoots", func() {
				var (
					nonSupportedType      = "non-supported"
					nonSupportedExtension = createControllerRegistrationForKindType(extensionsv1alpha1.ExtensionResource, nonSupportedType, true, nil)
					shoot                 = &core.Shoot{
						Spec: core.ShootSpec{
							Extensions: []core.Extension{
								{
									Type: nonSupportedType,
								},
							},
						},
					}
				)

				Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(nonSupportedExtension)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("given Shoot is workerless and uses non-supported extension type: spec.extensions[0].type (%q)", nonSupportedType)))
			})

			It("should allow object creation because the extension type supports workerless Shoots", func() {
				var (
					supportedType      = "supported"
					supportedExtension = createControllerRegistrationForKindType(extensionsv1alpha1.ExtensionResource, supportedType, true, ptr.To(true))
					shoot              = &core.Shoot{
						Spec: core.ShootSpec{
							Extensions: []core.Extension{
								{
									Type: supportedType,
								},
							},
						},
					}
				)

				Expect(coreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(supportedExtension)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

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
			Expect(registered).To(ContainElement("ExtensionValidator"))
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
		It("should only handle CREATE + UPDATE operations", func() {
			dr, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no ControllerRegistrationLister and BackupBucketLister are set", func() {
			dr, _ := New()
			err := dr.ValidateInitialization()
			Expect(err).To(HaveOccurred())
		})

		It("should not return error if ControllerRegistrationLister, BackupBucketLister and core client are set", func() {
			dr, _ := New()
			dr.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			err := dr.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func createControllerRegistrationForKindType(extensionKind, extensionType string, primary bool, workerlessSupported *bool, clusterCompatibility ...gardencorev1beta1.ClusterType) *gardencorev1beta1.ControllerRegistration {
	return &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extensionKind + extensionType,
		},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{
				{
					Kind:                 extensionKind,
					Type:                 extensionType,
					Primary:              &primary,
					WorkerlessSupported:  workerlessSupported,
					ClusterCompatibility: clusterCompatibility,
				},
			},
		},
	}
}
