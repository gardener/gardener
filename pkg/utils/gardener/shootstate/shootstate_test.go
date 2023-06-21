// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootstate_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
		It("should deploy a ShootState with an empty spec when there is nothing to persist", func() {
			Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot)).To(Succeed())
			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(gardencorev1beta1.ShootStateSpec{}))
			Expect(shootState.Annotations).To(HaveKeyWithValue("gardener.cloud/timestamp", fakeClock.Now().UTC().Format(time.RFC3339)))
		})

		It("should compute the expected spec for both gardener and extensions data", func() {
			By("Creating Gardener data")
			Expect(fakeSeedClient.Create(ctx, newSecret("secret1", seedNamespace, true))).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, newSecret("secret2", seedNamespace, false))).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, newSecret("secret3", seedNamespace, true))).To(Succeed())

			By("Creating extensions data")
			purposeNormal := extensionsv1alpha1.Normal
			purposeExposure := extensionsv1alpha1.Exposure
			createExtensionObject(ctx, fakeSeedClient, "backupentry", seedNamespace, &extensionsv1alpha1.BackupEntry{})
			createExtensionObject(ctx, fakeSeedClient, "containerruntime", seedNamespace, &extensionsv1alpha1.ContainerRuntime{})
			createExtensionObject(ctx, fakeSeedClient, "controlplane", seedNamespace, &extensionsv1alpha1.ControlPlane{Spec: extensionsv1alpha1.ControlPlaneSpec{Purpose: &purposeNormal}})
			createExtensionObject(ctx, fakeSeedClient, "controlplane-exposure", seedNamespace, &extensionsv1alpha1.ControlPlane{Spec: extensionsv1alpha1.ControlPlaneSpec{Purpose: &purposeExposure}})
			createExtensionObject(ctx, fakeSeedClient, "dnsrecord", seedNamespace, &extensionsv1alpha1.DNSRecord{})
			createExtensionObject(ctx, fakeSeedClient, "extension", seedNamespace, &extensionsv1alpha1.Extension{}, gardencorev1beta1.NamedResourceReference{Name: "resource-ref1", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "ConfigMap", APIVersion: "v1", Name: "extension-configmap"}})
			Expect(fakeSeedClient.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "extension-configmap", Namespace: seedNamespace}, Data: map[string]string{"some-data": "for-extension"}})).To(Succeed())
			createExtensionObject(ctx, fakeSeedClient, "infrastructure", seedNamespace, &extensionsv1alpha1.Infrastructure{})
			createExtensionObject(ctx, fakeSeedClient, "network", seedNamespace, &extensionsv1alpha1.Network{})
			createExtensionObject(ctx, fakeSeedClient, "osc", seedNamespace, &extensionsv1alpha1.OperatingSystemConfig{})
			createExtensionObject(ctx, fakeSeedClient, "worker", seedNamespace, &extensionsv1alpha1.Worker{})

			Expect(Deploy(ctx, fakeClock, fakeGardenClient, fakeSeedClient, shoot)).To(Succeed())
			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(shootState), shootState)).To(Succeed())
			Expect(shootState.Spec).To(Equal(gardencorev1beta1.ShootStateSpec{
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
						Labels: map[string]string{"managed-by": "secrets-manager", "persist": "true"},
					},
				},
				Extensions: []gardencorev1beta1.ExtensionResourceState{
					{
						Kind:  "BackupEntry",
						Name:  pointer.String("backupentry"),
						State: &runtime.RawExtension{Raw: []byte(`{"name":"backupentry"}`)},
					},
					{
						Kind:  "ContainerRuntime",
						Name:  pointer.String("containerruntime"),
						State: &runtime.RawExtension{Raw: []byte(`{"name":"containerruntime"}`)},
					},
					{
						Kind:    "ControlPlane",
						Name:    pointer.String("controlplane"),
						Purpose: pointer.String("normal"),
						State:   &runtime.RawExtension{Raw: []byte(`{"name":"controlplane"}`)},
					},
					{
						Kind:    "ControlPlane",
						Name:    pointer.String("controlplane-exposure"),
						Purpose: pointer.String("exposure"),
						State:   &runtime.RawExtension{Raw: []byte(`{"name":"controlplane-exposure"}`)},
					},
					{
						Kind:  "DNSRecord",
						Name:  pointer.String("dnsrecord"),
						State: &runtime.RawExtension{Raw: []byte(`{"name":"dnsrecord"}`)},
					},
					{
						Kind:  "Extension",
						Name:  pointer.String("extension"),
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
						Name:  pointer.String("infrastructure"),
						State: &runtime.RawExtension{Raw: []byte(`{"name":"infrastructure"}`)},
					},
					{
						Kind:  "Network",
						Name:  pointer.String("network"),
						State: &runtime.RawExtension{Raw: []byte(`{"name":"network"}`)},
					},
					{
						Kind:    "OperatingSystemConfig",
						Name:    pointer.String("osc"),
						Purpose: pointer.String(""),
						State:   &runtime.RawExtension{Raw: []byte(`{"name":"osc"}`)},
					},
					{
						Kind:  "Worker",
						Name:  pointer.String("worker"),
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
			}))
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

func newSecret(name, namespace string, withPersistLabel bool) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"managed-by": "secrets-manager"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{name: []byte("some-data")},
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
	namedResourceReferences ...gardencorev1beta1.NamedResourceReference,
) {
	acc, err := extensions.Accessor(obj)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	acc.SetName(name)
	acc.SetNamespace(namespace)
	ExpectWithOffset(1, fakeSeedClient.Create(ctx, obj)).To(Succeed())

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	acc.GetExtensionStatus().SetState(&runtime.RawExtension{Raw: []byte(`{"name":"` + name + `"}`)})
	acc.GetExtensionStatus().SetResources(namedResourceReferences)
	ExpectWithOffset(1, fakeSeedClient.Patch(ctx, obj, patch)).To(Succeed())
}
