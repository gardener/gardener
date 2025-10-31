// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("ControlPlane", func() {
	var b *GardenadmBotanist

	BeforeEach(func() {
		b = &GardenadmBotanist{}
	})

	Describe("#DiscoverKubernetesVersion", func() {
		var (
			controlPlaneAddress = "control-plane-address"
			token               = "token"
			caBundle            = []byte("ca-bundle")
		)

		It("should succeed discovering the version", func() {
			DeferCleanup(test.WithVar(&NewWithConfig, func(_ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return fakekubernetes.NewClientSetBuilder().WithVersion("1.33.0").Build(), nil
			}))

			version, err := b.DiscoverKubernetesVersion(controlPlaneAddress, caBundle, token)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(semver.MustParse("1.33.0")))
		})

		It("should fail creating the client set from the kubeconfig", func() {
			DeferCleanup(test.WithVar(&NewWithConfig, func(_ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return nil, fmt.Errorf("fake err")
			}))

			version, err := b.DiscoverKubernetesVersion(controlPlaneAddress, caBundle, token)
			Expect(err).To(MatchError(ContainSubstring("fake err")))
			Expect(version).To(BeNil())
		})

		It("should fail parsing the kubernetes version", func() {
			DeferCleanup(test.WithVar(&NewWithConfig, func(_ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
				return fakekubernetes.NewClientSetBuilder().WithVersion("cannot-parse").Build(), nil
			}))

			version, err := b.DiscoverKubernetesVersion(controlPlaneAddress, caBundle, token)
			Expect(err).To(MatchError(ContainSubstring("failed parsing semver version")))
			Expect(version).To(BeNil())
		})
	})

	Describe("#WaitUntilControlPlaneDeploymentsReady", func() {
		var (
			ctx           = context.Background()
			fakeClient    client.Client
			fakeClientSet kubernetes.Interface

			podName  = "static-pod"
			poolName = "pool"
			nodeName = "node"

			staticPod               *corev1.Pod
			gardenerNodeAgentSecret *corev1.Secret
			node                    *corev1.Node
			managedResource         *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

			b.Botanist = &botanistpkg.Botanist{Operation: &operation.Operation{
				SeedClientSet:  fakeClientSet,
				ShootClientSet: fakeClientSet,
				Shoot: &shoot.Shoot{
					ControlPlaneNamespace: "kube-system",
				},
			}}

			b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{Name: poolName}},
					},
				},
			})

			DeferCleanup(test.WithVars(
				&botanistpkg.IntervalWaitOperatingSystemConfigUpdated, 5*time.Millisecond,
				&botanistpkg.GetTimeoutWaitOperatingSystemConfigUpdated, func(_ *shoot.Shoot) time.Duration { return 10 * time.Millisecond },
			))

			staticPod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        podName + "-" + nodeName,
					Namespace:   "kube-system",
					Labels:      map[string]string{"static-pod": "true"},
					Annotations: map[string]string{"gardener.cloud/config.mirror": "hash1"},
				},
				Spec: corev1.PodSpec{NodeName: nodeName},
			}
			gardenerNodeAgentSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener-node-agent-" + poolName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"worker.gardener.cloud/pool": poolName,
						"gardener.cloud/role":        "operating-system-config",
					},
					Annotations: map[string]string{"checksum/data-script": "foo"},
				},
			}
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        nodeName,
					Labels:      map[string]string{"worker.gardener.cloud/pool": poolName},
					Annotations: map[string]string{"checksum/cloud-config-data": gardenerNodeAgentSecret.Annotations["checksum/data-script"]},
				},
			}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "shoot-gardener-node-agent",
					Namespace:  "kube-system",
					Generation: 1,
				},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
						{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}

			Expect(fakeClient.Create(ctx, staticPod)).To(Succeed())
			Expect(fakeClient.Create(ctx, gardenerNodeAgentSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
		})

		It("should succeed when the hashes match and the OSC is up-to-date", func() {
			b.staticPodNameToHash = map[string]string{podName: staticPod.Annotations["gardener.cloud/config.mirror"]}

			Expect(b.WaitUntilControlPlaneDeploymentsReady(ctx)).To(Succeed())
		})

		It("should fail when the hashes is outdated", func() {
			b.staticPodNameToHash = map[string]string{podName: "some-other-hash"}

			Expect(b.WaitUntilControlPlaneDeploymentsReady(ctx)).To(MatchError(ContainSubstring("not all static pods have been updated yet")))
		})

		It("should fail when a desired static pod is missing", func() {
			b.staticPodNameToHash = map[string]string{
				podName: staticPod.Annotations["gardener.cloud/config.mirror"],
				"foo":   "bar",
			}

			Expect(b.WaitUntilControlPlaneDeploymentsReady(ctx)).To(MatchError(ContainSubstring("not all static pods have been updated yet")))
		})

		It("should fail when there is an unexpected static pod", func() {
			b.staticPodNameToHash = map[string]string{podName: staticPod.Annotations["gardener.cloud/config.mirror"]}

			Expect(fakeClient.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "some-other-pod-" + nodeName,
					Namespace:   "kube-system",
					Labels:      map[string]string{"static-pod": "true"},
					Annotations: map[string]string{"gardener.cloud/config.mirror": "foo"},
				},
				Spec: corev1.PodSpec{NodeName: nodeName},
			})).To(Succeed())

			Expect(b.WaitUntilControlPlaneDeploymentsReady(ctx)).To(MatchError(ContainSubstring("not all static pods have been updated yet")))
		})

		It("should fail when the operating system config is not up-to-date", func() {
			b.staticPodNameToHash = map[string]string{podName: staticPod.Annotations["gardener.cloud/config.mirror"]}

			node.Annotations["checksum/cloud-config-data"] = "some-outdated-checksum"
			Expect(fakeClient.Update(ctx, node)).To(Succeed())

			Expect(b.WaitUntilControlPlaneDeploymentsReady(ctx)).To(MatchError(ContainSubstring(`the last successfully applied operating system config on node "` + nodeName + `" is outdated`)))
		})
	})
})
