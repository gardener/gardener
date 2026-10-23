// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/etcd/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("StaticPods", func() {
	var (
		seedClient client.Client
		botanist   *Botanist
	)

	BeforeEach(func() {
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{
			Operation: &operation.Operation{
				SeedClientSet: fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build(),
				Shoot:         &shootpkg.Shoot{ControlPlaneNamespace: "kube-system"},
			},
		}
	})

	Describe("#useShootAccessTokensForSelfHostedShootControlPlane", func() {
		selfHostedShoot := func(gardenerName string) *gardencorev1beta1.Shoot {
			return &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
						}},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Gardener: gardencorev1beta1.Gardener{Name: gardenerName},
				},
			}
		}

		It("should return false for a non-self-hosted shoot", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when the shoot status indicates gardenadm", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenadm"))

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when gardener-resource-manager deployment does not exist", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when gardener-resource-manager runs with host network (bootstrap phase)", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: "kube-system"},
				Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: true}}},
			}
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return true when gardener-resource-manager runs in the pod network", func(ctx SpecContext) {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: "kube-system"},
				Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: false}}},
			}
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeTrue())
		})
	})

	Describe("admin kubeconfig", func() {
		BeforeEach(func(ctx SpecContext) {
			botanist.SecretsManager = fakesecretsmanager.New(seedClient, "kube-system")
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{TechnicalID: "shoot--foo--bar"}})

			Expect(seedClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameCACluster, Namespace: "kube-system"},
				Data:       map[string][]byte{"bundle.crt": []byte("ca-bundle")},
			})).To(Succeed())
		})

		loadAdminKubeconfig := func(files []extensionsv1alpha1.File) *clientcmdapi.Config {
			GinkgoHelper()

			Expect(files).To(HaveLen(1))
			Expect(files[0].Path).To(Equal(PathKubeconfig))

			rawKubeconfig, err := utils.DecodeBase64(files[0].Content.Inline.Data)
			Expect(err).NotTo(HaveOccurred())
			kubeconfig, err := clientcmd.Load(rawKubeconfig)
			Expect(err).NotTo(HaveOccurred())

			Expect(kubeconfig.Clusters).To(HaveKeyWithValue("shoot--foo--bar", PointTo(MatchFields(IgnoreExtras, Fields{
				"Server":                   Equal("https://localhost"),
				"CertificateAuthorityData": Equal([]byte("ca-bundle")),
			}))))
			Expect(kubeconfig.Contexts).To(HaveKeyWithValue("shoot--foo--bar", PointTo(MatchFields(IgnoreExtras, Fields{
				"Namespace": Equal("kube-system"),
			}))))
			return kubeconfig
		}

		Describe("#appendStaticAdminKubeconfigToFiles", func() {
			It("should target localhost and use the static admin token", func(ctx SpecContext) {
				Expect(seedClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-static-token", Namespace: "kube-system"},
					Data:       map[string][]byte{"static_tokens.csv": []byte("admin-token,system:cluster-admin,system:cluster-admin,system:masters")},
				})).To(Succeed())

				files, err := AppendStaticAdminKubeconfigToFiles(botanist, nil)
				Expect(err).NotTo(HaveOccurred())

				kubeconfig := loadAdminKubeconfig(files)
				Expect(kubeconfig.AuthInfos).To(HaveKeyWithValue("shoot--foo--bar", PointTo(MatchFields(IgnoreExtras, Fields{
					"Token":     Equal("admin-token"),
					"TokenFile": BeEmpty(),
				}))))
			})

			It("should fail if the static token secret does not exist", func() {
				_, err := AppendStaticAdminKubeconfigToFiles(botanist, nil)
				Expect(err).To(MatchError(ContainSubstring(`secret "kube-apiserver-static-token" not found`)))
			})
		})

		Describe("#appendDynamicAdminKubeconfigToFiles", func() {
			It("should target localhost and use the admin token file", func() {
				files, err := AppendDynamicAdminKubeconfigToFiles(botanist, nil)
				Expect(err).NotTo(HaveOccurred())

				kubeconfig := loadAdminKubeconfig(files)
				Expect(kubeconfig.AuthInfos).To(HaveKeyWithValue("shoot--foo--bar", PointTo(MatchFields(IgnoreExtras, Fields{
					"Token":     BeEmpty(),
					"TokenFile": Equal("/etc/kubernetes/admin-token"),
				}))))
			})
		})
	})

	Describe("#MergeFilesIntoOSC", func() {
		var (
			fileWithContent func(path, content string) extensionsv1alpha1.File

			staticPodFile func(name, hash string) extensionsv1alpha1.File
			podFromFile   func(file extensionsv1alpha1.File) *corev1.Pod
		)

		BeforeEach(func() {
			fileWithContent = func(path, content string) extensionsv1alpha1.File {
				return extensionsv1alpha1.File{
					Path:    path,
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(content))}},
				}
			}

			staticPodFile = func(name, hash string) extensionsv1alpha1.File {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						Namespace:   "kube-system",
						Annotations: map[string]string{"gardener.cloud/config.mirror": hash},
					},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "foo:" + name}}},
				}
				manifest, err := kubernetesutils.Serialize(pod, kubernetes.ShootScheme)
				Expect(err).NotTo(HaveOccurred())

				return extensionsv1alpha1.File{
					Path:    "/etc/kubernetes/manifests/" + name + ".yaml",
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(manifest))}},
				}
			}

			podFromFile = func(file extensionsv1alpha1.File) *corev1.Pod {
				data, err := utils.DecodeBase64(file.Content.Inline.Data)
				Expect(err).NotTo(HaveOccurred())

				pod := &corev1.Pod{}
				_, _, err = kubernetes.ShootCodec.UniversalDeserializer().Decode(data, nil, pod)
				Expect(err).NotTo(HaveOccurred())
				return pod
			}
		})

		When("the file does not exist yet in the OperatingSystemConfig", func() {
			It("should append regular files", func() {
				osc := &extensionsv1alpha1.OperatingSystemConfig{Spec: extensionsv1alpha1.OperatingSystemConfigSpec{Files: []extensionsv1alpha1.File{
					fileWithContent("/existing/file", "keep-me"),
				}}}

				Expect(MergeFilesIntoOSC(osc, []extensionsv1alpha1.File{fileWithContent("/var/lib/etcd/ca/current/ca.crt", "new-ca")})).To(Succeed())

				Expect(osc.Spec.Files).To(ConsistOf(
					fileWithContent("/existing/file", "keep-me"),
					fileWithContent("/var/lib/etcd/ca/current/ca.crt", "new-ca"),
				))
			})

			It("should not add static pod manifests that are not already present", func() {
				osc := &extensionsv1alpha1.OperatingSystemConfig{}

				Expect(MergeFilesIntoOSC(osc, []extensionsv1alpha1.File{staticPodFile("etcd-main", "new-hash")})).To(Succeed())

				Expect(osc.Spec.Files).To(BeEmpty())
			})
		})

		When("the file already exists in the OperatingSystemConfig", func() {
			It("should overwrite the content of regular files", func() {
				osc := &extensionsv1alpha1.OperatingSystemConfig{Spec: extensionsv1alpha1.OperatingSystemConfigSpec{Files: []extensionsv1alpha1.File{
					fileWithContent("/var/lib/etcd/ca/current/ca.crt", "old-ca"),
				}}}

				Expect(MergeFilesIntoOSC(osc, []extensionsv1alpha1.File{fileWithContent("/var/lib/etcd/ca/current/ca.crt", "rotated-ca")})).To(Succeed())

				Expect(osc.Spec.Files).To(ConsistOf(fileWithContent("/var/lib/etcd/ca/current/ca.crt", "rotated-ca")))
			})

			It("should only carry over the hash annotation for static pod manifests, keeping the old pod spec", func() {
				oldFile := staticPodFile("etcd-main", "old-hash")
				osc := &extensionsv1alpha1.OperatingSystemConfig{Spec: extensionsv1alpha1.OperatingSystemConfigSpec{Files: []extensionsv1alpha1.File{oldFile}}}

				newFile := staticPodFile("etcd-main", "new-hash")
				// Simulate a differing current-generation pod spec that must NOT leak into the old OperatingSystemConfig.
				newPod := podFromFile(newFile)
				newPod.Spec.Containers[0].Image = "foo:current-generation"
				newManifest, err := kubernetesutils.Serialize(newPod, kubernetes.ShootScheme)
				Expect(err).NotTo(HaveOccurred())
				newFile.Content.Inline.Data = utils.EncodeBase64([]byte(newManifest))

				Expect(MergeFilesIntoOSC(osc, []extensionsv1alpha1.File{newFile})).To(Succeed())

				Expect(osc.Spec.Files).To(HaveLen(1))
				mergedPod := podFromFile(osc.Spec.Files[0])
				Expect(mergedPod.Annotations).To(HaveKeyWithValue("gardener.cloud/config.mirror", "new-hash"))
				Expect(mergedPod.Spec.Containers[0].Image).To(Equal("foo:etcd-main"))
			})
		})

		When("a static pod manifest cannot be decoded", func() {
			It("should return an error", func() {
				osc := &extensionsv1alpha1.OperatingSystemConfig{Spec: extensionsv1alpha1.OperatingSystemConfigSpec{Files: []extensionsv1alpha1.File{
					fileWithContent("/etc/kubernetes/manifests/etcd-main.yaml", "not-a-pod"),
				}}}

				Expect(MergeFilesIntoOSC(osc, []extensionsv1alpha1.File{staticPodFile("etcd-main", "new-hash")})).To(MatchError(ContainSubstring("failed merging hash annotation")))
			})
		})
	})

	Describe("#AppendEtcdCAsToFiles", func() {
		var (
			ctrl       *gomock.Controller
			fakeClient client.Client
			sm         secretsmanager.Interface
			etcdMain   *mocketcd.MockInterface
			etcdEvents *mocketcd.MockInterface
			botanist   *Botanist

			ctx            = context.Background()
			namespace      = "shoot--foo--bar"
			createCASecret func(name string, caCrt, caKey string)
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			sm = fakesecretsmanager.New(fakeClient, namespace)
			etcdMain = mocketcd.NewMockInterface(ctrl)
			etcdEvents = mocketcd.NewMockInterface(ctrl)

			botanist = &Botanist{Operation: &operation.Operation{}}
			botanist.SecretsManager = sm
			botanist.Shoot = &shootpkg.Shoot{
				ControlPlaneNamespace: namespace,
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						EtcdMain:   etcdMain,
						EtcdEvents: etcdEvents,
					},
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

			createCASecret = func(name, caCrt, caKey string) {
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data:       map[string][]byte{"ca.crt": []byte(caCrt), "ca.key": []byte(caKey)},
				})).To(Succeed())
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		fileWithContent := func(path, content string) extensionsv1alpha1.File {
			return extensionsv1alpha1.File{
				Path:        path,
				Permissions: new(int32(0600)),
				Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(content))}},
			}
		}

		When("certificate authorities are not being rotated", func() {
			It("should write both the current and old CAs for the client and peer CAs", func() {
				createCASecret("ca-etcd-current", "etcd-current-crt", "etcd-current-key")
				createCASecret("ca-etcd-old", "etcd-old-crt", "etcd-old-key")
				createCASecret("ca-etcd-peer-current", "peer-current-crt", "peer-current-key")
				createCASecret("ca-etcd-peer-old", "peer-old-crt", "peer-old-key")

				files, err := AppendEtcdCAsToFiles(botanist, ctx, nil)
				Expect(err).NotTo(HaveOccurred())

				Expect(files).To(ConsistOf(
					fileWithContent("/var/lib/etcd/ca/current/ca.crt", "etcd-current-crt"),
					fileWithContent("/var/lib/etcd/ca/current/ca.key", "etcd-current-key"),
					fileWithContent("/var/lib/etcd/ca/old/ca.crt", "etcd-old-crt"),
					fileWithContent("/var/lib/etcd/ca/old/ca.key", "etcd-old-key"),
					fileWithContent("/var/lib/etcd/ca-peer/current/ca.crt", "peer-current-crt"),
					fileWithContent("/var/lib/etcd/ca-peer/current/ca.key", "peer-current-key"),
					fileWithContent("/var/lib/etcd/ca-peer/old/ca.crt", "peer-old-crt"),
					fileWithContent("/var/lib/etcd/ca-peer/old/ca.key", "peer-old-key"),
				))
			})

			It("should preserve the passed-in files", func() {
				existingFile := fileWithContent("/some/other/file", "foo")

				files, err := AppendEtcdCAsToFiles(botanist, ctx, []extensionsv1alpha1.File{existingFile})
				Expect(err).NotTo(HaveOccurred())

				Expect(files).To(ContainElement(existingFile))
			})
		})

		When("certificate authorities are being prepared", func() {
			BeforeEach(func() {
				botanist.Shoot.GetInfo().Status.Credentials = &gardencorev1beta1.ShootCredentials{
					Rotation: &gardencorev1beta1.ShootCredentialsRotation{
						CertificateAuthorities: &gardencorev1beta1.CARotation{Phase: "Preparing"},
					},
				}

				createCASecret("ca-etcd-current", "etcd-current-crt", "etcd-current-key")
				createCASecret("ca-etcd-old", "etcd-old-crt", "etcd-old-key")
				createCASecret("ca-etcd-peer-current", "peer-current-crt", "peer-current-key")
				createCASecret("ca-etcd-peer-old", "peer-old-crt", "peer-old-key")
			})

			When("the peer CA rollout is not yet completed", func() {
				BeforeEach(func() {
					// The members are checked in non-deterministic (map iteration) order and the check short-circuits
					// on the first member that is not yet rolled out, so both mocks must allow the call.
					etcdMain.EXPECT().IsPeerCARolledOut(ctx).Return(false, nil).AnyTimes()
					etcdEvents.EXPECT().IsPeerCARolledOut(ctx).Return(false, nil).AnyTimes()
				})

				It("should mask the old etcd and peer CAs as current and omit the old folders", func() {
					files, err := AppendEtcdCAsToFiles(botanist, ctx, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(files).To(ConsistOf(
						fileWithContent("/var/lib/etcd/ca/current/ca.crt", "etcd-old-crt"),
						fileWithContent("/var/lib/etcd/ca/current/ca.key", "etcd-old-key"),
						fileWithContent("/var/lib/etcd/ca-peer/current/ca.crt", "peer-old-crt"),
						fileWithContent("/var/lib/etcd/ca-peer/current/ca.key", "peer-old-key"),
					))
				})

				It("should fall back to the current CAs when no old ones exist", func() {
					Expect(fakeClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd-old", Namespace: namespace}})).To(Succeed())
					Expect(fakeClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd-peer-old", Namespace: namespace}})).To(Succeed())

					files, err := AppendEtcdCAsToFiles(botanist, ctx, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(files).To(ConsistOf(
						fileWithContent("/var/lib/etcd/ca/current/ca.crt", "etcd-current-crt"),
						fileWithContent("/var/lib/etcd/ca/current/ca.key", "etcd-current-key"),
						fileWithContent("/var/lib/etcd/ca-peer/current/ca.crt", "peer-current-crt"),
						fileWithContent("/var/lib/etcd/ca-peer/current/ca.key", "peer-current-key"),
					))
				})
			})

			When("the peer CA rollout is completed for all members", func() {
				It("should write both the current and old etcd and peer CAs", func() {
					etcdMain.EXPECT().IsPeerCARolledOut(ctx).Return(true, nil)
					etcdEvents.EXPECT().IsPeerCARolledOut(ctx).Return(true, nil)

					files, err := AppendEtcdCAsToFiles(botanist, ctx, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(files).To(ConsistOf(
						fileWithContent("/var/lib/etcd/ca/current/ca.crt", "etcd-current-crt"),
						fileWithContent("/var/lib/etcd/ca/current/ca.key", "etcd-current-key"),
						fileWithContent("/var/lib/etcd/ca/old/ca.crt", "etcd-old-crt"),
						fileWithContent("/var/lib/etcd/ca/old/ca.key", "etcd-old-key"),
						fileWithContent("/var/lib/etcd/ca-peer/current/ca.crt", "peer-current-crt"),
						fileWithContent("/var/lib/etcd/ca-peer/current/ca.key", "peer-current-key"),
						fileWithContent("/var/lib/etcd/ca-peer/old/ca.crt", "peer-old-crt"),
						fileWithContent("/var/lib/etcd/ca-peer/old/ca.key", "peer-old-key"),
					))
				})
			})

			It("should return an error when the peer CA rollout check fails", func() {
				etcdMain.EXPECT().IsPeerCARolledOut(ctx).Return(false, errors.New("fake err")).AnyTimes()
				etcdEvents.EXPECT().IsPeerCARolledOut(ctx).Return(false, errors.New("fake err")).AnyTimes()

				_, err := AppendEtcdCAsToFiles(botanist, ctx, nil)
				Expect(err).To(MatchError(ContainSubstring("fake err")))
			})
		})
	})
})
