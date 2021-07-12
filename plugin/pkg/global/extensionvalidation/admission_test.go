// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionvalidation_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	externalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/global/extensionvalidation"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"
)

var _ = Describe("ExtensionValidator", func() {
	var (
		gardenExternalCoreInformerFactory externalcoreinformers.SharedInformerFactory
		admissionHandler                  *ExtensionValidator
	)

	BeforeEach(func() {
		admissionHandler, _ = New()
		admissionHandler.AssignReadyFunc(func() bool { return true })

		gardenExternalCoreInformerFactory = externalcoreinformers.NewSharedInformerFactory(nil, 0)
		admissionHandler.SetExternalCoreInformerFactory(gardenExternalCoreInformerFactory)
	})

	It("should do nothing because the resource is not BackupBucket, BackupEntry, Seed, or Shoot", func() {
		attrs := admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), "", "", core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

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
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupBucketResource, backupBucket.Spec.Provider.Type, true)
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because no primary extension is registered for type", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupBucketResource, backupBucket.Spec.Provider.Type, false)
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupBucketResource, "some-other-type", true)
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), backupBucket.Namespace, backupBucket.Name, core.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
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
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupEntryResource, backupBucket.Spec.Provider.Type, true)
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().BackupBuckets().Informer().GetStore().Add(backupBucket)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because no primary extension is registered for type", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupEntryResource, backupBucket.Spec.Provider.Type, false)
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().BackupBuckets().Informer().GetStore().Add(backupBucket)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			controllerRegistration := createControllerRegistrationForKindType(extensionsv1alpha1.BackupEntryResource, "some-other-type", true)
			Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should prevent the object from being created because extension type is not registered", func() {
			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("backupentries").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})
	})

	Context("Seed", func() {
		var seed = &core.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "seed",
			},
			Spec: core.SeedSpec{
				Provider: core.SeedProvider{
					Type: "foo",
				},
				Backup: &core.SeedBackup{
					Provider: "bar",
				},
				Ingress: &core.Ingress{},
				DNS: core.SeedDNS{
					Provider: &core.SeedDNSProvider{
						Type: "baz",
					},
				},
			},
		}

		var (
			kindToTypes = []struct {
				extensionKind, extensionType string
			}{
				{extensionsv1alpha1.ControlPlaneResource, seed.Spec.Provider.Type},
				{extensionsv1alpha1.BackupBucketResource, seed.Spec.Backup.Provider},
				{extensionsv1alpha1.BackupEntryResource, seed.Spec.Backup.Provider},
				{dnsv1alpha1.DNSProviderKind, seed.Spec.DNS.Provider.Type},
			}
			registerAllExtensions = func() {
				for _, registration := range kindToTypes {
					controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true)
					Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
				}
			}
		)

		It("should allow to create the object", func() {
			registerAllExtensions()

			attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because some extension is not registered", func() {
			for _, registration := range kindToTypes {
				registerAllExtensions()

				controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true)
				Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Delete(controllerRegistration)).To(Succeed())

				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred(), fmt.Sprintf("expected that extension %s is not registered", controllerRegistration.Name))
				Expect(err.Error()).To(ContainSubstring(registration.extensionType))
			}
		})

		It("should prevent the object from being created because no primary extension is registered for some type", func() {
			for _, registration := range kindToTypes {
				registerAllExtensions()

				controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, false)
				Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Update(controllerRegistration)).To(Succeed())

				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred(), fmt.Sprintf("expected that extension %s is not registered", controllerRegistration.Name))
				Expect(err.Error()).To(ContainSubstring(registration.extensionType))
			}
		})

		It("should prevent the object from being created because no extension type is registered", func() {
			attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), seed.Namespace, seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
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
						{Type: pointer.String("foo-1")},
						{Type: pointer.String("foo0")},
					},
				},
				Extensions: []core.Extension{
					{Type: "foo1"},
					{Type: "foo2"},
				},
				Networking: core.Networking{
					Type: "foo3",
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
			}{
				{dnsv1alpha1.DNSProviderKind, *shoot.Spec.DNS.Providers[0].Type},
				{dnsv1alpha1.DNSProviderKind, *shoot.Spec.DNS.Providers[1].Type},
				{extensionsv1alpha1.ControlPlaneResource, shoot.Spec.Provider.Type},
				{extensionsv1alpha1.ExtensionResource, shoot.Spec.Extensions[0].Type},
				{extensionsv1alpha1.ExtensionResource, shoot.Spec.Extensions[1].Type},
				{extensionsv1alpha1.InfrastructureResource, shoot.Spec.Provider.Type},
				{extensionsv1alpha1.NetworkResource, shoot.Spec.Networking.Type},
				{extensionsv1alpha1.OperatingSystemConfigResource, shoot.Spec.Provider.Workers[0].Machine.Image.Name},
				{extensionsv1alpha1.OperatingSystemConfigResource, shoot.Spec.Provider.Workers[1].Machine.Image.Name},
				{extensionsv1alpha1.WorkerResource, shoot.Spec.Provider.Type},
				{extensionsv1alpha1.ContainerRuntimeResource, shoot.Spec.Provider.Workers[1].CRI.ContainerRuntimes[0].Type},
				{extensionsv1alpha1.ContainerRuntimeResource, shoot.Spec.Provider.Workers[1].CRI.ContainerRuntimes[1].Type},
			}
			registerAllExtensions = func() {
				for _, registration := range kindToTypes {
					controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true)
					Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(controllerRegistration)).To(Succeed())
				}
			}
		)

		It("should allow to create the object", func() {
			registerAllExtensions()

			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the object from being created because some extension is not registered", func() {
			for _, registration := range kindToTypes {
				registerAllExtensions()

				controllerRegistration := createControllerRegistrationForKindType(registration.extensionKind, registration.extensionType, true)
				Expect(gardenExternalCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Delete(controllerRegistration)).To(Succeed())

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred(), fmt.Sprintf("expected that extension %s is not registered", controllerRegistration.Name))
				Expect(err.Error()).To(ContainSubstring(registration.extensionType))
			}
		})

		It("should prevent the object from being created because no extension type is registered", func() {
			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.DeleteOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
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
			dr.SetExternalCoreInformerFactory(externalcoreinformers.NewSharedInformerFactory(nil, 0))
			err := dr.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func createControllerRegistrationForKindType(extensionKind, extensionType string, primary bool) *gardencorev1beta1.ControllerRegistration {
	return &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extensionKind + extensionType,
		},
		Spec: gardencorev1beta1.ControllerRegistrationSpec{
			Resources: []gardencorev1beta1.ControllerResource{
				{
					Kind:    extensionKind,
					Type:    extensionType,
					Primary: &primary,
				},
			},
		},
	}
}
