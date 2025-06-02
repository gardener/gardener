// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate_test

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ShootState", func() {
	var (
		ctx           = context.TODO()
		seedNamespace = "shoot--my-project--my-shoot"

		fakeGardenClient client.Client
		fakeSeedClient   client.Client
		fakeClock        clock.Clock

		shoot      *gardencorev1beta1.Shoot
		shootState *gardencorev1beta1.ShootState
	)

	BeforeEach(func() {
		fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClock = testclock.NewFakeClock(time.Now())

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-shoot",
				Namespace: "garden-my-project",
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: seedNamespace,
			},
		}
		shootState = &gardencorev1beta1.ShootState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-shoot",
				Namespace: "garden-my-project",
			},
		}
	})

	Describe("#Deploy", func() {
		It("should deploy an empty ShootState when there is nothing to persist", func() {
			Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot, true)).To(Succeed())
			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(gardencorev1beta1.ShootStateSpec{}))
			Expect(shootState.Annotations).To(HaveKeyWithValue("gardener.cloud/timestamp", fakeClock.Now().UTC().Format(time.RFC3339)))
		})

		Context("with data to backup", func() {
			var (
				existingGardenerData      = []gardencorev1beta1.GardenerResourceData{{Name: "some-data"}}
				existingExtensionsData    = []gardencorev1beta1.ExtensionResourceState{{Name: ptr.To("some-data")}}
				existingResourcesData     = []gardencorev1beta1.ResourceData{{Data: runtime.RawExtension{Raw: []byte("{}")}}}
				expectedSpec              gardencorev1beta1.ShootStateSpec
				cleanupMachineObjectsFunc func(ctx context.Context)
			)

			BeforeEach(func() {
				By("Create ShootState with some data")
				shootState.Spec.Gardener = append(shootState.Spec.Gardener, existingGardenerData...)
				shootState.Spec.Extensions = append(shootState.Spec.Extensions, existingExtensionsData...)
				shootState.Spec.Resources = append(shootState.Spec.Resources, existingResourcesData...)
				Expect(fakeGardenClient.Create(ctx, shootState)).To(Succeed())

				By("Creating Gardener data")
				Expect(fakeSeedClient.Create(ctx, newSecret("secret1", seedNamespace, true, true))).To(Succeed())
				Expect(fakeSeedClient.Create(ctx, newSecret("secret2", seedNamespace, false, true))).To(Succeed())
				Expect(fakeSeedClient.Create(ctx, newSecret("secret3", seedNamespace, true, false))).To(Succeed())

				By("Creating extensions data")
				purposeNormal := extensionsv1alpha1.Normal

				createExtensionObject(ctx, fakeSeedClient, "backupentry", seedNamespace, &extensionsv1alpha1.BackupEntry{}, &runtime.RawExtension{Raw: []byte(`{"name":"backupentry"}`)})
				createExtensionObject(ctx, fakeSeedClient, "containerruntime", seedNamespace, &extensionsv1alpha1.ContainerRuntime{}, &runtime.RawExtension{Raw: []byte(`{"name":"containerruntime"}`)})
				createExtensionObject(ctx, fakeSeedClient, "controlplane", seedNamespace, &extensionsv1alpha1.ControlPlane{Spec: extensionsv1alpha1.ControlPlaneSpec{Purpose: &purposeNormal}}, &runtime.RawExtension{Raw: []byte(`{"name":"controlplane"}`)})
				createExtensionObject(ctx, fakeSeedClient, "dnsrecord", seedNamespace, &extensionsv1alpha1.DNSRecord{}, &runtime.RawExtension{Raw: []byte(`{"name":"dnsrecord"}`)})
				createExtensionObject(ctx, fakeSeedClient, "extension", seedNamespace, &extensionsv1alpha1.Extension{}, &runtime.RawExtension{Raw: []byte(`{"name":"extension"}`)}, gardencorev1beta1.NamedResourceReference{Name: "resource-ref1", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "ConfigMap", APIVersion: "v1", Name: "extension-configmap"}})
				Expect(fakeSeedClient.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "extension-configmap", Namespace: seedNamespace}, Data: map[string]string{"some-data": "for-extension"}})).To(Succeed())
				createExtensionObject(ctx, fakeSeedClient, "infrastructure", seedNamespace, &extensionsv1alpha1.Infrastructure{}, &runtime.RawExtension{Raw: []byte(`{"name":"infrastructure"}`)})
				createExtensionObject(ctx, fakeSeedClient, "network", seedNamespace, &extensionsv1alpha1.Network{}, &runtime.RawExtension{Raw: []byte(`{"name":"network"}`)})
				createExtensionObject(ctx, fakeSeedClient, "osc", seedNamespace, &extensionsv1alpha1.OperatingSystemConfig{}, &runtime.RawExtension{Raw: []byte(`{"name":"osc"}`)})
				createExtensionObject(ctx, fakeSeedClient, "worker", seedNamespace, &extensionsv1alpha1.Worker{}, &runtime.RawExtension{Raw: []byte(`{"name":"worker"}`)})
				// this extension object has no state, hence it should not be persisted in the ShootState
				createExtensionObject(ctx, fakeSeedClient, "osc2", seedNamespace, &extensionsv1alpha1.OperatingSystemConfig{}, nil)

				By("Creating machine data")
				cleanupMachineObjectsFunc = createMachineObjects(ctx, fakeSeedClient, seedNamespace)

				expectedSpec = gardencorev1beta1.ShootStateSpec{
					Gardener: []gardencorev1beta1.GardenerResourceData{
						{
							Name:   "secret1",
							Type:   "secret",
							Data:   runtime.RawExtension{Raw: []byte(`{"secret1":"c29tZS1kYXRh"}`)},
							Labels: map[string]string{"managed-by": "secrets-manager", "persist": "true"},
						},
						{
							Name:   "secret3",
							Type:   "secret",
							Data:   runtime.RawExtension{Raw: []byte(`{"secret3":"c29tZS1kYXRh"}`)},
							Labels: map[string]string{"persist": "true"},
						},
						{
							Name: "machine-state",
							Type: "machine-state",
							Data: runtime.RawExtension{Raw: []byte(`{"state":"H4sIAAAAAAAC/+yUPU8DMQyG/4vnZChst9KFiaFlQgwmsdSgfCl2kaoq/x0lvX4gVEGrG5DoLZfYb177nli3hYBm5SLNKfu0CRSFYdiC7dtZWxbK3hlkGO7VXr2gJnvZQiBBi4JNGDEQDPujmklmoHqUM5qW4lVKonXY6FzSO5ndukdBgSmE4lJcukAsGDIMce29AowxSU/13jj1Osco1KqAM5mWHTt88MhNXRUIhexRqGdP+j1T78TLHE1isrS81qi2RwELyrp/gkeWp0xl1/4YeM4WhZrJeL5W9RPhu2kIe3wjz99LQP3P9F8P8/6rYdejeDb5nSTbfNqrl730UiYEeTlGBWZdCkVZfDl41bDvCf+1qT8BnEv6cJbK4xyGw0Y7Czf2N/ZnfjO11s8AAAD//7Qj9vuIBwAA"}`)},
						},
					},
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Kind:  "BackupEntry",
							Name:  ptr.To("backupentry"),
							State: &runtime.RawExtension{Raw: []byte(`{"name":"backupentry"}`)},
						},
						{
							Kind:  "ContainerRuntime",
							Name:  ptr.To("containerruntime"),
							State: &runtime.RawExtension{Raw: []byte(`{"name":"containerruntime"}`)},
						},
						{
							Kind:    "ControlPlane",
							Name:    ptr.To("controlplane"),
							Purpose: ptr.To("normal"),
							State:   &runtime.RawExtension{Raw: []byte(`{"name":"controlplane"}`)},
						},
						{
							Kind:  "DNSRecord",
							Name:  ptr.To("dnsrecord"),
							State: &runtime.RawExtension{Raw: []byte(`{"name":"dnsrecord"}`)},
						},
						{
							Kind:  "Extension",
							Name:  ptr.To("extension"),
							State: &runtime.RawExtension{Raw: []byte(`{"name":"extension"}`)},
							Resources: []gardencorev1beta1.NamedResourceReference{{
								Name: "resource-ref1",
								ResourceRef: autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "ConfigMap",
									Name:       "extension-configmap",
								},
							}},
						},
						{
							Kind:  "Infrastructure",
							Name:  ptr.To("infrastructure"),
							State: &runtime.RawExtension{Raw: []byte(`{"name":"infrastructure"}`)},
						},
						{
							Kind:  "Network",
							Name:  ptr.To("network"),
							State: &runtime.RawExtension{Raw: []byte(`{"name":"network"}`)},
						},
						{
							Kind:    "OperatingSystemConfig",
							Name:    ptr.To("osc"),
							Purpose: ptr.To(""),
							State:   &runtime.RawExtension{Raw: []byte(`{"name":"osc"}`)},
						},
						{
							Kind:  "Worker",
							Name:  ptr.To("worker"),
							State: &runtime.RawExtension{Raw: []byte(`{"name":"worker"}`)},
						},
					},
					Resources: []gardencorev1beta1.ResourceData{{
						CrossVersionObjectReference: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "extension-configmap",
						},
						Data: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","data":{"some-data":"for-extension"},"kind":"ConfigMap","metadata":{"name":"extension-configmap","namespace":"shoot--my-project--my-shoot"}}`)},
					}},
				}
			})

			It("should compute the expected spec for both gardener and extensions data and overwrite the spec", func() {
				Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot, true)).To(Succeed())
				Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
				Expect(shootState.Spec).To(Equal(expectedSpec))
			})

			It("should compute expected spec for both gardener and extension data and overwrite the spec with no longer existing machine resources", func() {
				Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot, true)).To(Succeed())

				cleanupMachineObjectsFunc(ctx)
				Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot, true)).To(Succeed())
				Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())

				gardenerResourceData := v1beta1helper.GardenerResourceDataList(shootState.Spec.Gardener)
				gardenerResourceData.Delete("machine-state")
				expectedSpec.Gardener = gardenerResourceData

				Expect(shootState.Spec).To(Equal(expectedSpec))
			})

			It("should compute the expected spec for both gardener and extensions data and keep existing data in the spec", func() {
				Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot, false)).To(Succeed())
				Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())

				expectedSpec.Gardener = append(existingGardenerData, expectedSpec.Gardener...)
				expectedSpec.Extensions = append(existingExtensionsData, expectedSpec.Extensions...)
				expectedSpec.Resources = append(existingResourcesData, expectedSpec.Resources...)
				Expect(shootState.Spec).To(Equal(expectedSpec))
			})

			It("should compute the expected spec for both gardener and extension data and keep existing data in the spec if machine resources were deleted", func() {
				Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot, false)).To(Succeed())

				cleanupMachineObjectsFunc(ctx)
				Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot, false)).To(Succeed())
				Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())

				expectedSpec.Gardener = append(existingGardenerData, expectedSpec.Gardener...)
				expectedSpec.Extensions = append(existingExtensionsData, expectedSpec.Extensions...)
				expectedSpec.Resources = append(existingResourcesData, expectedSpec.Resources...)
				Expect(shootState.Spec).To(Equal(expectedSpec))
			})
		})
	})

	Describe("#Delete", func() {
		It("should do nothing when the shoot state does not exist", func() {
			Expect(Delete(ctx, fakeGardenClient, shoot)).To(Succeed())
		})

		It("should delete the shoot state", func() {
			Expect(fakeGardenClient.Create(ctx, shootState)).To(Succeed())
			Expect(Delete(ctx, fakeGardenClient, shoot)).To(Succeed())
			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(BeNotFoundError())
		})
	})
})

