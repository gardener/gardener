// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/join"
	operationpkg "github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
)

var _ = Describe("Join", func() {
	Describe("#GetGardenerNodeAgentSecretName", func() {
		var (
			ctx = context.Background()

			fakeClient client.Client
			b          *botanist.GardenadmBotanist
			options    *Options

			shoot   *gardencorev1beta1.Shoot
			cluster *extensionsv1alpha1.Cluster
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			b = &botanist.GardenadmBotanist{
				Botanist: &botanistpkg.Botanist{
					Operation: &operationpkg.Operation{
						ShootClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
					},
				},
			}
			options = &Options{}

			shoot = &gardencorev1beta1.Shoot{}

			shootRaw, err := runtime.Encode(&json.Serializer{}, shoot)
			Expect(err).NotTo(HaveOccurred())

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{Raw: shootRaw},
				},
			}
		})

		When("Cluster object does not exist", func() {
			It("should fail when worker pool name is not set", func() {
				secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
				Expect(err).To(MatchError(ContainSubstring(`clusters.extensions.gardener.cloud "kube-system" not found`)))
				Expect(secretName).To(BeEmpty())
			})

			When("worker pool name is set", func() {
				BeforeEach(func() {
					options.WorkerPoolName = "some-pool-name"
				})

				It("should fail because there are no gardener-node-agent secrets", func() {
					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
					Expect(secretName).To(BeEmpty())
				})

				It("should succeed when there are gardener-node-agent secrets", func() {
					secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-node-agent-test-pool",
						Namespace: "kube-system",
						Labels: map[string]string{
							"gardener.cloud/role":        "operating-system-config",
							"worker.gardener.cloud/pool": options.WorkerPoolName,
						},
					}}

					Expect(fakeClient.Create(ctx, secret)).To(Succeed())

					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).NotTo(HaveOccurred())
					Expect(secretName).To(Equal(secret.Name))
				})
			})
		})

		When("Cluster object exists", func() {
			BeforeEach(func() {
				Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
			})

			When("control plane node should be joined", func() {
				BeforeEach(func() {
					options.ControlPlane = true
				})

				It("should fail when there is no control plane pool in the Shoot spec", func() {
					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).To(MatchError(ContainSubstring("no control plane worker pool found in Shoot manifest")))
					Expect(secretName).To(BeEmpty())
				})

				When("control plane pool is in Shoot spec", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "cp", ControlPlane: &gardencorev1beta1.WorkerControlPlane{}})

						var err error
						cluster.Spec.Shoot.Raw, err = runtime.Encode(&json.Serializer{}, shoot)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeClient.Update(ctx, cluster)).To(Succeed())
					})

					It("should fail because there are no gardener-node-agent secrets", func() {
						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
						Expect(secretName).To(BeEmpty())
					})

					It("should succeed when there are gardener-node-agent secrets", func() {
						secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
							Name:      "gardener-node-agent-cp",
							Namespace: "kube-system",
							Labels: map[string]string{
								"gardener.cloud/role":        "operating-system-config",
								"worker.gardener.cloud/pool": "cp",
							},
						}}

						Expect(fakeClient.Create(ctx, secret)).To(Succeed())

						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).NotTo(HaveOccurred())
						Expect(secretName).To(Equal(secret.Name))
					})
				})
			})

			When("worker node should be joined", func() {
				It("should fail when there is no non-control-plane pool in Shoot manifest", func() {
					secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
					Expect(err).To(MatchError(ContainSubstring("no non-control-plane pool found in Shoot manifest")))
					Expect(secretName).To(BeEmpty())
				})

				When("there are non-control-plane pools in Shoot manifest", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers,
							gardencorev1beta1.Worker{Name: "worker1"},
							gardencorev1beta1.Worker{Name: "worker2"},
						)

						var err error
						cluster.Spec.Shoot.Raw, err = runtime.Encode(&json.Serializer{}, shoot)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeClient.Update(ctx, cluster)).To(Succeed())
					})

					It("should fail because there are no gardener-node-agent secrets", func() {
						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).To(MatchError(ContainSubstring("no gardener-node-agent secrets found")))
						Expect(secretName).To(BeEmpty())
					})

					It("should succeed when there are gardener-node-agent secrets", func() {
						secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
							Name:      "gardener-node-agent-worker1",
							Namespace: "kube-system",
							Labels: map[string]string{
								"gardener.cloud/role":        "operating-system-config",
								"worker.gardener.cloud/pool": "worker1",
							},
						}}

						Expect(fakeClient.Create(ctx, secret)).To(Succeed())

						secretName, err := GetGardenerNodeAgentSecretName(ctx, options, b)
						Expect(err).NotTo(HaveOccurred())
						Expect(secretName).To(Equal(secret.Name))
					})
				})
			})
		})
	})
})
