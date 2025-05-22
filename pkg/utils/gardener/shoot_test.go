// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
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
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: v1beta1constants.GardenNamespace, Name: "foo"}},
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
					Architecture: ptr.To("arm64"),
				},
				SystemComponents: &gardencorev1beta1.WorkerSystemComponents{
					Allow: true,
				},
			}
		})

		It("should maintain the common labels", func() {
			Expect(NodeLabelsForWorkerPool(workerPool, false, "osc-key")).To(And(
				HaveKeyWithValue("node.kubernetes.io/role", "node"),
				HaveKeyWithValue("kubernetes.io/arch", "arm64"),
				HaveKeyWithValue("networking.gardener.cloud/node-local-dns-enabled", "false"),
				HaveKeyWithValue("worker.gardener.cloud/system-components", "true"),
				HaveKeyWithValue("worker.gardener.cloud/pool", "worker"),
				HaveKeyWithValue("worker.garden.sapcloud.io/group", "worker"),
				HaveKeyWithValue("worker.gardener.cloud/gardener-node-agent-secret-name", "osc-key"),
			))
		})

		It("should add user-specified labels", func() {
			workerPool.Labels = map[string]string{
				"test": "foo",
				"bar":  "baz",
			}
			Expect(NodeLabelsForWorkerPool(workerPool, false, "osc-key")).To(And(
				HaveKeyWithValue("test", "foo"),
				HaveKeyWithValue("bar", "baz"),
			))
		})

		It("should not add system components label if they are not allowed", func() {
			workerPool.SystemComponents.Allow = false
			Expect(NodeLabelsForWorkerPool(workerPool, false, "osc-key")).NotTo(
				HaveKey("worker.gardener.cloud/system-components"),
			)
		})

		It("should correctly handle the node-local-dns label", func() {
			Expect(NodeLabelsForWorkerPool(workerPool, false, "osc-key")).To(
				HaveKeyWithValue("networking.gardener.cloud/node-local-dns-enabled", "false"),
			)
			Expect(NodeLabelsForWorkerPool(workerPool, true, "osc-key")).To(
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
			Expect(NodeLabelsForWorkerPool(workerPool, false, "osc-key")).To(And(
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

	Describe("#GetShootProjectInternalSecretSuffixes", func() {
		It("should return the expected list", func() {
			Expect(GetShootProjectInternalSecretSuffixes()).To(ConsistOf("ca-client"))
		})
	})

	Describe("#ComputeShootProjectResourceName", func() {
		It("should compute the expected name", func() {
			Expect(ComputeShootProjectResourceName("foo", "bar")).To(Equal("foo.bar"))
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
		Entry("ssh-keypair.old suffix", "bar.ssh-keypair.old", "bar", true),
		Entry("monitoring suffix", "baz.monitoring", "baz", true),
	)

	DescribeTable("#IsShootProjectInternalSecret",
		func(name, expectedShootName string, expectedOK bool) {
			shootName, ok := IsShootProjectInternalSecret(name)
			Expect(shootName).To(Equal(expectedShootName))
			Expect(ok).To(Equal(expectedOK))
		},
		Entry("no suffix", "foo", "", false),
		Entry("unrelated suffix", "foo.bar", "", false),
		Entry("wrong suffix delimiter", "foo:kubeconfig", "", false),
		Entry("ca-client suffix", "baz.ca-client", "baz", true),
	)

	DescribeTable("#ComputeManagedShootIssuerSecretName",
		func(projectName string, shootUID types.UID, expectedName string) {
			Expect(ComputeManagedShootIssuerSecretName(projectName, shootUID)).To(Equal(expectedName))
		},
		Entry("test one", "foo", types.UID("123"), "foo--123"),
		Entry("test two", "bar", types.UID("4-5"), "bar--4-5"),
	)

	DescribeTable("#IsShootProjectConfigMap",
		func(name, expectedShootName string, expectedOK bool) {
			shootName, ok := IsShootProjectConfigMap(name)
			Expect(shootName).To(Equal(expectedShootName))
			Expect(ok).To(Equal(expectedOK))
		},
		Entry("no suffix", "foo", "", false),
		Entry("unrelated suffix", "foo.bar", "", false),
		Entry("wrong suffix delimiter", "foo:kubeconfig", "", false),
		Entry("ca-cluster suffix", "baz.ca-cluster", "baz", true),
		Entry("ca-kubelet suffix", "baz.ca-kubelet", "baz", true),
	)

	Describe("#NewShootAccessSecret", func() {
		var (
			name      = "name"
			namespace = "namespace"
		)

		DescribeTable("default name/namespace",
			func(prefix string) {
				Expect(NewShootAccessSecret(prefix+name, namespace)).To(Equal(&AccessSecret{
					Secret:             &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-" + name, Namespace: namespace}},
					ServiceAccountName: name,
					Class:              "shoot",
				}))
			},

			Entry("no prefix", ""),
			Entry("prefix", "shoot-access-"),
		)

		It("should override the name and namespace", func() {
			Expect(NewShootAccessSecret(name, namespace).
				WithNameOverride("other-name").
				WithNamespaceOverride("other-namespace"),
			).To(Equal(&AccessSecret{
				Secret:             &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other-name", Namespace: "other-namespace"}},
				ServiceAccountName: name,
				Class:              "shoot",
			}))
		})
	})

	Describe("AccessSecret", func() {
		var (
			ctx                     = context.TODO()
			fakeClient              client.Client
			accessSecret            *AccessSecret
			serviceAccountName      = "serviceaccount"
			tokenExpirationDuration = "1234h"
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			accessSecret = &AccessSecret{
				Secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "access-secret",
						Namespace: "namespace",
					},
				},
				ServiceAccountName: serviceAccountName,
				Class:              "foo",
			}
		})

		Describe("#Reconcile", func() {
			validate := func() {
				Expect(accessSecret.Reconcile(ctx, fakeClient)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(accessSecret.Secret), accessSecret.Secret)).To(Succeed())
				Expect(accessSecret.Secret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(accessSecret.Secret.Labels).To(And(
					HaveKeyWithValue("resources.gardener.cloud/purpose", "token-requestor"),
					HaveKeyWithValue("resources.gardener.cloud/class", accessSecret.Class),
				))
				Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/name", serviceAccountName))
			}

			Context("create", func() {
				BeforeEach(func() {
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(accessSecret.Secret), accessSecret.Secret)).To(BeNotFoundError())
				})

				It("should work w/o settings", func() {
					validate()
					Expect(accessSecret.Secret.Annotations).NotTo(HaveKey("serviceaccount.resources.gardener.cloud/namespace"))
				})

				It("should set ServiceAccount namespace to kube-system for shoot class", func() {
					accessSecret.Class = "shoot"
					validate()
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/namespace", "kube-system"))
				})

				It("should work w/ token expiration duration", func() {
					accessSecret.WithTokenExpirationDuration(tokenExpirationDuration)
					validate()
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-expiration-duration", tokenExpirationDuration))
				})

				It("should work w/ kubeconfig", func() {
					kubeconfig := &clientcmdv1.Config{}
					kubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
					Expect(err).NotTo(HaveOccurred())

					accessSecret.WithKubeconfig(kubeconfig)
					validate()
					Expect(accessSecret.Secret.Data).To(HaveKeyWithValue("kubeconfig", kubeconfigRaw))
				})

				It("should work w/ target secret", func() {
					targetSecretName, targetSecretNamespace := "tname", "tnamespace"

					accessSecret.WithTargetSecret(targetSecretName, targetSecretNamespace)
					validate()
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-name", targetSecretName))
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-namespace", targetSecretNamespace))
				})

				It("should work w/ service account labels", func() {
					accessSecret.WithServiceAccountLabels(map[string]string{"foo": "bar"})
					validate()
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/labels", `{"foo":"bar"}`))
				})
			})

			Context("update", func() {
				BeforeEach(func() {
					accessSecret.Secret.Type = corev1.SecretTypeServiceAccountToken
					accessSecret.Secret.Annotations = map[string]string{"foo": "bar"}
					accessSecret.Secret.Labels = map[string]string{"bar": "foo"}
					Expect(fakeClient.Create(ctx, accessSecret.Secret)).To(Succeed())
				})

				AfterEach(func() {
					Expect(accessSecret.Secret.Labels).To(HaveKeyWithValue("bar", "foo"))
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("foo", "bar"))
				})

				It("should work w/o settings", func() {
					validate()
				})

				It("should work w/ token expiration duration", func() {
					accessSecret.WithTokenExpirationDuration(tokenExpirationDuration)
					validate()
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("serviceaccount.resources.gardener.cloud/token-expiration-duration", tokenExpirationDuration))
				})

				It("should work w/ kubeconfig", func() {
					existingAuthInfos := []clientcmdv1.NamedAuthInfo{{AuthInfo: clientcmdv1.AuthInfo{Token: "some-token"}}}

					existingKubeconfig := &clientcmdv1.Config{AuthInfos: existingAuthInfos}
					existingKubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, existingKubeconfig)
					Expect(err).NotTo(HaveOccurred())

					accessSecret.Secret.Data = map[string][]byte{"kubeconfig": existingKubeconfigRaw}
					Expect(fakeClient.Update(ctx, accessSecret.Secret)).To(Succeed())

					newKubeconfig := existingKubeconfig.DeepCopy()
					newKubeconfig.AuthInfos = nil
					accessSecret.WithKubeconfig(newKubeconfig)

					expectedKubeconfig := newKubeconfig.DeepCopy()
					expectedKubeconfig.AuthInfos = existingAuthInfos
					expectedKubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, expectedKubeconfig)
					Expect(err).NotTo(HaveOccurred())

					validate()
					Expect(accessSecret.Secret.Data).To(HaveKeyWithValue("kubeconfig", expectedKubeconfigRaw))
				})

				It("should delete the kubeconfig key", func() {
					accessSecret.Secret.Data = map[string][]byte{"kubeconfig": []byte("foo")}
					Expect(fakeClient.Update(ctx, accessSecret.Secret)).To(Succeed())

					validate()
					Expect(accessSecret.Secret.Data).NotTo(HaveKey("kubeconfig"))
				})

				It("should delete the token key", func() {
					accessSecret.Secret.Data = map[string][]byte{"token": []byte("foo")}
					Expect(fakeClient.Update(ctx, accessSecret.Secret)).To(Succeed())

					accessSecret.WithKubeconfig(&clientcmdv1.Config{})

					validate()
					Expect(accessSecret.Secret.Data).NotTo(HaveKey("token"))
				})

				It("should work w/ target secret", func() {
					targetSecretName, targetSecretNamespace := "tname", "tnamespace"

					accessSecret.WithTargetSecret(targetSecretName, targetSecretNamespace)
					validate()
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-name", targetSecretName))
					Expect(accessSecret.Secret.Annotations).To(HaveKeyWithValue("token-requestor.resources.gardener.cloud/target-secret-namespace", targetSecretNamespace))
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

			deployment *appsv1.Deployment
			podSpec    *corev1.PodSpec
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: containerName1},
								{Name: containerName2},
							},
						},
					},
				},
			}

			podSpec = &deployment.Spec.Template.Spec
		})

		It("should do nothing because object is not handled", func() {
			Expect(InjectGenericKubeconfig(&corev1.Service{}, genericTokenKubeconfigSecretName, tokenSecretName)).To(MatchError(ContainSubstring("unhandled object type")))
		})

		It("should inject the generic kubeconfig into the specified container", func() {
			Expect(InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecretName, tokenSecretName, containerName1)).To(Succeed())

			Expect(podSpec.Volumes).To(ContainElement(corev1.Volume{
				Name: "kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: ptr.To[int32](420),
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
									Optional: ptr.To(false),
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
									Optional: ptr.To(false),
								},
							},
						},
					},
				},
			}))

			Expect(podSpec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "kubeconfig",
				MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
				ReadOnly:  true,
			}))

			Expect(podSpec.Containers[1].VolumeMounts).To(BeEmpty())
		})
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
					SeedName: ptr.To("spec"),
				},
				Status: gardencorev1beta1.ShootStatus{
					SeedName: ptr.To("status"),
				},
			})
			Expect(specSeedName).To(Equal(ptr.To("spec")))
			Expect(statusSeedName).To(Equal(ptr.To("status")))
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

		It("should return the control plane toleration when the pool is a control plane pool", func() {
			Expect(ExtractSystemComponentsTolerations([]gardencorev1beta1.Worker{
				{
					ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
				},
			})).To(HaveExactElements(
				corev1.Toleration{
					Key:      "node-role.kubernetes.io/control-plane",
					Operator: corev1.TolerationOpExists,
				},
			))
		})

		It("should return tolerations in order", func() {
			Expect(ExtractSystemComponentsTolerations([]gardencorev1beta1.Worker{
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
					Taints: []corev1.Taint{
						{
							Key:    "b",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
					Taints: []corev1.Taint{
						{
							Key:    "a",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
				{
					SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true},
					Taints: []corev1.Taint{
						{
							Key:    "c",
							Value:  "someValue",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			})).To(HaveExactElements(
				corev1.Toleration{
					Key:      "a",
					Operator: corev1.TolerationOpEqual,
					Value:    "someValue",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				corev1.Toleration{
					Key:      "b",
					Operator: corev1.TolerationOpEqual,
					Value:    "someValue",
					Effect:   corev1.TaintEffectNoExecute,
				},
				corev1.Toleration{
					Key:      "c",
					Operator: corev1.TolerationOpEqual,
					Value:    "someValue",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			))
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
			Expect(ConstructInternalClusterDomain(shootName, shootProject, &Domain{Domain: internalDomain})).To(Equal(expected))
		},

		Entry("with internal domain key", "foo", "bar", "internal.example.com", "foo.bar.internal.example.com"),
		Entry("without internal domain key", "foo", "bar", "example.com", "foo.bar.internal.example.com"),
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
		defaultDomain           = &Domain{
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
									Primary:    ptr.To(true),
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

			Expect(externalDomain).To(Equal(&Domain{
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

			externalDomain, err := ConstructExternalDomain(ctx, fakeClient, shoot, nil, []*Domain{defaultDomain})

			Expect(externalDomain).To(Equal(&Domain{
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
									Primary: ptr.To(true),
								},
							},
						},
					},
				}
			)

			externalDomain, err := ConstructExternalDomain(ctx, fakeClient, shoot, shootSecret, nil)

			Expect(externalDomain).To(Equal(&Domain{
				Domain:     domain,
				Provider:   provider,
				SecretData: shootSecretData,
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns error because WorkloadIdentity credential is not supported", func() {
			var (
				ctx = context.TODO()

				workloadIdentity = &securityv1alpha1.WorkloadIdentity{}
				shoot            = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: &domain,
							Providers: []gardencorev1beta1.DNSProvider{
								{
									Type:    &provider,
									Primary: ptr.To(true),
								},
							},
						},
					},
				}
			)

			_, err := ConstructExternalDomain(ctx, fakeClient, shoot, workloadIdentity, nil)
			Expect(err).To(MatchError(Equal("shoot credentials of type WorkloadIdentity cannot be used as domain secret")))
		})

		It("returns error because shoot credential type is not supported", func() {
			var (
				ctx = context.TODO()

				pod   = &corev1.Pod{}
				shoot = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: &domain,
							Providers: []gardencorev1beta1.DNSProvider{
								{
									Type:    &provider,
									Primary: ptr.To(true),
								},
							},
						},
					},
				}
			)

			_, err := ConstructExternalDomain(ctx, fakeClient, shoot, pod, nil)
			Expect(err).To(MatchError(Equal("unexpected shoot credentials type")))
		})
	})

	Describe("#ComputeRequiredExtensionsForShoot", func() {
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
			internalDomain             *Domain
			externalDomain             *Domain
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
									GloballyEnabled: ptr.To(true),
								},
							},
						},
					},
				},
			}
			internalDomain = &Domain{Provider: dnsProviderType1}
			externalDomain = &Domain{Provider: dnsProviderType2}
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.Backup{
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
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To(networkingType),
					},
					Extensions: []gardencorev1beta1.Extension{
						{Type: extensionType1},
					},
					DNS: &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{
							{Type: ptr.To(dnsProviderType3)},
						},
					},
				},
			}
		})

		It("should compute the correct list of required extensions", func() {
			Expect(ComputeRequiredExtensionsForShoot(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)).To(Equal(sets.New(
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

			Expect(ComputeRequiredExtensionsForShoot(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)).To(Equal(sets.New(
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
				Disabled: ptr.To(true),
			})

			Expect(ComputeRequiredExtensionsForShoot(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)).To(Equal(sets.New(
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

		It("should compute the correct list of required extensions (workerless Shoot)", func() {
			shoot.Spec.Provider.Workers = nil

			Expect(ComputeRequiredExtensionsForShoot(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)).To(Equal(sets.New(
				ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.BackupEntryResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, seedProvider),
				ExtensionsID(extensionsv1alpha1.ExtensionResource, extensionType1),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
			)))
		})

		It("should compute the correct list of required extensions (no seed)", func() {
			Expect(ComputeRequiredExtensionsForShoot(shoot, nil, controllerRegistrationList, internalDomain, externalDomain)).To(Equal(sets.New(
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

		It("should compute the correct list of required extensions (workerless Shoot and globally enabled extension)", func() {
			shoot.Spec.Extensions = []gardencorev1beta1.Extension{}
			shoot.Spec.Provider.Workers = nil
			controllerRegistrationList = &gardencorev1beta1.ControllerRegistrationList{
				Items: []gardencorev1beta1.ControllerRegistration{
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:                extensionsv1alpha1.ExtensionResource,
									Type:                extensionType1,
									GloballyEnabled:     ptr.To(true),
									WorkerlessSupported: ptr.To(false),
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:                extensionsv1alpha1.ExtensionResource,
									Type:                extensionType2,
									GloballyEnabled:     ptr.To(true),
									WorkerlessSupported: ptr.To(true),
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:            extensionsv1alpha1.ExtensionResource,
									Type:            extensionType3,
									GloballyEnabled: ptr.To(true),
								},
							},
						},
					},
				},
			}

			Expect(ComputeRequiredExtensionsForShoot(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)).To(Equal(sets.New(
				ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.BackupEntryResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.ControlPlaneResource, seedProvider),
				ExtensionsID(extensionsv1alpha1.ExtensionResource, extensionType2),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
				ExtensionsID(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
			)))
		})

		It("should compute the correct list of required extensions (autonomous shoot with backup)", func() {
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{
				ControlPlane: &gardencorev1beta1.WorkerControlPlane{Backup: &gardencorev1beta1.Backup{Provider: backupProvider}},
			})

			Expect(ComputeRequiredExtensionsForShoot(shoot, nil, controllerRegistrationList, internalDomain, externalDomain)).To(Equal(sets.New(
				ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupProvider),
				ExtensionsID(extensionsv1alpha1.BackupEntryResource, backupProvider),
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
	})

	Describe("#ExtensionsID", func() {
		It("should return the expected identifier", func() {
			Expect(ExtensionsID("foo", "bar")).To(Equal("foo/bar"))
		})
	})

	Describe("#ComputeTechnicalID", func() {
		var (
			projectName string
			shoot       *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			projectName = "project-a"
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}
		})

		It("should return the ID from status", func() {
			shoot.Status = gardencorev1beta1.ShootStatus{
				TechnicalID: "some-id",
			}

			Expect(ComputeTechnicalID(projectName, shoot)).To(Equal("some-id"))
		})

		It("should calculate a new ID", func() {
			Expect(ComputeTechnicalID(projectName, shoot)).To(Equal(fmt.Sprintf("shoot--%s--%s", projectName, shoot.Name)))
		})
	})

	Describe("#GetShootConditionTypes", func() {
		It("should return all shoot condition types", func() {
			Expect(GetShootConditionTypes(false)).To(HaveExactElements(
				gardencorev1beta1.ConditionType("APIServerAvailable"),
				gardencorev1beta1.ConditionType("ControlPlaneHealthy"),
				gardencorev1beta1.ConditionType("ObservabilityComponentsHealthy"),
				gardencorev1beta1.ConditionType("EveryNodeReady"),
				gardencorev1beta1.ConditionType("SystemComponentsHealthy"),
			))
		})

		It("should return all shoot condition types for workerless shoot", func() {
			Expect(GetShootConditionTypes(true)).To(HaveExactElements(
				gardencorev1beta1.ConditionType("APIServerAvailable"),
				gardencorev1beta1.ConditionType("ControlPlaneHealthy"),
				gardencorev1beta1.ConditionType("ObservabilityComponentsHealthy"),
				gardencorev1beta1.ConditionType("SystemComponentsHealthy"),
			))
		})
	})

	Describe("#DefaultGVKsForEncryption", func() {
		It("should return all default GroupVersionKinds", func() {
			Expect(DefaultGVKsForEncryption()).To(ConsistOf(
				schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
			))
		})
	})

	Describe("#DefaultResourcesForEncryption", func() {
		It("should return all default resources", func() {
			Expect(DefaultResourcesForEncryption().UnsortedList()).To(ConsistOf(
				"secrets",
			))
		})
	})

	DescribeTable("#GetIPStackForShoot",
		func(shoot *gardencorev1beta1.Shoot, expectedResult string) {
			Expect(GetIPStackForShoot(shoot)).To(Equal(expectedResult))
		},

		Entry("default shoot", &gardencorev1beta1.Shoot{}, "ipv4"),
		Entry("ipv4 shoot", &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}}}}, "ipv4"),
		Entry("ipv6 shoot", &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}}}}, "ipv6"),
		Entry("dual-stack shoot (ipv4 preferred)", &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6}}}}, "dual-stack"),
		Entry("dual-stack shoot (ipv6 preferred)", &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6, gardencorev1beta1.IPFamilyIPv4}}}}, "dual-stack"),
	)

	Describe("CalculateDataStringForKubeletConfiguration", func() {
		var kubeletConfig *gardencorev1beta1.KubeletConfig

		BeforeEach(func() {
			kubeletConfig = &gardencorev1beta1.KubeletConfig{}
		})

		It("should return nil if the kubelet config is nil", func() {
			Expect(CalculateDataStringForKubeletConfiguration(nil)).To(BeNil())
		})

		It("should return an empty string if the kubelet config is empty", func() {
			Expect(CalculateDataStringForKubeletConfiguration(kubeletConfig)).To(BeEmpty())
		})

		It("should return the correct data string for the kubelet config", func() {
			kubeletConfig = &gardencorev1beta1.KubeletConfig{
				CPUManagerPolicy: ptr.To("static"),
				EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
					ImageFSAvailable:  ptr.To("200Mi"),
					ImageFSInodesFree: ptr.To("1k"),
					MemoryAvailable:   ptr.To("200Mi"),
					NodeFSAvailable:   ptr.To("200Mi"),
					NodeFSInodesFree:  ptr.To("1k"),
				},
				SystemReserved: &gardencorev1beta1.KubeletConfigReserved{
					CPU:              ptr.To(resource.MustParse("1m")),
					Memory:           ptr.To(resource.MustParse("1Mi")),
					PID:              ptr.To(resource.MustParse("1k")),
					EphemeralStorage: ptr.To(resource.MustParse("100Gi")),
				},
				KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
					CPU:              ptr.To(resource.MustParse("100m")),
					Memory:           ptr.To(resource.MustParse("2Gi")),
					PID:              ptr.To(resource.MustParse("15k")),
					EphemeralStorage: ptr.To(resource.MustParse("42Gi")),
				},
			}

			Expect(CalculateDataStringForKubeletConfiguration(kubeletConfig)).To(ConsistOf(
				"101m-2049Mi-16k-142Gi",
				"200Mi-1k-200Mi-200Mi-1k",
				"static",
			))
		})
	})

	Describe("IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled", func() {
		var (
			shoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
						KubeScheduler: &gardencorev1beta1.KubeSchedulerConfig{},
					},
				},
			}
		})

		It("should return false if the shoot is nil", func() {
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(nil)).To(BeFalse())
		})

		It("should return false if the shoot.spec.kubernetes.kubeAPIServer is nil", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = nil
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot)).To(BeFalse())
		})

		It("should return false if the shoot.spec.kubernetes.kubeScheduler is nil", func() {
			shoot.Spec.Kubernetes.KubeScheduler = nil
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot)).To(BeFalse())
		})

		It("should return false if the feature gate is not present in the kube-apiserver", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates = nil
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot)).To(BeFalse())
		})

		It("should return false if the feature gate is not present in the kube-scheduler", func() {
			shoot.Spec.Kubernetes.KubeScheduler.FeatureGates = nil
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot)).To(BeFalse())
		})

		It("should return false if the feature gate is disabled only in the kube-apiserver", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates = map[string]bool{
				"MatchLabelKeysInPodTopologySpread": false,
			}
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot)).To(BeFalse())
		})

		It("should return false if the feature gate is disabled only in the kube-scheduler", func() {
			shoot.Spec.Kubernetes.KubeScheduler.FeatureGates = map[string]bool{
				"MatchLabelKeysInPodTopologySpread": false,
			}
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot)).To(BeFalse())
		})

		It("should return true if the feature gate is disabled in both kube-apiserver and kube-scheduler", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates = map[string]bool{
				"MatchLabelKeysInPodTopologySpread": false,
			}
			shoot.Spec.Kubernetes.KubeScheduler.FeatureGates = map[string]bool{
				"MatchLabelKeysInPodTopologySpread": false,
			}
			Expect(IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot)).To(BeTrue())
		})
	})

	DescribeTable("#IsAuthorizeWithSelectorsEnabled",
		func(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, kubernetesVersion *semver.Version, match gomegatypes.GomegaMatcher) {
			Expect(IsAuthorizeWithSelectorsEnabled(kubeAPIServerConfig, kubernetesVersion)).To(match)
		},

		Entry("version is >= 1.32 and kubeAPIServerConfig is nil",
			&gardencorev1beta1.KubeAPIServerConfig{},
			semver.MustParse("1.32.0"),
			BeTrue()),
		Entry("version is >= 1.32 and AuthorizeWithSelectors feature is true",
			&gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{
						"AuthorizeWithSelectors": true,
					},
				},
			},
			semver.MustParse("1.32.0"),
			BeTrue()),
		Entry("version is >= 1.32 and AuthorizeWithSelectors feature is false",
			&gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{
						"AuthorizeWithSelectors": false,
					},
				},
			},
			semver.MustParse("1.32.0"),
			BeFalse()),
		Entry("version is 1.31 and kubeAPIServerConfig is nil",
			&gardencorev1beta1.KubeAPIServerConfig{},
			semver.MustParse("1.31.0"),
			BeFalse()),
		Entry("version is 1.31 and AuthorizeWithSelectors feature is true",
			&gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{
						"AuthorizeWithSelectors": true,
					},
				},
			},
			semver.MustParse("1.31.0"),
			BeTrue()),
		Entry("version is 1.31 and AuthorizeWithSelectors feature is false",
			&gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{
						"AuthorizeWithSelectors": false,
					},
				},
			},
			semver.MustParse("1.31.0"),
			BeFalse()),
		Entry("version is < 1.31",
			&gardencorev1beta1.KubeAPIServerConfig{},
			semver.MustParse("1.30.0"),
			BeFalse()),
	)

	Describe("#CalculateWorkerPoolHashForInPlaceUpdate", func() {
		var (
			kubernetesVersion *string
			kubeletConfig     *gardencorev1beta1.KubeletConfig
			credentials       *gardencorev1beta1.ShootCredentials

			machineImageVersion string
			workerPoolName      string

			hash                        string
			lastCARotationInitiation    = metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
			lastSAKeyRotationInitiation = metav1.Time{Time: time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC)}
		)

		BeforeEach(func() {
			workerPoolName = "worker"
			kubernetesVersion = ptr.To("1.2.3")
			machineImageVersion = "1.1.1"

			kubeletConfig = &gardencorev1beta1.KubeletConfig{
				KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
					CPU:              ptr.To(resource.MustParse("80m")),
					Memory:           ptr.To(resource.MustParse("1Gi")),
					PID:              ptr.To(resource.MustParse("10k")),
					EphemeralStorage: ptr.To(resource.MustParse("20Gi")),
				},
				EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
					MemoryAvailable: ptr.To("100Mi"),
				},
				CPUManagerPolicy: nil,
			}

			credentials = &gardencorev1beta1.ShootCredentials{
				Rotation: &gardencorev1beta1.ShootCredentialsRotation{
					CertificateAuthorities: &gardencorev1beta1.CARotation{
						LastInitiationTime: &lastCARotationInitiation,
					},
					ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
						LastInitiationTime: &lastSAKeyRotationInitiation,
					},
				},
			}

			var err error
			hash, err = CalculateWorkerPoolHashForInPlaceUpdate(workerPoolName, kubernetesVersion, kubeletConfig, machineImageVersion, credentials)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("hash value should change", func() {
			AfterEach(func() {
				actual, err := CalculateWorkerPoolHashForInPlaceUpdate(workerPoolName, kubernetesVersion, kubeletConfig, machineImageVersion, credentials)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).NotTo(Equal(hash))
			})

			It("when changing machine image version", func() {
				machineImageVersion = "new-version"
			})

			It("when changing the kubernetes major/minor version of the worker pool version", func() {
				kubernetesVersion = ptr.To("1.3.2")
			})

			It("when a shoot CA rotation is triggered", func() {
				newRotationTime := metav1.Time{Time: lastCARotationInitiation.Add(time.Hour)}
				credentials.Rotation.CertificateAuthorities.LastInitiationTime = &newRotationTime
			})

			It("when a shoot CA rotation is triggered for the first time (lastInitiationTime was nil)", func() {
				var err error
				credentialStatusWithInitiatedRotation := credentials.Rotation.CertificateAuthorities.DeepCopy()
				credentials.Rotation.CertificateAuthorities = nil
				hash, err = CalculateWorkerPoolHashForInPlaceUpdate(workerPoolName, kubernetesVersion, kubeletConfig, machineImageVersion, credentials)
				Expect(err).ToNot(HaveOccurred())

				credentials.Rotation.CertificateAuthorities = credentialStatusWithInitiatedRotation
			})

			It("when a shoot service account key rotation is triggered", func() {
				newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
				credentials.Rotation.ServiceAccountKey.LastInitiationTime = &newRotationTime
			})

			It("when a shoot service account key rotation is triggered for the first time (lastInitiationTime was nil)", func() {
				var err error
				credentialStatusWithInitiatedRotation := credentials.Rotation.ServiceAccountKey.DeepCopy()
				credentials.Rotation.ServiceAccountKey = nil
				hash, err = CalculateWorkerPoolHashForInPlaceUpdate(workerPoolName, kubernetesVersion, kubeletConfig, machineImageVersion, credentials)
				Expect(err).ToNot(HaveOccurred())

				credentials.Rotation.ServiceAccountKey = credentialStatusWithInitiatedRotation
			})

			It("when changing kubeReserved CPU", func() {
				kubeletConfig.KubeReserved.CPU = ptr.To(resource.MustParse("100m"))
			})

			It("when changing kubeReserved memory", func() {
				kubeletConfig.KubeReserved.Memory = ptr.To(resource.MustParse("2Gi"))
			})

			It("when changing kubeReserved PID", func() {
				kubeletConfig.KubeReserved.PID = ptr.To(resource.MustParse("15k"))
			})

			It("when changing kubeReserved ephemeral storage", func() {
				kubeletConfig.KubeReserved.EphemeralStorage = ptr.To(resource.MustParse("42Gi"))
			})

			It("when changing evictionHard memory threshold", func() {
				kubeletConfig.EvictionHard.MemoryAvailable = ptr.To("200Mi")
			})

			It("when changing evictionHard image fs threshold", func() {
				kubeletConfig.EvictionHard.ImageFSAvailable = ptr.To("200Mi")
			})

			It("when changing evictionHard image fs inodes threshold", func() {
				kubeletConfig.EvictionHard.ImageFSInodesFree = ptr.To("1k")
			})

			It("when changing evictionHard node fs threshold", func() {
				kubeletConfig.EvictionHard.NodeFSAvailable = ptr.To("200Mi")
			})

			It("when changing evictionHard node fs inodes threshold", func() {
				kubeletConfig.EvictionHard.NodeFSInodesFree = ptr.To("1k")
			})

			It("when changing CPUManagerPolicy", func() {
				kubeletConfig.CPUManagerPolicy = ptr.To("test")
			})

			It("when changing systemReserved CPU", func() {
				kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
					CPU: ptr.To(resource.MustParse("1m")),
				}
			})

			It("when changing systemReserved memory", func() {
				kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
					Memory: ptr.To(resource.MustParse("1Mi")),
				}
			})

			It("when systemReserved PID", func() {
				kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
					PID: ptr.To(resource.MustParse("1k")),
				}
			})

			It("when changing systemReserved EphemeralStorage", func() {
				kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
					EphemeralStorage: ptr.To(resource.MustParse("100Gi")),
				}
			})
		})

		Context("hash value should not change", func() {
			AfterEach(func() {
				actual, err := CalculateWorkerPoolHashForInPlaceUpdate(workerPoolName, kubernetesVersion, kubeletConfig, machineImageVersion, credentials)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(hash))
			})

			It("when a shoot CA rotation is triggered but the pool name is present in pendingWorkersRollouts", func() {
				credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts = []gardencorev1beta1.PendingWorkersRollout{
					{
						Name:               workerPoolName,
						LastInitiationTime: &lastCARotationInitiation,
					},
				}

				newRotationTime := metav1.Time{Time: lastCARotationInitiation.Add(time.Hour)}
				credentials.Rotation.CertificateAuthorities.LastInitiationTime = &newRotationTime
			})

			It("when a shoot ServiceAccountKey rotation is triggered but the pool name is present in pendingWorkersRollouts", func() {
				credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts = []gardencorev1beta1.PendingWorkersRollout{
					{
						Name:               workerPoolName,
						LastInitiationTime: &lastSAKeyRotationInitiation,
					},
				}

				newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
				credentials.Rotation.ServiceAccountKey.LastInitiationTime = &newRotationTime
			})
		})

		It("should return an error if kubernetes version is invalid", func() {
			kubernetesVersion = ptr.To("invalid")

			_, err := CalculateWorkerPoolHashForInPlaceUpdate(workerPoolName, kubernetesVersion, kubeletConfig, machineImageVersion, credentials)
			Expect(err).To(MatchError(ContainSubstring("failed to parse")))
		})
	})
})
