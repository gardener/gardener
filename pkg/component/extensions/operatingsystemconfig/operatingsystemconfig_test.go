// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/gardeneruser"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/sshdensurer"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/features"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/version"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("OperatingSystemConfig", func() {
	Describe("> Interface", func() {
		const (
			namespace = "test-namespace"

			worker1Name = "worker1"
			worker2Name = "worker2"

			inPlaceWorkerName1 = worker1Name + "-in-place"
			inPlaceWorkerName2 = worker2Name + "-in-place"
		)

		var (
			ctrl             *gomock.Controller
			c                client.Client
			fakeClient       client.Client
			sm               secretsmanager.Interface
			defaultDepWaiter Interface

			ctx     context.Context
			values  *Values
			log     logr.Logger
			fakeErr = fmt.Errorf("some random error")

			mockNow *mocktime.MockNow
			now     time.Time

			poolHashesSecret    *corev1.Secret
			apiServerURL        = "https://url-to-apiserver"
			caBundle            = ptr.To("ca-bundle")
			clusterDNSAddresses = []string{"cluster-dns", "backup-cluster-dns"}
			clusterDomain       = "cluster-domain"
			images              = map[string]*imagevector.Image{
				"gardener-node-agent": {},
				"pause-container":     {Repository: ptr.To("registry.k8s.io/pause"), Tag: ptr.To("latest")},
			}
			evictionHardMemoryAvailable = "100Mi"
			kubeletConfig               = &gardencorev1beta1.KubeletConfig{
				EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
					MemoryAvailable: &evictionHardMemoryAvailable,
				},
			}
			kubeletDataVolumeName   = "foo"
			machineTypes            []gardencorev1beta1.MachineType
			sshPublicKeys           = []string{"ssh-public-key", "ssh-public-key-b"}
			kubernetesVersion       = semver.MustParse("1.2.3")
			workerKubernetesVersion = "4.5.6"
			valitailEnabled         = false

			//nolint:unparam
			initConfigFn = func(worker gardencorev1beta1.Worker, nodeAgentImage string, config *nodeagentconfigv1alpha1.NodeAgentConfiguration) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
				return []extensionsv1alpha1.Unit{
						{Name: worker.Name},
						{
							Name:    "gardener-node-init.service",
							Content: &nodeAgentImage,
						},
					},
					[]extensionsv1alpha1.File{
						{Path: config.APIServer.Server},
						{Path: ""},
						{Path: "/var/lib/gardener-node-agent/init.sh"},
					},
					nil
			}
			//nolint:unparam
			originalConfigFn = func(cctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
				return []extensionsv1alpha1.Unit{
						{Name: cctx.Key},
						{Name: *cctx.CABundle},
						{Name: strings.Join(cctx.ClusterDNSAddresses, "-")},
						{Name: cctx.ClusterDomain},
						{Name: string(cctx.CRIName)},
					},
					[]extensionsv1alpha1.File{
						{Path: fmt.Sprintf("%s", cctx.Images)},
						{Path: string(cctx.KubeletCABundle)},
						{Path: fmt.Sprintf("%v", cctx.KubeletCLIFlags)},
						{Path: fmt.Sprintf("%v", cctx.KubeletConfigParameters)},
						{Path: *cctx.KubeletDataVolumeName},
						{Path: cctx.KubernetesVersion.String()},
						{Path: fmt.Sprintf("%s", cctx.SSHPublicKeys)},
						{Path: strconv.FormatBool(cctx.ValitailEnabled)},
						{Path: fmt.Sprintf("%+v", cctx.Taints)},
					},
					nil
			}

			workers = []gardencorev1beta1.Worker{
				{
					Name: worker1Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:           "type1",
							Version:        ptr.To("12.34"),
							ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					ControlPlane:          &gardencorev1beta1.WorkerControlPlane{},
				},
				{
					Name: worker2Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "type2",
							Version: ptr.To("12.34"),
						},
					},
					CRI: &gardencorev1beta1.CRI{
						Name: gardencorev1beta1.CRINameContainerD,
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Version: &workerKubernetesVersion,
					},
				},
			}

			inPlaceUpdateWorkers = []gardencorev1beta1.Worker{
				{
					Name: worker1Name + "-in-place",
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:           "type1",
							Version:        ptr.To("12.34"),
							ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Kubelet: &gardencorev1beta1.KubeletConfig{
							KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
								CPU:    ptr.To(resource.MustParse("100m")),
								Memory: ptr.To(resource.MustParse("100Mi")),
							},
						},
					},

					UpdateStrategy: ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
				},
				{
					Name: worker2Name + "-in-place",
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "type2",
							Version: ptr.To("12.34"),
						},
					},
					CRI: &gardencorev1beta1.CRI{
						Name: gardencorev1beta1.CRINameContainerD,
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Version: &workerKubernetesVersion,
						Kubelet: &gardencorev1beta1.KubeletConfig{},
					},
					UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
				},
			}
			empty    *extensionsv1alpha1.OperatingSystemConfig
			expected []*extensionsv1alpha1.OperatingSystemConfig

			globalLastInitiationTime = &metav1.Time{Time: time.Date(2020, 12, 2, 10, 0, 0, 0, time.UTC)}
		)

		computeExpectedOperatingSystemConfigs := func(sshAccessEnabled, inPlaceUpdate bool) []*extensionsv1alpha1.OperatingSystemConfig {
			w := workers
			if inPlaceUpdate {
				w = inPlaceUpdateWorkers
			}

			expected := make([]*extensionsv1alpha1.OperatingSystemConfig, 0, 2*len(w))
			for _, worker := range w {
				var (
					criName               = extensionsv1alpha1.CRINameContainerD
					criConfig             *extensionsv1alpha1.CRIConfig
					criConfigProvisioning *extensionsv1alpha1.CRIConfig
				)

				k8sVersion := values.KubernetesVersion
				if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
					k8sVersion = semver.MustParse(*worker.Kubernetes.Version)
				}

				if worker.CRI != nil {
					criName = extensionsv1alpha1.CRIName(worker.CRI.Name)
					criConfig = &extensionsv1alpha1.CRIConfig{
						Name: extensionsv1alpha1.CRIName(worker.CRI.Name),
						Containerd: &extensionsv1alpha1.ContainerdConfig{
							SandboxImage: "registry.k8s.io/pause:latest",
						},
					}
					if version.ConstraintK8sGreaterEqual131.Check(k8sVersion) {
						criConfig.CgroupDriver = ptr.To(extensionsv1alpha1.CgroupDriverSystemd)
					}

					criConfigProvisioning = &extensionsv1alpha1.CRIConfig{
						Name: extensionsv1alpha1.CRIName(worker.CRI.Name),
					}
				}

				key := KeyV1(worker.Name, k8sVersion, worker.CRI)

				imagesCopy := make(map[string]*imagevector.Image, len(images))
				for imageName, image := range images {
					imagesCopy[imageName] = image
				}
				imagesCopy["hyperkube"] = &imagevector.Image{Repository: ptr.To("europe-docker.pkg.dev/gardener-project/releases/hyperkube"), Tag: ptr.To("v" + k8sVersion.String())}

				initUnits, initFiles, _ := initConfigFn(
					worker,
					imagesCopy["gardener-node-agent"].String(),
					&nodeagentconfigv1alpha1.NodeAgentConfiguration{APIServer: nodeagentconfigv1alpha1.APIServer{
						Server:   apiServerURL,
						CABundle: []byte(*caBundle),
					}},
				)
				componentsContext := components.Context{
					Key:                 key,
					CABundle:            caBundle,
					ClusterDNSAddresses: clusterDNSAddresses,
					ClusterDomain:       clusterDomain,
					CRIName:             criName,
					Images:              imagesCopy,
					KubeletConfigParameters: components.ConfigurableKubeletConfigParameters{
						EvictionHard: map[string]string{
							"memory.available": evictionHardMemoryAvailable,
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					KubernetesVersion:     k8sVersion,
					SSHAccessEnabled:      true,
					SSHPublicKeys:         sshPublicKeys,
					ValitailEnabled:       valitailEnabled,
				}

				if worker.ControlPlane != nil {
					componentsContext.KubeletConfigParameters.WithStaticPodPath = true
					componentsContext.Taints = append(componentsContext.Taints, corev1.Taint{
						Key:    "node-role.kubernetes.io/control-plane",
						Effect: corev1.TaintEffectNoSchedule,
					})
				}

				originalUnits, originalFiles, _ := originalConfigFn(componentsContext)

				oscInit := &extensionsv1alpha1.OperatingSystemConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key + "-" + worker.Machine.Image.Name + "-init",
						Namespace: namespace,
						Annotations: map[string]string{
							v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
							v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						},
						Labels: map[string]string{
							"worker.gardener.cloud/pool":                                         worker.Name,
							"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
						},
					},
					Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type:           worker.Machine.Image.Name,
							ProviderConfig: worker.Machine.Image.ProviderConfig,
						},
						CRIConfig: criConfigProvisioning,
						Purpose:   extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
					},
				}

				oscInit.Spec.Units = initUnits
				oscInit.Spec.Files = initFiles
				if sshAccessEnabled {
					gUnits, gFiles, err := gardeneruser.New().Config(componentsContext)
					Expect(err).ToNot(HaveOccurred())
					oscInit.Spec.Units = append(oscInit.Spec.Units, gUnits...)
					oscInit.Spec.Files = append(oscInit.Spec.Files, gFiles...)
					sUnits, sFiles, err := sshdensurer.New().Config(componentsContext)
					Expect(err).ToNot(HaveOccurred())
					oscInit.Spec.Units = append(oscInit.Spec.Units, sUnits...)
					oscInit.Spec.Files = append(oscInit.Spec.Files, sFiles...)
				}

				oscOriginal := &extensionsv1alpha1.OperatingSystemConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key + "-" + worker.Machine.Image.Name + "-original",
						Namespace: namespace,
						Annotations: map[string]string{
							v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
							v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						},
						Labels: map[string]string{
							"worker.gardener.cloud/pool":                                         worker.Name,
							"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
						},
					},
					Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type:           worker.Machine.Image.Name,
							ProviderConfig: worker.Machine.Image.ProviderConfig,
						},
						Purpose:   extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
						CRIConfig: criConfig,
						Units:     originalUnits,
						Files:     originalFiles,
					},
				}

				if inPlaceUpdate {
					componentsContext.KubeletConfigParameters = components.ConfigurableKubeletConfigParameters{}
					if worker.Kubernetes.Kubelet.KubeReserved != nil {
						componentsContext.KubeletConfigParameters.KubeReserved = map[string]string{
							"cpu":    "100m",
							"memory": "100Mi",
						}
					}

					originalUnits, originalFiles, _ := originalConfigFn(componentsContext)

					oscOriginal.Spec.Units = originalUnits
					oscOriginal.Spec.Files = originalFiles

					caRotationLastInitiationTime := globalLastInitiationTime
					if worker.Name == inPlaceWorkerName1 {
						caRotationLastInitiationTime = &metav1.Time{Time: time.Date(2020, 12, 2, 1, 0, 0, 0, time.UTC)}
					}

					serviceAccountKeyRotationLastInitiationTime := globalLastInitiationTime
					if worker.Name == inPlaceWorkerName2 {
						serviceAccountKeyRotationLastInitiationTime = &metav1.Time{Time: time.Date(2020, 12, 2, 2, 0, 0, 0, time.UTC)}
					}

					oscOriginal.Spec.InPlaceUpdates = &extensionsv1alpha1.InPlaceUpdates{
						OperatingSystemVersion: *worker.Machine.Image.Version,
						KubeletVersion:         k8sVersion.String(),
						CredentialsRotation: &extensionsv1alpha1.CredentialsRotation{
							CertificateAuthorities: &extensionsv1alpha1.CARotation{
								LastInitiationTime: caRotationLastInitiationTime,
							},
							ServiceAccountKey: &extensionsv1alpha1.ServiceAccountKeyRotation{
								LastInitiationTime: serviceAccountKeyRotationLastInitiationTime,
							},
						},
					}
				}

				expected = append(expected, oscInit, oscOriginal)
			}
			return expected
		}

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockNow = mocktime.NewMockNow(ctrl)
			now = time.Now()

			ctx = context.TODO()
			log = logr.Discard()

			s := runtime.NewScheme()
			Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())
			Expect(fakekubernetes.AddToScheme(s)).To(Succeed())
			c = fakeclient.NewClientBuilder().WithScheme(s).Build()

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			sm = fakesecretsmanager.New(fakeClient, namespace)

			By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-kubelet", Namespace: namespace}})).To(Succeed())

			workers = []gardencorev1beta1.Worker{
				{
					Name: worker1Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:           "type1",
							Version:        ptr.To("12.34"),
							ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					ControlPlane:          &gardencorev1beta1.WorkerControlPlane{},
				},
				{
					Name: worker2Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "type2",
							Version: ptr.To("12.34"),
						},
					},
					CRI: &gardencorev1beta1.CRI{
						Name: gardencorev1beta1.CRINameContainerD,
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Version: &workerKubernetesVersion,
					},
				},
			}
			inPlaceUpdateWorkers = []gardencorev1beta1.Worker{
				{
					Name: inPlaceWorkerName1,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:           "type1",
							Version:        ptr.To("12.34"),
							ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Kubelet: &gardencorev1beta1.KubeletConfig{
							KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
								CPU:    ptr.To(resource.MustParse("100m")),
								Memory: ptr.To(resource.MustParse("100Mi")),
							},
						},
					},

					UpdateStrategy: ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
				},
				{
					Name: inPlaceWorkerName2,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "type2",
							Version: ptr.To("12.34"),
						},
					},
					CRI: &gardencorev1beta1.CRI{
						Name: gardencorev1beta1.CRINameContainerD,
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Version: &workerKubernetesVersion,
						Kubelet: &gardencorev1beta1.KubeletConfig{},
					},
					UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
				},
			}

			values = &Values{
				Namespace:         namespace,
				Workers:           workers,
				KubernetesVersion: kubernetesVersion,
				InitValues: InitValues{
					APIServerURL: apiServerURL,
				},
				OriginalValues: OriginalValues{
					CABundle:            caBundle,
					ClusterDNSAddresses: clusterDNSAddresses,
					ClusterDomain:       clusterDomain,
					Images:              images,
					KubeletConfig:       kubeletConfig,
					MachineTypes:        machineTypes,
					SSHPublicKeys:       sshPublicKeys,
					ValitailEnabled:     valitailEnabled,
				},
			}

			empty = &extensionsv1alpha1.OperatingSystemConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
				},
			}

			poolHashesSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-pools-operatingsystemconfig-hashes",
					Namespace: namespace,
					Labels: map[string]string{
						"persist": "true",
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"pools": []byte(`pools:
    - name: worker1
      currentVersion: 1
      hashVersionToOSCKey:
        1: gardener-node-agent-worker1-77ac3
    - name: worker2
      currentVersion: 1
      hashVersionToOSCKey:
        1: gardener-node-agent-worker2-d9e53
`)},
			}

			expected = computeExpectedOperatingSystemConfigs(false, false)
			defaultDepWaiter = New(log, c, sm, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#Deploy", func() {
			It("should successfully deploy the shoot access secret for the gardener-node-agent when NodeAgentAuthorizer feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.NodeAgentAuthorizer, false))
				DeferCleanup(test.WithVars(
					&OriginalConfigFn, originalConfigFn,
				))

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Name: "shoot-access-gardener-node-agent", Namespace: namespace}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				}))
				Expect(secret.Annotations).To(Equal(map[string]string{
					"serviceaccount.resources.gardener.cloud/name":                      "gardener-node-agent",
					"serviceaccount.resources.gardener.cloud/namespace":                 "kube-system",
					"serviceaccount.resources.gardener.cloud/token-expiration-duration": "720h",
					"token-requestor.resources.gardener.cloud/target-secret-name":       "gardener-node-agent",
					"token-requestor.resources.gardener.cloud/target-secret-namespace":  "kube-system",
				}))
			})

			It("should successfully create the worker-pools-operatingsystemconfig-hashes secret", func() {
				DeferCleanup(test.WithVars(
					&OriginalConfigFn, originalConfigFn,
				))

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())
				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Name: "worker-pools-operatingsystemconfig-hashes", Namespace: namespace}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{
					"persist": "true",
				}))
				pools := secret.Data["pools"]
				Expect(string(pools)).To(Equal(`pools:
    - name: worker1
      currentVersion: 1
      hashVersionToOSCKey:
        1: gardener-node-agent-worker1-77ac3
    - name: worker2
      currentVersion: 1
      hashVersionToOSCKey:
        1: gardener-node-agent-worker2-d9e53
`))
			})

			calculateKeyForVersionFn := func(
				oscVersion int,
				_ *semver.Version,
				_ *Values,
				worker *gardencorev1beta1.Worker,
				_ *gardencorev1beta1.KubeletConfig,
			) (
				string,
				error,
			) {
				switch oscVersion {
				case 1:
					return worker.Name + "-version1", nil
				case 2:
					return worker.Name + "-version2", nil
				default:
					return "", fmt.Errorf("unsupported osc key version %v", oscVersion)
				}
			}

			It("should successfully fill missing hashes and workers in the worker-pools-operatingsystemconfig-hashes secret", func() {
				DeferCleanup(test.WithVars(
					&OriginalConfigFn, originalConfigFn,
					&LatestHashVersion, func() int { return 2 },
					&CalculateKeyForVersion, calculateKeyForVersionFn,
				))

				poolHashesSecret.Data["pools"] = []byte(`pools:
    - name: worker1
      currentVersion: 1
`)
				Expect(c.Create(ctx, poolHashesSecret)).To(Succeed())
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Name: "worker-pools-operatingsystemconfig-hashes", Namespace: namespace}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{
					"persist": "true",
				}))
				pools := secret.Data["pools"]
				Expect(string(pools)).To(Equal(`pools:
    - name: worker1
      currentVersion: 1
      hashVersionToOSCKey:
        1: worker1-version1
        2: worker1-version2
    - name: worker2
      currentVersion: 2
      hashVersionToOSCKey:
        2: worker2-version2
`))

			})

			It("should successfully upgrade the hash versions in the worker-pools-operatingsystemconfig-hashes secret", func() {
				DeferCleanup(test.WithVars(
					&OriginalConfigFn, originalConfigFn,
					&LatestHashVersion, func() int { return 2 },
					&CalculateKeyForVersion, calculateKeyForVersionFn,
				))

				Expect(c.Create(ctx, poolHashesSecret)).To(Succeed())
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Name: "worker-pools-operatingsystemconfig-hashes", Namespace: namespace}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{
					"persist": "true",
				}))
				pools := secret.Data["pools"]
				Expect(string(pools)).To(Equal(`pools:
    - name: worker1
      currentVersion: 2
      hashVersionToOSCKey:
        2: worker1-version2
    - name: worker2
      currentVersion: 2
      hashVersionToOSCKey:
        2: worker2-version2
`))
			})

			calculateStableKeyForVersionFn := func(
				oscVersion int,
				kubernetesVersion *semver.Version,
				_ *Values,
				worker *gardencorev1beta1.Worker,
				_ *gardencorev1beta1.KubeletConfig,
			) (string, error) {
				switch oscVersion {
				case 1:
					return KeyV1(worker.Name, kubernetesVersion, worker.CRI), nil
				case 2:
					return worker.Name + "-version2", nil
				default:
					return "", fmt.Errorf("unsupported osc key version %v", oscVersion)
				}
			}

			It("should successfully keep the current hash versions if nothing changes in the worker-pools-operatingsystemconfig-hashes secret", func() {
				DeferCleanup(test.WithVars(
					&OriginalConfigFn, originalConfigFn,
					&LatestHashVersion, func() int { return 2 },
					&CalculateKeyForVersion, calculateStableKeyForVersionFn,
				))

				Expect(c.Create(ctx, poolHashesSecret)).To(Succeed())
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Name: "worker-pools-operatingsystemconfig-hashes", Namespace: namespace}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{
					"persist": "true",
				}))
				pools := secret.Data["pools"]
				Expect(string(pools)).To(Equal(`pools:
    - name: worker1
      currentVersion: 1
      hashVersionToOSCKey:
        1: gardener-node-agent-worker1-77ac3
        2: worker1-version2
    - name: worker2
      currentVersion: 1
      hashVersionToOSCKey:
        1: gardener-node-agent-worker2-d9e53
        2: worker2-version2
`))
			})

			It("should successfully deploy all extensions resources", func() {
				DeferCleanup(test.WithVars(
					&TimeNow, mockNow.Do,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
				))

				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range computeExpectedOperatingSystemConfigs(false, false) {
					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

					obj := e.DeepCopy()
					obj.ResourceVersion = "1"

					Expect(actual).To(Equal(obj))
				}
			})

			It("should successfully deploy all extensions resources and SSH access is enabled", func() {
				DeferCleanup(test.WithVars(
					&TimeNow, mockNow.Do,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
					&values.SSHAccessEnabled, true,
				))

				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range computeExpectedOperatingSystemConfigs(true, false) {
					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

					obj := e.DeepCopy()
					obj.ResourceVersion = "1"

					Expect(actual).To(Equal(obj))
				}
			})

			Context("In-place update", func() {
				BeforeEach(func() {
					values = &Values{
						Namespace:         namespace,
						Workers:           inPlaceUpdateWorkers,
						KubernetesVersion: kubernetesVersion,
						InitValues: InitValues{
							APIServerURL: apiServerURL,
						},
						OriginalValues: OriginalValues{
							CABundle:            caBundle,
							ClusterDNSAddresses: clusterDNSAddresses,
							ClusterDomain:       clusterDomain,
							Images:              images,
							KubeletConfig:       kubeletConfig,
							MachineTypes:        machineTypes,
							SSHPublicKeys:       sshPublicKeys,
							ValitailEnabled:     valitailEnabled,
						},
						CredentialsRotationStatus: &gardencorev1beta1.ShootCredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								LastInitiationTime: globalLastInitiationTime,
								PendingWorkersRollouts: []gardencorev1beta1.PendingWorkersRollout{
									{
										Name:               inPlaceWorkerName1,
										LastInitiationTime: &metav1.Time{Time: time.Date(2020, 12, 2, 1, 0, 0, 0, time.UTC)},
									},
								},
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								LastInitiationTime: globalLastInitiationTime,
								PendingWorkersRollouts: []gardencorev1beta1.PendingWorkersRollout{
									{
										Name:               inPlaceWorkerName2,
										LastInitiationTime: &metav1.Time{Time: time.Date(2020, 12, 2, 2, 0, 0, 0, time.UTC)},
									},
								},
							},
						},
					}

					defaultDepWaiter = New(log, c, sm, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
				})

				It("should successfully deploy all extensions resources and SSH access is enabled", func() {
					DeferCleanup(test.WithVars(
						&TimeNow, mockNow.Do,
						&InitConfigFn, initConfigFn,
						&OriginalConfigFn, originalConfigFn,
						&values.SSHAccessEnabled, true,
						&format.MaxLength, 0,
					))

					mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

					Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

					for _, e := range computeExpectedOperatingSystemConfigs(true, true) {
						actual := &extensionsv1alpha1.OperatingSystemConfig{}
						Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

						obj := e.DeepCopy()
						obj.ResourceVersion = "1"

						// Getting the object from the client results in the location set to the local timezone
						if obj.Spec.InPlaceUpdates != nil && obj.Spec.InPlaceUpdates.CredentialsRotation != nil {
							actual.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities.LastInitiationTime = &metav1.Time{Time: actual.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities.LastInitiationTime.UTC()}
							actual.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey.LastInitiationTime = &metav1.Time{Time: actual.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey.LastInitiationTime.UTC()}
						}

						Expect(actual).To(Equal(obj))
					}
				})
			})

			It("should exclude the bootstrap token file if purpose is not provision", func() {
				bootstrapTokenFile := extensionsv1alpha1.File{Path: "/var/lib/gardener-node-agent/credentials/bootstrap-token"}
				initConfigFnWithBootstrapToken := func(worker gardencorev1beta1.Worker, nodeAgentImage string, config *nodeagentconfigv1alpha1.NodeAgentConfiguration) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
					units, files, err := initConfigFn(worker, nodeAgentImage, config)
					return units, append(files, bootstrapTokenFile), err
				}

				defer test.WithVars(
					&TimeNow, mockNow.Do,
					&InitConfigFn, initConfigFnWithBootstrapToken,
					&OriginalConfigFn, originalConfigFn,
				)()

				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range expected {
					if e.Spec.Purpose == extensionsv1alpha1.OperatingSystemConfigPurposeProvision {
						e.Spec.Files = append(e.Spec.Files, bootstrapTokenFile)
					}

					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

					obj := e.DeepCopy()

					obj.ResourceVersion = "1"

					Expect(actual).To(Equal(obj))
				}
			})
		})

		Describe("#Restore", func() {
			var (
				stateInit     = []byte(`{"dummy":"state init"}`)
				stateOriginal = []byte(`{"dummy":"state original"}`)
				shootState    *gardencorev1beta1.ShootState
			)

			BeforeEach(func() {
				extensions := make([]gardencorev1beta1.ExtensionResourceState, 0, 2*len(workers))
				for _, worker := range workers {
					k8sVersion := values.KubernetesVersion
					if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
						k8sVersion = semver.MustParse(*worker.Kubernetes.Version)
					}
					key := KeyV1(worker.Name, k8sVersion, worker.CRI)

					extensions = append(extensions,
						gardencorev1beta1.ExtensionResourceState{
							Name:    ptr.To(key + "-" + worker.Machine.Image.Name + "-init"),
							Kind:    extensionsv1alpha1.OperatingSystemConfigResource,
							Purpose: ptr.To(string(extensionsv1alpha1.OperatingSystemConfigPurposeProvision)),
							State:   &runtime.RawExtension{Raw: stateInit},
						},
						gardencorev1beta1.ExtensionResourceState{
							Name:    ptr.To(key + "-" + worker.Machine.Image.Name + "-original"),
							Kind:    extensionsv1alpha1.OperatingSystemConfigResource,
							Purpose: ptr.To(string(extensionsv1alpha1.OperatingSystemConfigPurposeReconcile)),
							State:   &runtime.RawExtension{Raw: stateOriginal},
						},
					)
				}
				shootState = &gardencorev1beta1.ShootState{
					Spec: gardencorev1beta1.ShootStateSpec{
						Extensions: extensions,
					},
				}
			})

			It("should properly restore the extensions state if it exists when NodeAgentAuthorizer feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.NodeAgentAuthorizer, false))
				defer test.WithVars(
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
					&TimeNow, mockNow.Do,
					&extensions.TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				mc := mockclient.NewMockClient(ctrl)
				mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

				mc.EXPECT().Status().Return(mockStatusWriter).AnyTimes()

				mc.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot-access-gardener-node-agent"}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				mc.EXPECT().Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-access-gardener-node-agent",
						Namespace: namespace,
						Annotations: map[string]string{
							"serviceaccount.resources.gardener.cloud/name":                      "gardener-node-agent",
							"serviceaccount.resources.gardener.cloud/namespace":                 "kube-system",
							"serviceaccount.resources.gardener.cloud/token-expiration-duration": "720h",
							"token-requestor.resources.gardener.cloud/target-secret-name":       "gardener-node-agent",
							"token-requestor.resources.gardener.cloud/target-secret-namespace":  "kube-system",
						},
						Labels: map[string]string{
							"resources.gardener.cloud/purpose": "token-requestor",
							"resources.gardener.cloud/class":   "shoot",
						},
					},
					Type: corev1.SecretTypeOpaque,
				})

				for i := range expected {
					var state []byte
					if strings.HasSuffix(expected[i].Name, "init") {
						state = stateInit
					} else {
						state = stateOriginal
					}

					emptyWithName := empty.DeepCopy()
					emptyWithName.SetName(expected[i].GetName())
					mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(emptyWithName), gomock.AssignableToTypeOf(emptyWithName)).
						Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("operatingsystemconfigs"), emptyWithName.GetName()))

					// deploy with wait-for-state annotation
					obj := expected[i].DeepCopy()
					metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
					metav1.SetMetaDataAnnotation(&obj.ObjectMeta, "gardener.cloud/timestamp", now.UTC().Format(time.RFC3339Nano))
					obj.TypeMeta = metav1.TypeMeta{}
					mc.EXPECT().Create(ctx, test.HasObjectKeyOf(obj)).
						DoAndReturn(func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
							Expect(actual).To(DeepEqual(obj))
							return nil
						})

					// restore state
					expectedWithState := obj.DeepCopy()
					expectedWithState.Status.State = &runtime.RawExtension{Raw: state}
					test.EXPECTStatusPatch(ctx, mockStatusWriter, expectedWithState, obj, types.MergePatchType)

					// annotate with restore annotation
					expectedWithRestore := expectedWithState.DeepCopy()
					metav1.SetMetaDataAnnotation(&expectedWithRestore.ObjectMeta, "gardener.cloud/operation", "restore")
					test.EXPECTPatch(ctx, mc, expectedWithRestore, expectedWithState, types.MergePatchType)
				}

				clientGet := func(result client.Object) interface{} {
					return func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
						switch obj.(type) {
						case *corev1.Secret:
							*obj.(*corev1.Secret) = *result.(*corev1.Secret)
						}
						return nil
					}
				}

				mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(poolHashesSecret), gomock.AssignableToTypeOf(poolHashesSecret)).
					DoAndReturn(clientGet(poolHashesSecret))
				mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(poolHashesSecret), client.RawPatch(types.MergePatchType, []byte("{}"))).
					Return(nil)

				defaultDepWaiter = New(log, mc, sm, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
				Expect(defaultDepWaiter.Restore(ctx, shootState)).To(Succeed())
			})
		})

		Describe("#Wait", func() {
			It("should return error when no resources are found", func() {
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())
				for i := range expected {
					Expect(c.Delete(ctx, expected[i])).To(Succeed())
				}

				Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should return error when resource is not ready", func() {
				defer test.WithVars(&TimeNow, mockNow.Do)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				errDescription := "Some error"

				for i := range expected {
					Expect(c.Delete(ctx, expected[i])).To(Succeed())
					expected[i].Status.LastError = &gardencorev1beta1.LastError{
						Description: errDescription,
					}
					// unconditional replace for testing
					Expect(c.Create(ctx, expected[i])).To(Succeed())
				}

				Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("error during reconciliation: " + errDescription)))
			})

			It("should return error when status does not contain cloud config information", func() {
				defer test.WithVars(
					&TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				// Deploy should fill internal state with the added timestamp annotation
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for i := range expected {
					Expect(c.Delete(ctx, expected[i])).To(Succeed())
					// remove operation annotation
					expected[i].Annotations = map[string]string{
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					}
					// set last operation
					expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
						State:          gardencorev1beta1.LastOperationStateSucceeded,
						LastUpdateTime: metav1.NewTime(now.UTC()),
					}
					// unconditional replace for testing
					Expect(c.Create(ctx, expected[i])).To(Succeed())
				}

				Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("no cloud config information provided in status")))
			})

			It("should return error if we haven't observed the latest timestamp annotation", func() {
				defer test.WithVars(
					&TimeNow, mockNow.Do,
					&OriginalConfigFn, originalConfigFn,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				By("Deploy")
				// Deploy should fill internal state with the added timestamp annotation
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				By("Patch object")
				for i := range expected {
					patch := client.MergeFrom(expected[i].DeepCopy())
					// remove operation annotation, add old timestamp annotation
					expected[i].Annotations = map[string]string{
						v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
					}
					// set last operation
					expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					}
					// set cloud-config secret information
					expected[i].Status.CloudConfig = &extensionsv1alpha1.CloudConfig{
						SecretRef: corev1.SecretReference{
							Name:      "cc-" + expected[i].Name,
							Namespace: expected[i].Name,
						},
					}
					Expect(c.Patch(ctx, expected[i], patch)).ToNot(HaveOccurred(), "patching operatingsystemconfig succeeds")

					// create cloud-config secret
					ccSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cc-" + expected[i].Name,
							Namespace: expected[i].Name,
						},
						Data: map[string][]byte{
							"cloud_config": []byte("foobar-" + expected[i].Name),
						},
					}
					Expect(c.Create(ctx, ccSecret)).To(Succeed())
				}

				By("Wait")
				Expect(defaultDepWaiter.Wait(ctx)).NotTo(Succeed(), "operatingsystemconfig indicates error")
			})

			It("should return no error when it's ready", func() {
				defer test.WithVars(
					&TimeNow, mockNow.Do,
					&OriginalConfigFn, originalConfigFn,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				By("Deploy")
				// Deploy should fill internal state with the added timestamp annotation
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				By("Patch object")
				for i := range expected {
					patch := client.MergeFrom(expected[i].DeepCopy())
					// remove operation annotation, add up-to-date timestamp annotation
					expected[i].Annotations = map[string]string{
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					}
					// set last operation
					expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
						State:          gardencorev1beta1.LastOperationStateSucceeded,
						LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
					}
					// set cloud-config secret information
					expected[i].Status.CloudConfig = &extensionsv1alpha1.CloudConfig{
						SecretRef: corev1.SecretReference{
							Name:      "cc-" + expected[i].Name,
							Namespace: expected[i].Name,
						},
					}
					Expect(c.Patch(ctx, expected[i], patch)).ToNot(HaveOccurred(), "patching operatingsystemconfig succeeds")

					// create cloud-config secret
					ccSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cc-" + expected[i].Name,
							Namespace: expected[i].Name,
						},
						Data: map[string][]byte{
							"cloud_config": []byte("foobar-" + expected[i].Name),
						},
					}
					Expect(c.Create(ctx, ccSecret)).To(Succeed())
				}

				By("Wait")
				Expect(defaultDepWaiter.Wait(ctx)).To(Succeed(), "operatingsystemconfig is ready")
			})
		})

		Describe("WorkerNameToOperatingSystemConfigsMap", func() {
			It("should return the correct result from the Deploy and Wait operations", func() {
				DeferCleanup(test.WithVars(
					&TimeNow, mockNow.Do,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
				))
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				// Deploy should fill internal state with the added timestamp annotation
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for i := range expected {
					expected[i].ResourceVersion = "1"
				}

				worker1OSCDownloader := expected[0]
				worker1OSCOriginal := expected[1]
				worker2OSCDownloader := expected[2]
				worker2OSCOriginal := expected[3]

				Expect(defaultDepWaiter.WorkerPoolNameToOperatingSystemConfigsMap()).To(Equal(map[string]*OperatingSystemConfigs{
					worker1Name: {
						Init: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-77ac3",
							Object:                      worker1OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-77ac3",
							Object:                      worker1OSCOriginal,
						},
					},
					worker2Name: {
						Init: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-d9e53",
							Object:                      worker2OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-d9e53",
							Object:                      worker2OSCOriginal,
						},
					},
				}))

				for i := range expected {
					// remove operation annotation
					expected[i].Annotations = map[string]string{
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					}
					// set last operation
					lastUpdateTime := metav1.Time{Time: now}.Rfc3339Copy()
					// fix timezone
					lastUpdateTime.Time = lastUpdateTime.Local()
					expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
						State:          gardencorev1beta1.LastOperationStateSucceeded,
						LastUpdateTime: lastUpdateTime,
					}
					// set cloud-config secret information
					expected[i].Status.CloudConfig = &extensionsv1alpha1.CloudConfig{
						SecretRef: corev1.SecretReference{
							Name:      "cc-" + expected[i].Name,
							Namespace: expected[i].Name,
						},
					}
					Expect(c.Update(ctx, expected[i])).To(Succeed())

					// create cloud-config secret
					ccSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cc-" + expected[i].Name,
							Namespace: expected[i].Name,
						},
						Data: map[string][]byte{
							"cloud_config": []byte("foobar-" + expected[i].Name),
						},
					}
					Expect(c.Create(ctx, ccSecret)).To(Succeed())
				}

				Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())

				Expect(defaultDepWaiter.WorkerPoolNameToOperatingSystemConfigsMap()).To(Equal(map[string]*OperatingSystemConfigs{
					worker1Name: {
						Init: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-77ac3",
							SecretName:                  ptr.To("cc-" + expected[0].Name),
							Object:                      worker1OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-77ac3",
							Object:                      worker1OSCOriginal,
						},
					},
					worker2Name: {
						Init: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-d9e53",
							SecretName:                  ptr.To("cc-" + expected[2].Name),
							Object:                      worker2OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-d9e53",
							Object:                      worker2OSCOriginal,
						},
					},
				}))
			})
		})

		Describe("#Destroy", func() {
			It("should not return error when not found", func() {
				Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
			})

			It("should not return error when deleted successfully", func() {
				Expect(c.Create(ctx, expected[0])).To(Succeed())
				Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())
			})

			It("should return error if not deleted successfully", func() {
				defer test.WithVars(
					&extensions.TimeNow, mockNow.Do,
					&gardenerutils.TimeNow, mockNow.Do,
				)()
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				expectedOSC := extensionsv1alpha1.OperatingSystemConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "osc1",
						Namespace: namespace,
						Annotations: map[string]string{
							v1beta1constants.ConfirmationDeletion: "true",
							v1beta1constants.GardenerTimestamp:    now.UTC().Format(time.RFC3339Nano),
						},
					},
				}

				mc := mockclient.NewMockClient(ctrl)
				// check if the operatingsystemconfigs exist
				mc.EXPECT().List(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.OperatingSystemConfigList{}), client.InNamespace(namespace)).SetArg(1, extensionsv1alpha1.OperatingSystemConfigList{Items: []extensionsv1alpha1.OperatingSystemConfig{expectedOSC}})
				// add deletion confirmation and Timestamp annotation
				mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.OperatingSystemConfig{}), gomock.Any())
				mc.EXPECT().Delete(ctx, &expectedOSC).Return(fakeErr)

				defaultDepWaiter = New(log, mc, nil, &Values{Namespace: namespace}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
				Expect(defaultDepWaiter.Destroy(ctx)).To(MatchError(error(multierror.Append(fakeErr))))
			})
		})

		Describe("#WaitCleanup", func() {
			It("should not return error if all resources are gone", func() {
				Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
			})

			It("should return error if resources still exist", func() {
				Expect(c.Create(ctx, expected[0])).To(Succeed())
				Expect(defaultDepWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring("OperatingSystemConfig test-namespace/gardener-node-agent-worker1-77ac3-type1-init is still present")))
			})
		})

		Describe("#Migrate", func() {
			It("should migrate the resources", func() {
				Expect(c.Create(ctx, expected[0])).To(Succeed())

				Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())

				annotatedResource := &extensionsv1alpha1.OperatingSystemConfig{}
				Expect(c.Get(ctx, client.ObjectKey{Name: expected[0].Name, Namespace: expected[0].Namespace}, annotatedResource)).To(Succeed())
				Expect(annotatedResource.Annotations[v1beta1constants.GardenerOperation]).To(Equal(v1beta1constants.GardenerOperationMigrate))
			})

			It("should not return error if resource does not exist", func() {
				Expect(defaultDepWaiter.Migrate(ctx)).To(Succeed())
			})
		})

		Describe("#WaitMigrate", func() {
			It("should not return error when resource is missing", func() {
				Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed())
			})

			It("should return error if resource is not yet migrated successfully", func() {
				expected[0].Status.LastError = &gardencorev1beta1.LastError{
					Description: "Some error",
				}
				expected[0].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateError,
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
				}

				Expect(c.Create(ctx, expected[0])).To(Succeed())
				Expect(defaultDepWaiter.WaitMigrate(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
			})

			It("should not return error if resource gets migrated successfully", func() {
				expected[0].Status.LastError = nil
				expected[0].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
				}

				Expect(c.Create(ctx, expected[0])).To(Succeed())
				Expect(defaultDepWaiter.WaitMigrate(ctx)).To(Succeed())
			})

			It("should return error if one resources is not migrated successfully and others are", func() {
				for i := range expected[1:] {
					expected[i].Status.LastError = nil
					expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
						Type:  gardencorev1beta1.LastOperationTypeMigrate,
					}
				}
				expected[0].Status.LastError = &gardencorev1beta1.LastError{
					Description: "Some error",
				}
				expected[0].Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateError,
					Type:  gardencorev1beta1.LastOperationTypeMigrate,
				}

				for _, e := range expected {
					Expect(c.Create(ctx, e)).To(Succeed())
				}
				Expect(defaultDepWaiter.WaitMigrate(ctx)).To(MatchError(ContainSubstring("to be successfully migrated")))
			})
		})

		Describe("#DeleteStaleResources", func() {
			It("should delete stale extensions resources", func() {
				newType := "new-type"

				staleOSC := expected[0].DeepCopy()
				staleOSC.Name = "new-name"
				Expect(c.Create(ctx, staleOSC)).To(Succeed())

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				Expect(defaultDepWaiter.DeleteStaleResources(ctx)).To(Succeed())

				oscList := &extensionsv1alpha1.OperatingSystemConfigList{}
				Expect(c.List(ctx, oscList)).To(Succeed())
				Expect(oscList.Items).To(HaveLen(2 * len(workers)))
				for _, item := range oscList.Items {
					Expect(item.Spec.Type).ToNot(Equal(newType))
				}
			})
		})

		Describe("#WaitCleanupStaleResources", func() {
			It("should not return error if all resources are gone", func() {

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())
				for i := range expected {
					Expect(c.Delete(ctx, expected[i])).To(Succeed())
				}

				Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should not return error if wanted resources exist", func() {
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should return error if stale resources still exist", func() {
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				staleOSC := expected[0].DeepCopy()
				staleOSC.Name = "new-name"
				Expect(c.Create(ctx, staleOSC)).To(Succeed(), "creating stale OSC succeeds")

				Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("OperatingSystemConfig test-namespace/new-name is still present")))
			})
		})
	})

	Describe("#KeyV1", func() {
		var (
			workerName        = "foo"
			kubernetesVersion = "1.2.3"
		)

		It("should return an empty string", func() {
			Expect(KeyV1(workerName, nil, nil)).To(BeEmpty())
		})

		It("should return the expected key", func() {
			Expect(KeyV1(workerName, semver.MustParse(kubernetesVersion), nil)).To(Equal("gardener-node-agent-" + workerName + "-77ac3"))
		})

		It("is different for different worker.cri configurations", func() {
			containerDKey := KeyV1(workerName, semver.MustParse("1.2.3"), &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD})
			otherKey := KeyV1(workerName, semver.MustParse("1.2.3"), &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRIName("other")})
			Expect(containerDKey).NotTo(Equal(otherKey))
		})

		It("should return the expected key for version 1", func() {
			key, err := CalculateKeyForVersion(1, semver.MustParse(kubernetesVersion), nil,
				&gardencorev1beta1.Worker{
					Name: workerName,
					Machine: gardencorev1beta1.Machine{
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "type1",
							Version: ptr.To("12.34"),
						},
					},
				}, nil)
			Expect(err).To(Succeed())
			Expect(key).To(Equal("gardener-node-agent-" + workerName + "-77ac3"))
		})

		It("should return an error for unknown versions", func() {
			for _, version := range []int{0, 3} {
				_, err := CalculateKeyForVersion(version, semver.MustParse(kubernetesVersion), nil, nil, nil)
				Expect(err).NotTo(Succeed())
			}
		})
	})

	Describe("#KeyV2", func() {
		var (
			kubernetesVersion *semver.Version
			values            *Values
			p                 *gardencorev1beta1.Worker
			kubeletConfig     *gardencorev1beta1.KubeletConfig

			hash                        string
			lastCARotationInitiation    = metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
			lastSAKeyRotationInitiation = metav1.Time{Time: time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC)}
		)

		BeforeEach(func() {
			p = &gardencorev1beta1.Worker{
				Name: "test-worker",
				Machine: gardencorev1beta1.Machine{
					Type: "foo",
					Image: &gardencorev1beta1.ShootMachineImage{
						Name:    "bar",
						Version: ptr.To("baz"),
					},
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: []byte("foo"),
				},
				Volume: &gardencorev1beta1.Volume{
					Type:       ptr.To("fast"),
					VolumeSize: "20Gi",
				},
			}
			kubernetesVersion = semver.MustParse("1.2.3")
			values = &Values{
				CredentialsRotationStatus: &gardencorev1beta1.ShootCredentialsRotation{
					CertificateAuthorities: &gardencorev1beta1.CARotation{
						LastInitiationTime: &lastCARotationInitiation,
					},
					ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
						LastInitiationTime: &lastSAKeyRotationInitiation,
					},
				},
				OriginalValues: OriginalValues{
					NodeLocalDNSEnabled: false,
				},
			}
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

			var err error
			hash, err = CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("hash value should not change", func() {
			AfterEach(func() {
				actual, err := CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(hash))
			})

			Context("rolling update", func() {
				It("when changing minimum", func() {
					p.Minimum = 1
				})

				It("when changing maximum", func() {
					p.Maximum = 2
				})

				It("when changing max surge", func() {
					p.MaxSurge = &intstr.IntOrString{StrVal: "new-val"}
				})

				It("when changing max unavailable", func() {
					p.MaxUnavailable = &intstr.IntOrString{StrVal: "new-val"}
				})

				It("when changing annotations", func() {
					p.Annotations = map[string]string{"foo": "bar"}
				})

				It("when changing labels", func() {
					p.Labels = map[string]string{"foo": "bar"}
				})

				It("when changing taints", func() {
					p.Taints = []corev1.Taint{{Key: "foo"}}
				})

				It("when changing zones", func() {
					p.Zones = []string{"1"}
				})

				It("when changing provider config", func() {
					// must not be interpreted by the operating system config
					p.ProviderConfig.Raw = nil
				})

				It("when changing the kubernetes version in the worker object", func() {
					// must use kubernetesVersion instead
					p.Kubernetes = &gardencorev1beta1.WorkerKubernetes{
						Version: ptr.To("12.34.5"),
					}
				})

				It("when changing the combined kubernetes patch version", func() {
					kubernetesVersion = semver.MustParse("1.2.4")
				})

				It("when systemReserved is empty", func() {
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{}
				})

				It("when systemReserved has zero value for CPU", func() {
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						CPU: ptr.To(resource.MustParse("0")),
					}
				})

				It("when systemReserved has zero value for memory", func() {
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						Memory: ptr.To(resource.MustParse("0")),
					}
				})

				It("when systemReserved has zero value for PID", func() {
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						PID: ptr.To(resource.MustParse("0")),
					}
				})

				It("when systemReserved has zero value for EphemeralStorage", func() {
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						EphemeralStorage: ptr.To(resource.MustParse("0")),
					}
				})

				It("when moving CPU between kubeReserved and systemReserved", func() {
					kubeletConfig.KubeReserved.CPU = ptr.To(resource.MustParse("70m"))
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						CPU: ptr.To(resource.MustParse("10m")),
					}
				})

				It("when moving memory between kubeReserved and systemReserved", func() {
					kubeletConfig.KubeReserved.Memory = ptr.To(resource.MustParse("896Mi"))
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						Memory: ptr.To(resource.MustParse("128Mi")),
					}
				})

				It("when moving PID between kubeReserved and systemReserved", func() {
					kubeletConfig.KubeReserved.PID = ptr.To(resource.MustParse("9k"))
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						PID: ptr.To(resource.MustParse("1000")),
					}
				})

				It("when moving EphemeralStorage between kubeReserved and systemReserved", func() {
					kubeletConfig.KubeReserved.EphemeralStorage = ptr.To(resource.MustParse("18Gi"))
					kubeletConfig.SystemReserved = &gardencorev1beta1.KubeletConfigReserved{
						EphemeralStorage: ptr.To(resource.MustParse("2048Mi")),
					}
				})

				It("when specifying kubeReserved with different quantities", func() {
					kubeletConfig.KubeReserved = &gardencorev1beta1.KubeletConfigReserved{
						CPU:              ptr.To(resource.MustParse("80m")),
						Memory:           ptr.To(resource.MustParse("1024Mi")),
						PID:              ptr.To(resource.MustParse("10000")),
						EphemeralStorage: ptr.To(resource.MustParse("20480Mi")),
					}
				})

				It("when a shoot CA rotation is triggered", func() {
					newRotationTime := metav1.Time{Time: lastCARotationInitiation.Add(time.Hour)}
					values.CredentialsRotationStatus.CertificateAuthorities.LastInitiationTime = &newRotationTime
					values.CredentialsRotationStatus.CertificateAuthorities.PendingWorkersRollouts = []gardencorev1beta1.PendingWorkersRollout{{
						Name:               p.Name,
						LastInitiationTime: &lastCARotationInitiation,
					}}
				})

				It("when a shoot service account key rotation is triggered", func() {
					newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
					values.CredentialsRotationStatus.ServiceAccountKey.LastInitiationTime = &newRotationTime
					values.CredentialsRotationStatus.ServiceAccountKey.PendingWorkersRollouts = []gardencorev1beta1.PendingWorkersRollout{{
						Name:               p.Name,
						LastInitiationTime: &lastSAKeyRotationInitiation,
					}}
				})
			})

			Context("in-place update", func() {
				BeforeEach(func() {
					p.UpdateStrategy = ptr.To(gardencorev1beta1.AutoInPlaceUpdate)

					var err error
					hash, err = CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
					Expect(err).ToNot(HaveOccurred())
				})

				It("when changing machine image version", func() {
					p.Machine.Image.Version = ptr.To("new-version")
				})

				It("when changing the kubernetes major/minor version of the worker pool version", func() {
					kubernetesVersion = semver.MustParse("1.3.3")
				})

				It("when a shoot CA rotation is triggered", func() {
					newRotationTime := metav1.Time{Time: lastCARotationInitiation.Add(time.Hour)}
					values.CredentialsRotationStatus.CertificateAuthorities.LastInitiationTime = &newRotationTime
				})

				It("when a shoot CA rotation is triggered for the first time (lastInitiationTime was nil)", func() {
					var err error
					credentialStatusWithInitiatedRotation := values.CredentialsRotationStatus.CertificateAuthorities.DeepCopy()
					values.CredentialsRotationStatus.CertificateAuthorities = nil
					hash, err = CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
					Expect(err).ToNot(HaveOccurred())

					values.CredentialsRotationStatus.CertificateAuthorities = credentialStatusWithInitiatedRotation
				})

				It("when a shoot service account key rotation is triggered", func() {
					newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
					values.CredentialsRotationStatus.ServiceAccountKey.LastInitiationTime = &newRotationTime
				})

				It("when a shoot service account key rotation is triggered for the first time (lastInitiationTime was nil)", func() {
					var err error
					credentialStatusWithInitiatedRotation := values.CredentialsRotationStatus.ServiceAccountKey.DeepCopy()
					values.CredentialsRotationStatus.ServiceAccountKey = nil
					hash, err = CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
					Expect(err).ToNot(HaveOccurred())

					values.CredentialsRotationStatus.ServiceAccountKey = credentialStatusWithInitiatedRotation
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
		})

		Context("hash value should change", func() {
			AfterEach(func() {
				actual, err := CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).NotTo(Equal(hash))
			})

			Context("rolling update", func() {
				It("when changing name", func() {
					p.Name = "different-name"
				})

				It("when changing machine type", func() {
					p.Machine.Type = "small"
				})

				It("when changing machine image name", func() {
					p.Machine.Image.Name = "new-image"
				})

				It("when changing machine image version", func() {
					p.Machine.Image.Version = ptr.To("new-version")
				})

				It("when changing volume type", func() {
					t := "xl"
					p.Volume.Type = &t
				})

				It("when changing volume size", func() {
					p.Volume.VolumeSize = "100Mi"
				})

				It("when changing the kubernetes major/minor version of the worker pool version", func() {
					kubernetesVersion = semver.MustParse("1.3.3")
				})

				It("when changing the CRI configurations", func() {
					p.CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}
				})

				It("when a shoot CA rotation is triggered", func() {
					newRotationTime := metav1.Time{Time: lastCARotationInitiation.Add(time.Hour)}
					values.CredentialsRotationStatus.CertificateAuthorities.LastInitiationTime = &newRotationTime
				})

				It("when a shoot CA rotation is triggered for the first time (lastInitiationTime was nil)", func() {
					var err error
					credentialStatusWithInitiatedRotation := values.CredentialsRotationStatus.CertificateAuthorities.DeepCopy()
					values.CredentialsRotationStatus.CertificateAuthorities = nil
					hash, err = CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
					Expect(err).ToNot(HaveOccurred())

					values.CredentialsRotationStatus.CertificateAuthorities = credentialStatusWithInitiatedRotation
				})

				It("when a shoot service account key rotation is triggered", func() {
					newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
					values.CredentialsRotationStatus.ServiceAccountKey.LastInitiationTime = &newRotationTime
				})

				It("when a shoot service account key rotation is triggered for the first time (lastInitiationTime was nil)", func() {
					var err error
					credentialStatusWithInitiatedRotation := values.CredentialsRotationStatus.ServiceAccountKey.DeepCopy()
					values.CredentialsRotationStatus.ServiceAccountKey = nil
					hash, err = CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
					Expect(err).ToNot(HaveOccurred())

					values.CredentialsRotationStatus.ServiceAccountKey = credentialStatusWithInitiatedRotation
				})

				It("when enabling node local dns via specification", func() {
					values.NodeLocalDNSEnabled = true
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

			Context("in-place update", func() {
				BeforeEach(func() {
					p.UpdateStrategy = ptr.To(gardencorev1beta1.AutoInPlaceUpdate)

					var err error
					hash, err = CalculateKeyForVersion(2, kubernetesVersion, values, p, kubeletConfig)
					Expect(err).ToNot(HaveOccurred())
				})

				It("when changing name", func() {
					p.Name = "different-name"
				})

				It("when changing machine type", func() {
					p.Machine.Type = "small"
				})

				It("when changing machine image name", func() {
					p.Machine.Image.Name = "new-image"
				})

				It("when changing volume type", func() {
					t := "xl"
					p.Volume.Type = &t
				})

				It("when changing volume size", func() {
					p.Volume.VolumeSize = "100Mi"
				})

				It("when changing the CRI configurations", func() {
					p.CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}
				})

				It("when enabling node local dns via specification", func() {
					values.NodeLocalDNSEnabled = true
				})
			})
		})
	})
})
