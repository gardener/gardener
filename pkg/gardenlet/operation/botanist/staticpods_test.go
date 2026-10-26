// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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
})
