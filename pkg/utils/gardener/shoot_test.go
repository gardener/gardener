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

package gardener_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/garden"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

var _ = Describe("Shoot", func() {
	DescribeTable("#RespectSyncPeriodOverwrite",
		func(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot)).To(match)
		},

		Entry("respect overwrite",
			true,
			&gardencorev1beta1.Shoot{},
			BeTrue()),
		Entry("don't respect overwrite",
			false,
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("don't respect overwrite but garden namespace",
			false,
			&gardencorev1beta1.Shoot{ObjectMeta: kubernetesutils.ObjectMeta(v1beta1constants.GardenNamespace, "foo")},
			BeTrue()),
	)

	DescribeTable("#ShouldIgnoreShoot",
		func(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)).To(match)
		},

		Entry("respect overwrite with annotation",
			true,
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.ShootIgnore: "true"}}},
			BeTrue()),
		Entry("respect overwrite with wrong annotation",
			true,
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.ShootIgnore: "foo"}}},
			BeFalse()),
		Entry("respect overwrite with no annotation",
			true,
			&gardencorev1beta1.Shoot{},
			BeFalse()),
	)

	DescribeTable("#IsShootFailedAndUpToDate",
		func(shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(IsShootFailedAndUpToDate(shoot)).To(match)
		},

		Entry("no last operation",
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("with last operation but not in failed state",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state but not at latest generation",
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state and matching generation but not latest gardener version",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					Gardener: gardencorev1beta1.Gardener{
						Version: version.Get().GitVersion + "foo",
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state and matching generation and latest gardener version",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					Gardener: gardencorev1beta1.Gardener{
						Version: version.Get().GitVersion,
					},
				},
			},
			BeTrue()),
	)

	DescribeTable("#IsObservedAtLatestGenerationAndSucceeded",
		func(shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(IsObservedAtLatestGenerationAndSucceeded(shoot)).To(match)
		},

		Entry("not at observed generation",
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			},
			BeFalse()),
		Entry("last operation state not succeeded",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateError,
					},
				},
			},
			BeFalse()),
		Entry("observed at latest generation and no last operation state",
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("observed at latest generation and last operation state succeeded",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
			},
			BeTrue()),
	)

	DescribeTable("#SyncPeriodOfShoot",
		func(respectSyncPeriodOverwrite bool, defaultMinSyncPeriod time.Duration, shoot *gardencorev1beta1.Shoot, expected time.Duration) {
			Expect(SyncPeriodOfShoot(respectSyncPeriodOverwrite, defaultMinSyncPeriod, shoot)).To(Equal(expected))
		},

		Entry("don't respect overwrite",
			false,
			1*time.Second,
			&gardencorev1beta1.Shoot{},
			1*time.Second),
		Entry("respect overwrite but no overwrite",
			true,
			1*time.Second,
			&gardencorev1beta1.Shoot{},
			1*time.Second),
		Entry("respect overwrite but overwrite invalid",
			true,
			1*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1beta1constants.ShootSyncPeriod: "foo"},
				},
			},
			1*time.Second),
		Entry("respect overwrite but overwrite too short",
			true,
			2*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1beta1constants.ShootSyncPeriod: (1 * time.Second).String()},
				},
			},
			2*time.Second),
		Entry("respect overwrite with longer overwrite",
			true,
			2*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1beta1constants.ShootSyncPeriod: (3 * time.Second).String()},
				},
			},
			3*time.Second),
	)

	Describe("#EffectiveMaintenanceTimeWindow", func() {
		It("should shorten the end of the time window by 15 minutes", func() {
			var (
				begin = timewindow.NewMaintenanceTime(0, 0, 0)
				end   = timewindow.NewMaintenanceTime(1, 0, 0)
			)

			Expect(EffectiveMaintenanceTimeWindow(timewindow.NewMaintenanceTimeWindow(begin, end))).
				To(Equal(timewindow.NewMaintenanceTimeWindow(begin, timewindow.NewMaintenanceTime(0, 45, 0))))
		})
	})

	DescribeTable("#EffectiveShootMaintenanceTimeWindow",
		func(shoot *gardencorev1beta1.Shoot, window *timewindow.MaintenanceTimeWindow) {
			Expect(EffectiveShootMaintenanceTimeWindow(shoot)).To(Equal(window))
		},

		Entry("no maintenance section",
			&gardencorev1beta1.Shoot{},
			timewindow.AlwaysTimeWindow),
		Entry("no time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{},
				},
			},
			timewindow.AlwaysTimeWindow),
		Entry("invalid time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{},
					},
				},
			},
			timewindow.AlwaysTimeWindow),
		Entry("valid time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "010000+0000",
							End:   "020000+0000",
						},
					},
				},
			},
			timewindow.NewMaintenanceTimeWindow(
				timewindow.NewMaintenanceTime(1, 0, 0),
				timewindow.NewMaintenanceTime(1, 45, 0))),
	)

	DescribeTable("#GetShootNameFromOwnerReferences",
		func(ownerRefs []metav1.OwnerReference, expectedName string) {
			obj := &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: ownerRefs,
				},
			}
			name := GetShootNameFromOwnerReferences(obj)
			Expect(name).To(Equal(expectedName))
		},
		Entry("object is owned by shoot", []metav1.OwnerReference{{Kind: "Shoot", Name: "foo"}}, "foo"),
		Entry("object has no OwnerReferences", nil, ""),
		Entry("object is not owned by shoot", []metav1.OwnerReference{{Kind: "Foo", Name: "foo"}}, ""),
	)

	Describe("#NodeLabelsForWorkerPool", func() {
		var workerPool gardencorev1beta1.Worker

		BeforeEach(func() {
			workerPool = gardencorev1beta1.Worker{
				Name: "worker",
				Machine: gardencorev1beta1.Machine{
					Architecture: pointer.String("arm64"),
				},
				SystemComponents: &gardencorev1beta1.WorkerSystemComponents{
					Allow: true,
				},
			}
		})

		It("should maintain the common labels", func() {
			Expect(NodeLabelsForWorkerPool(workerPool, false)).To(And(
				HaveKeyWithValue("node.kubernetes.io/role", "node"),
				HaveKeyWithValue("kubernetes.io/arch", "arm64"),
				HaveKeyWithValue("networking.gardener.cloud/node-local-dns-enabled", "false"),
				HaveKeyWithValue("worker.gardener.cloud/system-components", "true"),
				HaveKeyWithValue("worker.gardener.cloud/pool", "worker"),
				HaveKeyWithValue("worker.garden.sapcloud.io/group", "worker"),
			))
		})

		It("should add user-specified labels", func() {
			workerPool.Labels = map[string]string{
				"test": "foo",
				"bar":  "baz",
			}
			Expect(NodeLabelsForWorkerPool(workerPool, false)).To(And(
				HaveKeyWithValue("test", "foo"),
				HaveKeyWithValue("bar", "baz"),
			))
		})

		It("should not add system components label if they are not allowed", func() {
			workerPool.SystemComponents.Allow = false
			Expect(NodeLabelsForWorkerPool(workerPool, false)).NotTo(
				HaveKey("worker.gardener.cloud/system-components"),
			)
		})

		It("should correctly handle the node-local-dns label", func() {
			Expect(NodeLabelsForWorkerPool(workerPool, false)).To(
				HaveKeyWithValue("networking.gardener.cloud/node-local-dns-enabled", "false"),
			)
			Expect(NodeLabelsForWorkerPool(workerPool, true)).To(
				HaveKeyWithValue("networking.gardener.cloud/node-local-dns-enabled", "true"),
			)
		})

		It("should correctly add the CRI labels", func() {
			workerPool.CRI = &gardencorev1beta1.CRI{
				Name: "containerd",
				ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
					{
						Type: "gvisor",
					},
					{
						Type: "kata",
					},
				},
			}
			Expect(NodeLabelsForWorkerPool(workerPool, false)).To(And(
				HaveKeyWithValue("worker.gardener.cloud/cri-name", "containerd"),
				HaveKeyWithValue("containerruntime.worker.gardener.cloud/gvisor", "true"),
				HaveKeyWithValue("containerruntime.worker.gardener.cloud/kata", "true"),
			))
		})
	})

	Describe("#GetShootProjectSecretSuffixes", func() {
		It("should return the expected list", func() {
			Expect(GetShootProjectSecretSuffixes()).To(ConsistOf("kubeconfig", "ca-cluster", "ssh-keypair", "ssh-keypair.old", "monitoring"))
		})
	})

	Describe("#ComputeShootProjectSecretName", func() {
		It("should compute the expected name", func() {
			Expect(ComputeShootProjectSecretName("foo", "bar")).To(Equal("foo.bar"))
		})
	})

	DescribeTable("#IsShootProjectSecret",
		func(name, expectedShootName string, expectedOK bool) {
			shootName, ok := IsShootProjectSecret(name)
			Expect(shootName).To(Equal(expectedShootName))
			Expect(ok).To(Equal(expectedOK))
		},
		Entry("no suffix", "foo", "", false),
		Entry("unrelated suffix", "foo.bar", "", false),
		Entry("wrong suffix delimiter", "foo:kubeconfig", "", false),
		Entry("kubeconfig suffix", "foo.kubeconfig", "foo", true),
		Entry("ca-cluster suffix", "baz.ca-cluster", "baz", true),
		Entry("ssh-keypair suffix", "bar.ssh-keypair", "bar", true),
		Entry("monitoring suffix", "baz.monitoring", "baz", true),
	)

	Context("ShootAccessSecret", func() {
		Describe("#NewShootAccessSecret", func() {
			var (
				name      = "name"
				namespace = "namespace"
			)

			DescribeTable("default name/namespace",
				func(prefix string) {

					Expect(NewShootAccessSecret(prefix+name, namespace)).To(Equal(&ShootAccessSecret{
						Secret:             &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-" + name, Namespace: namespace}},
						ServiceAccountName: name,
					}))
				},

				Entry("no prefix", ""),
				Entry("prefix", "shoot-access-"),
			)

			It("should override the name and namespace", func() {
				Expect(NewShootAccessSecret(name, namespace).
					WithNameOverride("other-name").
					WithNamespaceOverride("other-namespace"),
				).To(Equal(&ShootAccessSecret{
					Secret:             &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other-name", Namespace: "other-namespace"}},
					ServiceAccountName: name,
				}))
			})
		})

		Describe("#Reconcile", func() {
			var (
				ctx                     = context.TODO()
				fakeClient              client.Client
				shootAccessSecret       *ShootAccessSecret
				serviceAccountName      = "serviceaccount"
				tokenExpirationDuration = "1234h"
			)

			BeforeEach(func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
				shootAccessSecret = NewShootAccessSecret("secret", "namespace").WithServiceAccountName(serviceAccountName)
			})

			Describe("#Reconcile", func() {
				validate := func() {
					Expect(shootAccessSecret.Reconcile(ctx, fakeClient)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret)).To(Succeed())
					Expect(shootAccessSecret.Secret.Type).To(Equal(corev1.SecretTypeOpaque))
					Expect(shootAccessSecret.Secret.Labels).To(HaveKeyWithValue("resources.gardener.cloud/purpose", "token-requestor"))
					Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/name", serviceAccountName))
					Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/namespace", "kube-system"))
				}

				Context("create", func() {
					BeforeEach(func() {
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shootAccessSecret.Secret), shootAccessSecret.Secret)).To(BeNotFoundError())
					})

					It("should work w/o settings", func() {
						validate()
					})

					It("should work w/ token expiration duration", func() {
						shootAccessSecret.WithTokenExpirationDuration(tokenExpirationDuration)
						validate()
						Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-expiration-duration", tokenExpirationDuration))
					})

					It("should work w/ kubeconfig", func() {
						kubeconfig := &clientcmdv1.Config{}
						kubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
						Expect(err).NotTo(HaveOccurred())

						shootAccessSecret.WithKubeconfig(kubeconfig)
						validate()
						Expect(shootAccessSecret.Secret.Data).To(HaveKeyWithValue("kubeconfig", kubeconfigRaw))
					})

					It("should work w/ target secret", func() {
						targetSecretName, targetSecretNamespace := "tname", "tnamespace"

						shootAccessSecret.WithTargetSecret(targetSecretName, targetSecretNamespace)
						validate()
						Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-name", targetSecretName))
						Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-namespace", targetSecretNamespace))
					})
				})

				Context("update", func() {
					BeforeEach(func() {
						shootAccessSecret.Secret.Type = corev1.SecretTypeServiceAccountToken
						shootAccessSecret.Secret.Annotations = map[string]string{"foo": "bar"}
						shootAccessSecret.Secret.Labels = map[string]string{"bar": "foo"}
						Expect(fakeClient.Create(ctx, shootAccessSecret.Secret)).To(Succeed())
					})

					AfterEach(func() {
						Expect(shootAccessSecret.Secret.Labels).To(HaveKeyWithValue("bar", "foo"))
						Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("foo", "bar"))
					})

					It("should work w/o settings", func() {
						validate()
					})

					It("should work w/ token expiration duration", func() {
						shootAccessSecret.WithTokenExpirationDuration(tokenExpirationDuration)
						validate()
						Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-expiration-duration", tokenExpirationDuration))
					})

					It("should work w/ kubeconfig", func() {
						existingAuthInfos := []clientcmdv1.NamedAuthInfo{{AuthInfo: clientcmdv1.AuthInfo{Token: "some-token"}}}

						existingKubeconfig := &clientcmdv1.Config{AuthInfos: existingAuthInfos}
						existingKubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, existingKubeconfig)
						Expect(err).NotTo(HaveOccurred())

						shootAccessSecret.Secret.Data = map[string][]byte{"kubeconfig": existingKubeconfigRaw}
						Expect(fakeClient.Update(ctx, shootAccessSecret.Secret)).To(Succeed())

						newKubeconfig := existingKubeconfig.DeepCopy()
						newKubeconfig.AuthInfos = nil
						shootAccessSecret.WithKubeconfig(newKubeconfig)

						expectedKubeconfig := newKubeconfig.DeepCopy()
						expectedKubeconfig.AuthInfos = existingAuthInfos
						expectedKubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, expectedKubeconfig)
						Expect(err).NotTo(HaveOccurred())

						validate()
						Expect(shootAccessSecret.Secret.Data).To(HaveKeyWithValue("kubeconfig", expectedKubeconfigRaw))
					})

					It("should delete the kubeconfig key", func() {
						shootAccessSecret.Secret.Data = map[string][]byte{"kubeconfig": []byte("foo")}
						Expect(fakeClient.Update(ctx, shootAccessSecret.Secret)).To(Succeed())

						validate()
						Expect(shootAccessSecret.Secret.Data).NotTo(HaveKey("kubeconfig"))
					})

					It("should delete the token key", func() {
						shootAccessSecret.Secret.Data = map[string][]byte{"token": []byte("foo")}
						Expect(fakeClient.Update(ctx, shootAccessSecret.Secret)).To(Succeed())

						shootAccessSecret.WithKubeconfig(&clientcmdv1.Config{})

						validate()
						Expect(shootAccessSecret.Secret.Data).NotTo(HaveKey("token"))
					})

					It("should work w/ target secret", func() {
						targetSecretName, targetSecretNamespace := "tname", "tnamespace"

						shootAccessSecret.WithTargetSecret(targetSecretName, targetSecretNamespace)
						validate()
						Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-name", targetSecretName))
						Expect(shootAccessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-namespace", targetSecretNamespace))
					})
				})
			})
		})
	})

	Describe("#InjectGenericKubeconfig", func() {
		var (
			genericTokenKubeconfigSecretName = "generic-token-kubeconfig-12345"
			tokenSecretName                  = "tokensecret"
			containerName1                   = "container1"
			containerName2                   = "container2"

			podSpec = corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: containerName1},
					{Name: containerName2},
				},
			}

			pod = &corev1.Pod{
				Spec: podSpec,
			}
			deployment = &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			deploymentV1beta2 = &appsv1beta2.Deployment{
				Spec: appsv1beta2.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			deploymentV1beta1 = &appsv1beta1.Deployment{
				Spec: appsv1beta1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			statefulSet = &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			statefulSetV1beta2 = &appsv1beta2.StatefulSet{
				Spec: appsv1beta2.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			statefulSetV1beta1 = &appsv1beta1.StatefulSet{
				Spec: appsv1beta1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			daemonSet = &appsv1.DaemonSet{
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			daemonSetV1beta2 = &appsv1beta2.DaemonSet{
				Spec: appsv1beta2.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			job = &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			cronJob = &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
			cronJobV1beta1 = &batchv1beta1.CronJob{
				Spec: batchv1beta1.CronJobSpec{
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
		)

		It("should do nothing because object is not handled", func() {
			Expect(InjectGenericKubeconfig(&corev1.Service{}, genericTokenKubeconfigSecretName, tokenSecretName)).To(MatchError(ContainSubstring("unhandled object type")))
		})

		DescribeTable("should behave properly",
			func(obj runtime.Object, podSpec *corev1.PodSpec, expectedVolumeMountInContainer1, expectedVolumeMountInContainer2 bool, containerNames ...string) {
				Expect(InjectGenericKubeconfig(obj, genericTokenKubeconfigSecretName, tokenSecretName, containerNames...)).To(Succeed())

				Expect(podSpec.Volumes).To(ContainElement(corev1.Volume{
					Name: "kubeconfig",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: pointer.Int32(420),
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: genericTokenKubeconfigSecretName,
										},
										Items: []corev1.KeyToPath{{
											Key:  "kubeconfig",
											Path: "kubeconfig",
										}},
										Optional: pointer.Bool(false),
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: tokenSecretName,
										},
										Items: []corev1.KeyToPath{{
											Key:  "token",
											Path: "token",
										}},
										Optional: pointer.Bool(false),
									},
								},
							},
						},
					},
				}))

				if expectedVolumeMountInContainer1 {
					Expect(podSpec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
						Name:      "kubeconfig",
						MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
						ReadOnly:  true,
					}))
				}
			},

			Entry("corev1.Pod, all containers", pod, &pod.Spec, true, true),
			Entry("corev1.Pod, only container 1", pod, &pod.Spec, true, false, containerName1),
			Entry("corev1.Pod, only container 2", pod, &pod.Spec, false, true, containerName2),

			Entry("appsv1.Deployment, all containers", deployment, &deployment.Spec.Template.Spec, true, true),
			Entry("appsv1.Deployment, only container 1", deployment, &deployment.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1.Deployment, only container 2", deployment, &deployment.Spec.Template.Spec, false, true, containerName2),

			Entry("appsv1beta2.Deployment, all containers", deploymentV1beta2, &deploymentV1beta2.Spec.Template.Spec, true, true),
			Entry("appsv1beta2.Deployment, only container 1", deploymentV1beta2, &deploymentV1beta2.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1beta2.Deployment, only container 2", deploymentV1beta2, &deploymentV1beta2.Spec.Template.Spec, false, true, containerName2),

			Entry("appsv1beta1.Deployment, all containers", deploymentV1beta1, &deploymentV1beta1.Spec.Template.Spec, true, true),
			Entry("appsv1beta1.Deployment, only container 1", deploymentV1beta1, &deploymentV1beta1.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1beta1.Deployment, only container 2", deploymentV1beta1, &deploymentV1beta1.Spec.Template.Spec, false, true, containerName2),

			Entry("appsv1.StatefulSet, all containers", statefulSet, &statefulSet.Spec.Template.Spec, true, true),
			Entry("appsv1.StatefulSet, only container 1", statefulSet, &statefulSet.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1.StatefulSet, only container 2", statefulSet, &statefulSet.Spec.Template.Spec, false, true, containerName2),

			Entry("appsv1beta2.StatefulSet, all containers", statefulSetV1beta2, &statefulSetV1beta2.Spec.Template.Spec, true, true),
			Entry("appsv1beta2.StatefulSet, only container 1", statefulSetV1beta2, &statefulSetV1beta2.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1beta2.StatefulSet, only container 2", statefulSetV1beta2, &statefulSetV1beta2.Spec.Template.Spec, false, true, containerName2),

			Entry("appsv1beta1.StatefulSet, all containers", statefulSetV1beta1, &statefulSetV1beta1.Spec.Template.Spec, true, true),
			Entry("appsv1beta1.StatefulSet, only container 1", statefulSetV1beta1, &statefulSetV1beta1.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1beta1.StatefulSet, only container 2", statefulSetV1beta1, &statefulSetV1beta1.Spec.Template.Spec, false, true, containerName2),

			Entry("appsv1.DaemonSet, all containers", daemonSet, &daemonSet.Spec.Template.Spec, true, true),
			Entry("appsv1.DaemonSet, only container 1", daemonSet, &daemonSet.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1.DaemonSet, only container 2", daemonSet, &daemonSet.Spec.Template.Spec, false, true, containerName2),

			Entry("appsv1beta2.DaemonSet, all containers", daemonSetV1beta2, &daemonSetV1beta2.Spec.Template.Spec, true, true),
			Entry("appsv1beta2.DaemonSet, only container 1", daemonSetV1beta2, &daemonSetV1beta2.Spec.Template.Spec, true, false, containerName1),
			Entry("appsv1beta2.DaemonSet, only container 2", daemonSetV1beta2, &daemonSetV1beta2.Spec.Template.Spec, false, true, containerName2),

			Entry("batchv1.Job, all containers", job, &job.Spec.Template.Spec, true, true),
			Entry("batchv1.Job, only container 1", job, &job.Spec.Template.Spec, true, false, containerName1),
			Entry("batchv1.Job, only container 2", job, &job.Spec.Template.Spec, false, true, containerName2),

			Entry("batchv1.CronJob, all containers", cronJob, &cronJob.Spec.JobTemplate.Spec.Template.Spec, true, true),
			Entry("batchv1.CronJob, only container 1", cronJob, &cronJob.Spec.JobTemplate.Spec.Template.Spec, true, false, containerName1),
			Entry("batchv1.CronJob, only container 2", cronJob, &cronJob.Spec.JobTemplate.Spec.Template.Spec, false, true, containerName2),

			Entry("batchv1beta1.CronJob, all containers", cronJobV1beta1, &cronJobV1beta1.Spec.JobTemplate.Spec.Template.Spec, true, true),
			Entry("batchv1beta1.CronJob, only container 1", cronJobV1beta1, &cronJobV1beta1.Spec.JobTemplate.Spec.Template.Spec, true, false, containerName1),
			Entry("batchv1beta1.CronJob, only container 2", cronJobV1beta1, &cronJobV1beta1.Spec.JobTemplate.Spec.Template.Spec, false, true, containerName2),
		)
	})

	Describe("#GetShootSeedNames", func() {
		It("returns nil for other objects than Shoot", func() {
			specSeedName, statusSeedName := GetShootSeedNames(&corev1.Secret{})
			Expect(specSeedName).To(BeNil())
			Expect(statusSeedName).To(BeNil())
		})

		It("returns the correct seed names of a Shoot", func() {
			specSeedName, statusSeedName := GetShootSeedNames(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String("spec"),
				},
				Status: gardencorev1beta1.ShootStatus{
					SeedName: pointer.String("status"),
				},
			})
			Expect(specSeedName).To(Equal(pointer.String("spec")))
			Expect(statusSeedName).To(Equal(pointer.String("status")))
		})
	})

	Describe("#ExtractSystemComponentsTolerations", func() {
		It("should return no tolerations when workers are 'nil'", func() {
			Expect(ExtractSystemComponentsTolerations(nil)).To(BeEmpty())
		})

		It("should return no tolerations when workers are empty", func() {
			Expect(ExtractSystemComponentsTolerations([]gardencorev1beta1.Worker{})).To(BeEmpty())
		})

		It("should return no tolerations when no taints are defined for system worker group", func() {
			Expect(ExtractSystemComponentsTolerations([]gardencorev1beta1.Worker{
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
				},
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: false},
					Taints: []corev1.Taint{
						{
							Key:    "someKey",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			})).To(BeEmpty())
		})

		It("should return tolerations when taints are defined for system worker group", func() {
			Expect(ExtractSystemComponentsTolerations([]gardencorev1beta1.Worker{
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
					Taints: []corev1.Taint{
						{
							Key:    "someKey",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: false},
					Taints: []corev1.Taint{
						{
							Key:    "someKey",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			})).To(ConsistOf(corev1.Toleration{
				Key:      "someKey",
				Operator: corev1.TolerationOpEqual,
				Value:    "someValue",
				Effect:   corev1.TaintEffectNoExecute,
			}))
		})

		It("should return tolerations when taints are defined multiple times for system worker group", func() {
			Expect(ExtractSystemComponentsTolerations([]gardencorev1beta1.Worker{
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
					Taints: []corev1.Taint{
						{
							Key:    "someKey",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
					Taints: []corev1.Taint{
						{
							Key:    "someKey",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
					Taints: []corev1.Taint{
						{
							Key:    "someKey",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			})).To(ConsistOf(
				corev1.Toleration{
					Key:      "someKey",
					Operator: corev1.TolerationOpEqual,
					Value:    "someValue",
					Effect:   corev1.TaintEffectNoExecute,
				},
				corev1.Toleration{
					Key:      "someKey",
					Operator: corev1.TolerationOpEqual,
					Value:    "someValue",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			))
		})
	})

	DescribeTable("#ConstructInternalClusterDomain",
		func(shootName, shootProject, internalDomain, expected string) {
			Expect(ConstructInternalClusterDomain(shootName, shootProject, &garden.Domain{Domain: internalDomain})).To(Equal(expected))
		},

		Entry("with internal domain key", "foo", "bar", "internal.nip.io", "foo.bar.internal.nip.io"),
		Entry("without internal domain key", "foo", "bar", "nip.io", "foo.bar.internal.nip.io"),
	)

	Describe("#ConstructExternalClusterDomain", func() {
		It("should return nil", func() {
			Expect(ConstructExternalClusterDomain(&gardencorev1beta1.Shoot{})).To(BeNil())
		})

		It("should return the constructed domain", func() {
			var (
				domain = "foo.bar.com"
				shoot  = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: &domain,
						},
					},
				}
			)

			Expect(ConstructExternalClusterDomain(shoot)).To(Equal(&domain))
		})
	})

	var (
		defaultDomainProvider   = "default-domain-provider"
		defaultDomainSecretData = map[string][]byte{"default": []byte("domain")}
		defaultDomain           = &garden.Domain{
			Domain:     "bar.com",
			Provider:   defaultDomainProvider,
			SecretData: defaultDomainSecretData,
		}
	)

	Describe("#ConstructExternalDomain", func() {
		var (
			namespace = "default"
			provider  = "my-dns-provider"
			domain    = "foo.bar.com"

			fakeClient client.Client
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		})

		It("returns nil because no external domain is used", func() {
			var (
				ctx   = context.TODO()
				shoot = &gardencorev1beta1.Shoot{}
			)

			externalDomain, err := ConstructExternalDomain(ctx, fakeClient, shoot, nil, nil)

			Expect(externalDomain).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the referenced secret", func() {
			var (
				ctx = context.TODO()

				dnsSecretName = "my-secret"
				dnsSecretData = map[string][]byte{"foo": []byte("bar")}

				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
					},
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: &domain,
							Providers: []gardencorev1beta1.DNSProvider{
								{
									Type:       &provider,
									SecretName: &dnsSecretName,
									Primary:    pointer.Bool(true),
								},
							},
						},
					},
				}
			)

			Expect(fakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: dnsSecretName, Namespace: namespace},
				Data:       dnsSecretData,
			})).To(Succeed())

			externalDomain, err := ConstructExternalDomain(ctx, fakeClient, shoot, nil, nil)

			Expect(externalDomain).To(Equal(&garden.Domain{
				Domain:     domain,
				Provider:   provider,
				SecretData: dnsSecretData,
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the default domain secret", func() {
			var (
				ctx = context.TODO()

				shoot = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: &domain,
							Providers: []gardencorev1beta1.DNSProvider{
								{
									Type: &provider,
								},
							},
						},
					},
				}
			)

			externalDomain, err := ConstructExternalDomain(ctx, fakeClient, shoot, nil, []*garden.Domain{defaultDomain})

			Expect(externalDomain).To(Equal(&garden.Domain{
				Domain:     domain,
				Provider:   defaultDomainProvider,
				SecretData: defaultDomainSecretData,
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the shoot secret", func() {
			var (
				ctx = context.TODO()

				shootSecretData = map[string][]byte{"foo": []byte("bar")}
				shootSecret     = &corev1.Secret{Data: shootSecretData}
				shoot           = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: &domain,
							Providers: []gardencorev1beta1.DNSProvider{
								{
									Type:    &provider,
									Primary: pointer.Bool(true),
								},
							},
						},
					},
				}
			)

			externalDomain, err := ConstructExternalDomain(ctx, fakeClient, shoot, shootSecret, nil)

			Expect(externalDomain).To(Equal(&garden.Domain{
				Domain:     domain,
				Provider:   provider,
				SecretData: shootSecretData,
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ComputeRequiredExtensions", func() {
		const (
			backupProvider       = "backupprovider"
			seedProvider         = "seedprovider"
			shootProvider        = "providertype"
			networkingType       = "networkingtype"
			extensionType1       = "extension1"
			extensionType2       = "extension2"
			extensionType3       = "extension3"
			oscType              = "osctype"
			containerRuntimeType = "containerruntimetype"
			dnsProviderType1     = "dnsprovider1"
			dnsProviderType2     = "dnsprovider2"
			dnsProviderType3     = "dnsprovider3"
		)

		var (
			shoot                      *gardencorev1beta1.Shoot
			seed                       *gardencorev1beta1.Seed
			controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList
			internalDomain             *garden.Domain
			externalDomain             *garden.Domain
		)

		BeforeEach(func() {
			controllerRegistrationList = &gardencorev1beta1.ControllerRegistrationList{
				Items: []gardencorev1beta1.ControllerRegistration{
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind: extensionsv1alpha1.ContainerRuntimeResource,
									Type: extensionType3,
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind: extensionsv1alpha1.ExtensionResource,
									Type: extensionType1,
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:            extensionsv1alpha1.ExtensionResource,
									Type:            extensionType2,
									GloballyEnabled: pointer.Bool(true),
								},
							},
						},
					},
				},
			}
			internalDomain = &garden.Domain{Provider: dnsProviderType1}
			externalDomain = &garden.Domain{Provider: dnsProviderType2}
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.SeedBackup{
						Provider: backupProvider,
					},
					Provider: gardencorev1beta1.SeedProvider{
						Type: seedProvider,
					},
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: shootProvider,
						Workers: []gardencorev1beta1.Worker{
							{
								Machine: gardencorev1beta1.Machine{
									Image: &gardencorev1beta1.ShootMachineImage{
										Name: oscType,
									},
								},
								CRI: &gardencorev1beta1.CRI{
									ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
										{Type: containerRuntimeType},
									},
								},
							},
						},
					},
					Networking: gardencorev1beta1.Networking{
						Type: networkingType,
					},
					Extensions: []gardencorev1beta1.Extension{
						{Type: extensionType1},
					},
					DNS: &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{
							{Type: pointer.String(dnsProviderType3)},
						},
					},
				},
			}
		})

		It("should compute the correct list of required extensions", func() {
			result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)

			Expect(result).To(Equal(sets.New[string](
				ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.BackupEntryResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, seedProvider),
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.InfrastructureResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.NetworkResource, networkingType),
				ExtensionsID(extensionsv1alpha1.WorkerResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.ExtensionResource, extensionType1),
				ExtensionsID(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
				ExtensionsID(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
				ExtensionsID(extensionsv1alpha1.ExtensionResource, extensionType2),
			)))
		})

		It("should compute the correct list of required extensions (no seed backup)", func() {
			seed.Spec.Backup = nil

			result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)

			Expect(result).To(Equal(sets.New[string](
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, seedProvider),
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.InfrastructureResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.NetworkResource, networkingType),
				ExtensionsID(extensionsv1alpha1.WorkerResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.ExtensionResource, extensionType1),
				ExtensionsID(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
				ExtensionsID(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
				ExtensionsID(extensionsv1alpha1.ExtensionResource, extensionType2),
			)))
		})

		It("should compute the correct list of required extensions (shoot explicitly disables globally enabled extension)", func() {
			shoot.Spec.Extensions = append(shoot.Spec.Extensions, gardencorev1beta1.Extension{
				Type:     extensionType2,
				Disabled: pointer.Bool(true),
			})

			result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)

			Expect(result).To(Equal(sets.New[string](
				ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.BackupEntryResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, seedProvider),
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.InfrastructureResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.NetworkResource, networkingType),
				ExtensionsID(extensionsv1alpha1.WorkerResource, shootProvider),
				ExtensionsID(extensionsv1alpha1.ExtensionResource, extensionType1),
				ExtensionsID(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
				ExtensionsID(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
			)))
		})
	})
})