func newSecret(name, namespace string, withPersistLabel bool, withManagedByLabel bool) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{name: []byte("some-data")},
	}

	if withManagedByLabel {
		metav1.SetMetaDataLabel(&secret.ObjectMeta, "managed-by", "secrets-manager")
	}
	if withPersistLabel {
		metav1.SetMetaDataLabel(&secret.ObjectMeta, "persist", "true")
	}

	return secret
}

func createExtensionObject(
	ctx context.Context,
	fakeSeedClient client.Client,
	name, namespace string,
	obj client.Object,
	state *runtime.RawExtension,
	namedResourceReferences ...gardencorev1beta1.NamedResourceReference,
) {
	acc, err := extensions.Accessor(obj)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	acc.SetName(name)
	acc.SetNamespace(namespace)
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, obj)).To(Succeed())

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	acc.GetExtensionStatus().SetState(state)
	acc.GetExtensionStatus().SetResources(namedResourceReferences)
	ExpectWithOffset(1, fakeSeedClient.Patch(ctx, obj, patch)).To(Succeed())
}

func createMachineObjects(
	ctx context.Context,
	fakeSeedClient client.Client,
	namespace string,
) func(ctx context.Context) {
	machineDeployment1 := &machinev1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy1",
			Namespace: namespace,
		},
		Spec: machinev1alpha1.MachineDeploymentSpec{Replicas: 3},
	}
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, machineDeployment1)).To(Succeed())

	machineSet1 := &machinev1alpha1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "deploy1-set1",
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "MachineDeployment", Name: machineDeployment1.Name}},
			Annotations:     map[string]string{"some": "annotation"},
		},
		Status: machinev1alpha1.MachineSetStatus{
			Replicas:      1234,
			ReadyReplicas: 5678,
		},
	}
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, machineSet1)).To(Succeed())

	machineSet2 := &machinev1alpha1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "deploy1-set2",
			Namespace:   namespace,
			Labels:      map[string]string{"name": machineDeployment1.Name},
			Annotations: map[string]string{"some": "annotation"},
		},
		Status: machinev1alpha1.MachineSetStatus{
			Replicas:      1234,
			ReadyReplicas: 5678,
		},
	}
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, machineSet2)).To(Succeed())

	machine1 := &machinev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "deploy1-set1-machine1",
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "MachineSet", Name: machineSet1.Name}},
			Labels:          map[string]string{"node": "nodename"},
			Annotations:     map[string]string{"some": "annotation"},
		},
		Status: machinev1alpha1.MachineStatus{
			CurrentStatus: machinev1alpha1.CurrentStatus{TimeoutActive: true},
		},
	}
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, machine1)).To(Succeed())

	machine2 := &machinev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "deploy1-set2-machine2",
			Namespace:   namespace,
			Labels:      map[string]string{"name": machineDeployment1.Name},
			Annotations: map[string]string{"some": "annotation"},
		},
		Spec: machinev1alpha1.MachineSpec{ProviderID: "provider-id"},
		Status: machinev1alpha1.MachineStatus{
			CurrentStatus: machinev1alpha1.CurrentStatus{TimeoutActive: true},
		},
	}
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, machine2)).To(Succeed())

	machine3 := &machinev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "deploy1-set2-machine3",
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "MachineSet", Name: machineSet2.Name}},
		},
	}
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, machine3)).To(Succeed())

	machineDeployment2 := &machinev1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy2",
			Namespace: namespace,
		},
		Spec: machinev1alpha1.MachineDeploymentSpec{Replicas: 3},
	}
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, machineDeployment2)).To(Succeed())

	objectsToDelete := []client.Object{
		machineDeployment1, machineDeployment2, machineSet1, machineSet2, machine1, machine2, machine3,
	}
	return func(ctx context.Context) {
		for _, obj := range objectsToDelete {
			ExpectWithOffset(1, fakeSeedClient.Delete(ctx, obj)).To(Succeed())
		}
	}
}
