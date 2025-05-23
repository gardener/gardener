// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("cleaner", func() {
	var (
		ctx = context.Background()

		log          logr.Logger
		seedClient   client.Client
		gardenClient client.Client

		namespace      = "some-namespace"
		otherNamespace = "other-namespace"
		finalizer      = "some-finalizer"
		cleaner        *cleaner

		secret1              *corev1.Secret
		secret2              *corev1.Secret
		configMap1           *corev1.ConfigMap
		configMap2           *corev1.ConfigMap
		cluster              *extensionsv1alpha1.Cluster
		controlPlane         *extensionsv1alpha1.ControlPlane
		extension            *extensionsv1alpha1.Extension
		machineDeployment    *machinev1alpha1.MachineDeployment
		machineSet           *machinev1alpha1.MachineSet
		machineClass         *machinev1alpha1.MachineClass
		machine              *machinev1alpha1.Machine
		managedresourceShoot *resourcesv1alpha1.ManagedResource
		managedresourceSeed  *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVars(
			&DefaultInterval, 100*time.Millisecond,
			&DefaultTimeout, 500*time.Millisecond,
		))

		log = logr.Discard()

		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		cleaner = NewCleaner(log, seedClient, gardenClient, namespace)

		secret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-1",
				Namespace: namespace,
			},
		}
		secret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-2",
				Namespace: namespace,
			},
		}
		configMap1 = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm-1",
				Namespace: namespace,
			},
		}
		configMap2 = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm-2",
				Namespace: namespace,
			},
		}
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		controlPlane = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-obj",
				Namespace: namespace,
			},
		}
		extension = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-obj",
				Namespace: namespace,
			},
		}
		machineDeployment = &machinev1alpha1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-obj",
				Namespace: namespace,
			},
		}
		machineSet = &machinev1alpha1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-obj",
				Namespace: namespace,
			},
		}
		machineClass = &machinev1alpha1.MachineClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-obj",
				Namespace: namespace,
			},
		}
		machine = &machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-obj",
				Namespace: namespace,
			},
		}
		managedresourceShoot = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mr-shoot",
				Namespace: namespace,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				KeepObjects: ptr.To(true),
			},
		}
		managedresourceSeed = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mr-seed",
				Namespace: namespace,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:       ptr.To("seed"),
				KeepObjects: ptr.To(true),
			},
		}
	})

	Describe("#DeleteExtensionObjects", func() {
		It("should successfully delete all extension objects in the given namespace", func() {
			controlPlane.Finalizers = []string{finalizer}
			extension.Finalizers = []string{finalizer}

			copies := makeCopies(otherNamespace, controlPlane.DeepCopy(), extension.DeepCopy())

			for _, object := range append([]client.Object{controlPlane, extension}, copies...) {
				Expect(seedClient.Create(ctx, object)).To(Succeed())
			}

			Expect(cleaner.DeleteExtensionObjects(ctx)).To(Succeed())

			for _, object := range []client.Object{controlPlane, extension} {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())
				Expect(object.GetDeletionTimestamp()).NotTo(BeNil())
			}

			for _, object := range copies {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())
				Expect(object.GetDeletionTimestamp()).To(BeNil())
			}
		})
	})

	Describe("#WaitUntilExtensionObjectsDeleted", func() {
		It("should fail to delete if extension has status.lastError", func() {
			controlPlane.Status.LastError = &gardencorev1beta1.LastError{
				Description: "invalid credentials",
			}
			extension.Status.LastError = &gardencorev1beta1.LastError{
				Description: "invalid credentials",
			}

			for _, object := range []client.Object{controlPlane, extension} {
				Expect(seedClient.Create(ctx, object)).To(Succeed())
			}

			err := cleaner.WaitUntilExtensionObjectsDeleted(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to delete ControlPlane"))
			Expect(err.Error()).To(ContainSubstring("Failed to delete Extension"))
		})

		It("should not return error on Wait", func() {
			Expect(cleaner.WaitUntilExtensionObjectsDeleted(ctx)).To(Succeed())
		})
	})

	Describe("#DeleteMachineResources", func() {
		It("should successfully delete all machine related resources in the given namespace", func() {
			machine.Finalizers = []string{finalizer}
			machineClass.Finalizers = []string{finalizer}
			machineSet.Finalizers = []string{finalizer}
			machineDeployment.Finalizers = []string{finalizer}

			copies := makeCopies(otherNamespace, machine.DeepCopy(), machineClass.DeepCopy(), machineSet.DeepCopy(), machineDeployment.DeepCopy())

			for _, object := range append([]client.Object{machine, machineClass, machineSet, machineDeployment}, copies...) {
				Expect(seedClient.Create(ctx, object)).To(Succeed())
			}

			for _, object := range append([]client.Object{machine, machineClass, machineSet, machineDeployment}, copies...) {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())
				Expect(object.GetFinalizers()).To(ContainElement(finalizer))
			}

			Expect(cleaner.DeleteMachineResources(ctx)).To(Succeed())

			for _, object := range []client.Object{machine, machineClass, machineSet, machineDeployment} {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(BeNotFoundError())
			}

			for _, object := range copies {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())
				Expect(object.GetDeletionTimestamp()).To(BeNil())
			}
		})
	})

	Describe("#SetKeepObjectsForManagedResources", func() {
		It("should successfully set keepObjects to false for all managedResources in the given namespace", func() {
			managedresourceSeedCopy := managedresourceSeed.DeepCopy()
			managedresourceSeedCopy.SetNamespace(otherNamespace)
			managedresourceShootCopy := managedresourceShoot.DeepCopy()
			managedresourceShootCopy.SetNamespace(otherNamespace)

			for _, mr := range []*resourcesv1alpha1.ManagedResource{managedresourceSeed, managedresourceSeedCopy, managedresourceShoot, managedresourceShootCopy} {
				Expect(seedClient.Create(ctx, mr)).To(Succeed())
			}

			Expect(cleaner.SetKeepObjectsForManagedResources(ctx)).To(Succeed())

			for _, mr := range []*resourcesv1alpha1.ManagedResource{managedresourceSeed, managedresourceSeedCopy, managedresourceShoot, managedresourceShootCopy} {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(Succeed())
			}

			Expect(managedresourceShoot.Spec.KeepObjects).To(PointTo(BeFalse()))
			Expect(managedresourceSeed.Spec.KeepObjects).To(PointTo(BeFalse()))
			Expect(managedresourceShootCopy.Spec.KeepObjects).To(PointTo(BeTrue()))
			Expect(managedresourceSeedCopy.Spec.KeepObjects).To(PointTo(BeTrue()))
		})
	})

	Describe("#DeleteManagedResources", func() {
		It("should successfully delete all managed resources, and remove finalizers from Shoot managed resources in the given namespace", func() {
			managedresourceSeed.Finalizers = []string{finalizer}
			managedresourceShoot.Finalizers = []string{finalizer}

			copies := makeCopies(otherNamespace, managedresourceSeed.DeepCopy(), managedresourceShoot.DeepCopy())

			for _, object := range append([]client.Object{managedresourceSeed, managedresourceShoot}, copies...) {
				Expect(seedClient.Create(ctx, object)).To(Succeed())
			}

			Expect(cleaner.DeleteManagedResources(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(managedresourceShoot), managedresourceShoot)).To(BeNotFoundError())
			for _, object := range append([]client.Object{managedresourceSeed}, copies...) {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())
			}

			Expect(managedresourceSeed.DeletionTimestamp).NotTo(BeNil())

			for _, object := range copies {
				Expect(object.GetDeletionTimestamp()).To(BeNil())
			}

		})
	})

	Describe("#WaitUntilManagedResourcesDeleted", func() {
		It("should successfully delete all managed resources", func() {
			Expect(cleaner.WaitUntilManagedResourcesDeleted(ctx)).To(Succeed())
		})
	})

	Describe("#DeleteKubernetesResources", func() {
		It("should successfully delete all secrets and configmaps in the given namespace", func() {
			secret1.Finalizers = []string{finalizer}
			secret2.Finalizers = []string{finalizer}
			configMap1.Finalizers = []string{finalizer}
			configMap2.Finalizers = []string{finalizer}

			copies := makeCopies(otherNamespace, secret1.DeepCopy(), secret2.DeepCopy(), configMap1.DeepCopy(), configMap2.DeepCopy())

			for _, object := range append([]client.Object{secret1, secret2, configMap1, configMap2}, copies...) {
				Expect(seedClient.Create(ctx, object)).To(Succeed())
			}

			Expect(cleaner.DeleteKubernetesResources(ctx)).To(Succeed())

			for _, object := range []client.Object{secret1, secret2, configMap1, configMap2} {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(BeNotFoundError())
			}

			for _, object := range copies {
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(object), object)).To(Succeed())
			}
		})
	})

	Describe("#DeleteCluster", func() {
		It("should successfully delete cluster", func() {
			Expect(seedClient.Create(ctx, cluster)).To(Succeed())

			Expect(cleaner.DeleteCluster(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(BeNotFoundError())
		})
	})
})

func makeCopies(namespace string, objects ...client.Object) []client.Object {
	out := make([]client.Object, 0, len(objects))

	for _, object := range objects {
		object.SetNamespace(namespace)
		out = append(out, object)
	}

	return out
}
