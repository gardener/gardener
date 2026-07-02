// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/gardeneruser"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/sshdensurer"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
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
			c                client.Client
			fakeClient       client.Client
			sm               secretsmanager.Interface
			defaultDepWaiter Interface

			ctx     context.Context
			values  *Values
			log     logr.Logger
			fakeErr = fmt.Errorf("some random error")

			fakeClock *testclock.FakeClock
			now       time.Time

			poolHashesSecret    *corev1.Secret
			apiServerURL        = "https://url-to-apiserver"
			caBundle            = "ca-bundle"
			clusterDNSAddresses = []string{"cluster-dns", "backup-cluster-dns"}
			clusterDomain       = "cluster-domain"
			images              = map[string]*imagevector.Image{
				"gardener-node-agent": {},
				"pause-container":     {Repository: new("registry.k8s.io/pause"), Tag: new("latest")},
			}
			evictionHardMemoryAvailable = "100Mi"
			kubeletConfig               = &gardencorev1beta1.KubeletConfig{
				EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
					MemoryAvailable: &evictionHardMemoryAvailable,
				},
			}
			kubeletDataVolumeName                   = "foo"
			machineTypes                            []gardencorev1beta1.MachineType
			sshPublicKeys                           = []string{"ssh-public-key", "ssh-public-key-b"}
			kubernetesVersion                       = semver.MustParse("1.2.3")
			workerKubernetesVersion                 = "4.5.6"
			valitailEnabled                         = false
			openTelemetryCollectorLogShipperEnabled = false

			//nolint:unparam
			initConfigFn = func(worker gardencorev1beta1.Worker, nodeAgentImage string, config *nodeagentconfigv1alpha1.NodeAgentConfiguration, clusterCABundle []byte) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
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
						{Name: cctx.CABundle},
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
						{Path: strconv.FormatBool(cctx.OpenTelemetryCollectorLogShipperEnabled)},
						{Path: fmt.Sprintf("%+v", cctx.Taints)},
					},
					nil
			}

			workers              []gardencorev1beta1.Worker
			inPlaceUpdateWorkers []gardencorev1beta1.Worker

			expected []*extensionsv1alpha1.OperatingSystemConfig

			globalLastInitiationTime = &metav1.Time{Time: time.Date(2020, 12, 2, 10, 0, 0, 0, time.UTC)}
		)

		computeExpectedOperatingSystemConfigs := func(sshAccessEnabled bool, workers []gardencorev1beta1.Worker, inPlaceUpdate bool) []*extensionsv1alpha1.OperatingSystemConfig {
			w := workers

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
						CgroupDriver: new(extensionsv1alpha1.CgroupDriverSystemd),
					}

					criConfigProvisioning = &extensionsv1alpha1.CRIConfig{
						Name: extensionsv1alpha1.CRIName(worker.CRI.Name),
					}
				}

				if worker.CABundle != nil {
					caBundle = fmt.Sprintf("%s\n%s", caBundle, *worker.CABundle)
				}

				key := Key(k8sVersion, values.CredentialsRotationStatus, &worker, values.NodeLocalDNSEnabled, kubeletConfig, nil)
				if inPlaceUpdate {
					key = fmt.Sprintf("gardener-node-agent-%s", worker.Name)
				}

				imagesCopy := make(map[string]*imagevector.Image, len(images))
				maps.Copy(imagesCopy, images)
				imagesCopy["hyperkube"] = &imagevector.Image{Repository: new("europe-docker.pkg.dev/gardener-project/releases/hyperkube"), Tag: new("v" + k8sVersion.String())}

				apiServerURLForWorker := apiServerURL
				if worker.ControlPlane != nil {
					apiServerURLForWorker = "https://localhost:443"
				}

				initUnits, initFiles, _ := initConfigFn(
					worker,
					imagesCopy["gardener-node-agent"].String(),
					&nodeagentconfigv1alpha1.NodeAgentConfiguration{APIServer: nodeagentconfigv1alpha1.APIServer{
						Server: apiServerURLForWorker,
						CAFile: nodeagentconfigv1alpha1.ClusterCAFilePath,
					}},
					[]byte(caBundle),
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
					KubeletDataVolumeName:                   &kubeletDataVolumeName,
					KubernetesVersion:                       k8sVersion,
					SSHAccessEnabled:                        true,
					SSHPublicKeys:                           sshPublicKeys,
					ValitailEnabled:                         valitailEnabled,
					OpenTelemetryCollectorLogShipperEnabled: openTelemetryCollectorLogShipperEnabled,
				}

				if worker.ControlPlane != nil {
					componentsContext.KubeletConfigParameters.WithStaticPodPath = true
					componentsContext.Taints = append(componentsContext.Taints, corev1.Taint{
						Key:    "node-role.kubernetes.io/control-plane",
						Effect: corev1.TaintEffectNoSchedule,
					})
				}

				originalUnits, originalFiles, _ := originalConfigFn(componentsContext)

				name := key + "-init"

				oscInit := &extensionsv1alpha1.OperatingSystemConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
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

				name = key + "-original"

				oscOriginal := &extensionsv1alpha1.OperatingSystemConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
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
			now = time.Unix(60, 0)
			fakeClock = testclock.NewFakeClock(now)

			ctx = context.TODO()
			log = logr.Discard()

			s := runtime.NewScheme()
			Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())
			Expect(fakekubernetes.AddToScheme(s)).To(Succeed())
			Expect(machinev1alpha1.AddToScheme(s)).To(Succeed())
			c = fakeclient.NewClientBuilder().WithScheme(s).WithStatusSubresource(&extensionsv1alpha1.OperatingSystemConfig{}).Build()

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			sm = fakesecretsmanager.New(fakeClient, namespace)

			By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-kubelet", Namespace: namespace}})).To(Succeed())

			workers = []gardencorev1beta1.Worker{
				{
					Name: worker1Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: new(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:           "type1",
							Version:        new("12.34"),
							ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					ControlPlane:          &gardencorev1beta1.WorkerControlPlane{},
				},
				{
					Name: worker2Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: new(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "type2",
							Version: new("12.34"),
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
						Architecture: new(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:           "type1",
							Version:        new("12.34"),
							ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
					Kubernetes: &gardencorev1beta1.WorkerKubernetes{
						Kubelet: &gardencorev1beta1.KubeletConfig{
							KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
								CPU:    new(resource.MustParse("100m")),
								Memory: new(resource.MustParse("100Mi")),
							},
						},
					},

					UpdateStrategy: new(gardencorev1beta1.AutoInPlaceUpdate),
				},
				{
					Name: inPlaceWorkerName2,
					Machine: gardencorev1beta1.Machine{
						Architecture: new(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "type2",
							Version: new("12.34"),
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
					UpdateStrategy: new(gardencorev1beta1.ManualInPlaceUpdate),
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
					CABundle:                                caBundle,
					ClusterDNSAddresses:                     clusterDNSAddresses,
					ClusterDomain:                           clusterDomain,
					Images:                                  images,
					KubeletConfig:                           kubeletConfig,
					MachineTypes:                            machineTypes,
					SSHPublicKeys:                           sshPublicKeys,
					ValitailEnabled:                         valitailEnabled,
					OpenTelemetryCollectorLogShipperEnabled: openTelemetryCollectorLogShipperEnabled,
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
      currentVersion: 2
      hashVersionToOSCKey:
        2: gardener-node-agent-worker1-4692a28d44cc6a0c
    - name: worker2
      currentVersion: 2
      hashVersionToOSCKey:
        2: gardener-node-agent-worker2-2ad2eaf0b80f61a3
`)},
			}

			expected = computeExpectedOperatingSystemConfigs(false, workers, false)
			defaultDepWaiter = New(log, c, sm, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
		})

		AfterEach(func() {
		})

		Describe("#Deploy", func() {
			It("should successfully deploy all extensions resources", func() {
				DeferCleanup(test.WithVars(
					&TimeNow, fakeClock.Now,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
				))

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range computeExpectedOperatingSystemConfigs(false, workers, false) {
					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

					obj := e.DeepCopy()
					obj.ResourceVersion = "1"

					Expect(actual).To(Equal(obj))
				}
			})

			It("should successfully deploy all extensions resources and SSH access is enabled", func() {
				DeferCleanup(test.WithVars(
					&TimeNow, fakeClock.Now,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
					&values.SSHAccessEnabled, true,
				))

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range computeExpectedOperatingSystemConfigs(true, workers, false) {
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
							CABundle:                                caBundle,
							ClusterDNSAddresses:                     clusterDNSAddresses,
							ClusterDomain:                           clusterDomain,
							Images:                                  images,
							KubeletConfig:                           kubeletConfig,
							MachineTypes:                            machineTypes,
							SSHPublicKeys:                           sshPublicKeys,
							ValitailEnabled:                         valitailEnabled,
							OpenTelemetryCollectorLogShipperEnabled: openTelemetryCollectorLogShipperEnabled,
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
						&TimeNow, fakeClock.Now,
						&InitConfigFn, initConfigFn,
						&OriginalConfigFn, originalConfigFn,
						&values.SSHAccessEnabled, true,
						&format.MaxLength, 0,
					))

					Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

					for _, e := range computeExpectedOperatingSystemConfigs(true, inPlaceUpdateWorkers, true) {
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
				initConfigFnWithBootstrapToken := func(worker gardencorev1beta1.Worker, nodeAgentImage string, config *nodeagentconfigv1alpha1.NodeAgentConfiguration, clusterCABundle []byte) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
					units, files, err := initConfigFn(worker, nodeAgentImage, config, clusterCABundle)
					return units, append(files, bootstrapTokenFile), err
				}

				defer test.WithVars(
					&TimeNow, fakeClock.Now,
					&InitConfigFn, initConfigFnWithBootstrapToken,
					&OriginalConfigFn, originalConfigFn,
				)()

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

			Context("CA bundle computation", func() {
				var getROOTcertsFileContent = func(osc *extensionsv1alpha1.OperatingSystemConfig) *string {
					for _, f := range osc.Spec.Files {
						if f.Path == "/var/lib/ca-certificates-local/ROOTcerts.crt" {
							content, err := utils.DecodeBase64(f.Content.Inline.Data)
							ExpectWithOffset(1, err).ToNot(HaveOccurred())
							return new(string(content))
						}
					}
					return nil
				}

				JustBeforeEach(func() {
					Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())
				})

				When("worker CA bundle is not set", func() {
					It("should contain only the provided CA bundle", func() {
						for _, e := range expected {
							if e.Spec.Purpose != extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
								continue
							}

							actual := &extensionsv1alpha1.OperatingSystemConfig{}
							Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

							content := getROOTcertsFileContent(actual)
							Expect(content).NotTo(BeNil())
							Expect(*content).To(Equal(caBundle))
						}
					})
				})

				When("worker CA bundle is set", func() {
					BeforeEach(func() {
						values.Workers[0].CABundle = new("foo")
						values.Workers[1].CABundle = new("bar")
					})

					It("should append worker CA bundle to the CA bundle", func() {
						for _, e := range expected {
							if e.Spec.Purpose != extensionsv1alpha1.OperatingSystemConfigPurposeReconcile {
								continue
							}

							actual := &extensionsv1alpha1.OperatingSystemConfig{}
							Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())
							worker := actual.Labels["worker.gardener.cloud/pool"]

							content := getROOTcertsFileContent(actual)
							Expect(content).NotTo(BeNil())
							if worker == "worker1" {
								Expect(*content).To(Equal(fmt.Sprintf("%s\n%s", caBundle, "foo")))
							} else {
								Expect(*content).To(Equal(fmt.Sprintf("%s\n%s", caBundle, "bar")))
							}
						}
					})
				})
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
					key := Key(k8sVersion, values.CredentialsRotationStatus, &worker, values.NodeLocalDNSEnabled, kubeletConfig, nil)

					extensions = append(extensions,
						gardencorev1beta1.ExtensionResourceState{
							Name:    new(key + "-init"),
							Kind:    extensionsv1alpha1.OperatingSystemConfigResource,
							Purpose: new(string(extensionsv1alpha1.OperatingSystemConfigPurposeProvision)),
							State:   &runtime.RawExtension{Raw: stateInit},
						},
						gardencorev1beta1.ExtensionResourceState{
							Name:    new(key + "-original"),
							Kind:    extensionsv1alpha1.OperatingSystemConfigResource,
							Purpose: new(string(extensionsv1alpha1.OperatingSystemConfigPurposeReconcile)),
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

			It("should properly restore the extensions state if it exists", func() {
				defer test.WithVars(
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
					&TimeNow, fakeClock.Now,
					&extensions.TimeNow, fakeClock.Now,
				)()

				// Pre-create the poolHashesSecret
				Expect(c.Create(ctx, poolHashesSecret)).To(Succeed())

				defaultDepWaiter = New(log, c, sm, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
				Expect(defaultDepWaiter.Restore(ctx, shootState)).To(Succeed())

				for i := range expected {
					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expected[i]), actual)).To(Succeed())
					Expect(actual.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "restore"))
					var expectedState []byte
					if strings.HasSuffix(expected[i].Name, "init") {
						expectedState = stateInit
					} else {
						expectedState = stateOriginal
					}
					Expect(actual.Status.State).To(Equal(&runtime.RawExtension{Raw: expectedState}))
				}
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
				defer test.WithVars(&TimeNow, fakeClock.Now)()

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
					&TimeNow, fakeClock.Now,
				)()

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
					&TimeNow, fakeClock.Now,
					&OriginalConfigFn, originalConfigFn,
				)()

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
					&TimeNow, fakeClock.Now,
					&OriginalConfigFn, originalConfigFn,
				)()

				By("Deploy")
				// Deploy should fill internal state with the added timestamp annotation
				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				By("Patch object")
				for i := range expected {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expected[i]), expected[i])).To(Succeed())

					// remove operation annotation, add up-to-date timestamp annotation
					expected[i].Annotations = map[string]string{
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					}
					Expect(c.Update(ctx, expected[i])).To(Succeed(), "patching operatingsystemconfig succeeds")

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
					Expect(c.Status().Update(ctx, expected[i])).To(Succeed(), "patching operatingsystemconfig status succeeds")

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
					&TimeNow, fakeClock.Now,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
				))

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
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-4692a28d44cc6a0c",
							Object:                      worker1OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-4692a28d44cc6a0c",
							Object:                      worker1OSCOriginal,
						},
					},
					worker2Name: {
						Init: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-2ad2eaf0b80f61a3",
							Object:                      worker2OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-2ad2eaf0b80f61a3",
							Object:                      worker2OSCOriginal,
						},
					},
				}))

				for i := range expected {
					// remove operation annotation
					expected[i].Annotations = map[string]string{
						"gardener.cloud/timestamp": now.UTC().Format(time.RFC3339Nano),
					}
					Expect(c.Update(ctx, expected[i])).To(Succeed())

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
					Expect(c.Status().Update(ctx, expected[i])).To(Succeed())

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
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-4692a28d44cc6a0c",
							SecretName:                  new("cc-" + expected[0].Name),
							Object:                      worker1OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker1Name + "-4692a28d44cc6a0c",
							Object:                      worker1OSCOriginal,
						},
					},
					worker2Name: {
						Init: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-2ad2eaf0b80f61a3",
							SecretName:                  new("cc-" + expected[2].Name),
							Object:                      worker2OSCDownloader,
						},
						Original: Data{
							GardenerNodeAgentSecretName: "gardener-node-agent-" + worker2Name + "-2ad2eaf0b80f61a3",
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
					&extensions.TimeNow, fakeClock.Now,
					&gardenerutils.TimeNow, fakeClock.Now,
				)()

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

				cWithErr := fakeclient.NewClientBuilder().WithScheme(c.Scheme()).WithObjects(&expectedOSC).WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						if _, ok := obj.(*extensionsv1alpha1.OperatingSystemConfig); ok {
							return fakeErr
						}
						return cl.Delete(ctx, obj, opts...)
					},
				}).Build()

				defaultDepWaiter = New(log, cWithErr, nil, &Values{Namespace: namespace}, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
				Expect(defaultDepWaiter.Destroy(ctx)).To(MatchError(error(multierror.Append(fakeErr))))
			})
		})

		Describe("#WaitCleanup", func() {
			It("should not return error if all resources are gone", func() {
				Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
			})

			It("should return error if resources still exist", func() {
				Expect(c.Create(ctx, expected[0])).To(Succeed())
				Expect(defaultDepWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring("OperatingSystemConfig test-namespace/gardener-node-agent-worker1-4692a28d44cc6a0c-init is still present")))
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

	Describe("#Key", func() {
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
						Version: new("baz"),
					},
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: []byte("foo"),
				},
				Volume: &gardencorev1beta1.Volume{
					Type:       new("fast"),
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
					CPU:              new(resource.MustParse("80m")),
					Memory:           new(resource.MustParse("1Gi")),
					PID:              new(resource.MustParse("10k")),
					EphemeralStorage: new(resource.MustParse("20Gi")),
				},
				EvictionHard: &gardencorev1beta1.KubeletConfigEviction{
					MemoryAvailable: new("100Mi"),
				},
				CPUManagerPolicy: nil,
			}

			kubeProxyConfig := &gardencorev1beta1.KubeProxyConfig{
				Mode:    new(gardencorev1beta1.ProxyModeIPTables),
				Enabled: new(false),
			}

			hash = Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, kubeProxyConfig)
		})

		It("should handle an empty machine image version", func() {
			p.Machine.Image.Version = nil
			Expect(Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)).NotTo(BeEmpty())
		})

		Context("hash value should not change", func() {
			AfterEach(func() {
				kubeProxyConfig := &gardencorev1beta1.KubeProxyConfig{
					Mode:    new(gardencorev1beta1.ProxyModeIPTables),
					Enabled: new(false),
				}
				actual := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, kubeProxyConfig)
				Expect(actual).To(Equal(hash))
			})

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
					Version: new("12.34.5"),
				}
			})

			It("when changing the combined kubernetes patch version", func() {
				kubernetesVersion = semver.MustParse("1.2.4")
			})

			It("when specifying kubeReserved with different quantities", func() {
				kubeletConfig.KubeReserved = &gardencorev1beta1.KubeletConfigReserved{
					CPU:              new(resource.MustParse("80m")),
					Memory:           new(resource.MustParse("1024Mi")),
					PID:              new(resource.MustParse("10000")),
					EphemeralStorage: new(resource.MustParse("20480Mi")),
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

		Context("hash value should change", func() {
			AfterEach(func() {
				actual := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)
				Expect(actual).NotTo(Equal(hash))
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

			It("when changing machine image version", func() {
				p.Machine.Image.Version = new("new-version")
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
				credentialStatusWithInitiatedRotation := values.CredentialsRotationStatus.CertificateAuthorities.DeepCopy()
				values.CredentialsRotationStatus.CertificateAuthorities = nil
				hash = Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)

				values.CredentialsRotationStatus.CertificateAuthorities = credentialStatusWithInitiatedRotation
			})

			It("when a shoot service account key rotation is triggered", func() {
				newRotationTime := metav1.Time{Time: lastSAKeyRotationInitiation.Add(time.Hour)}
				values.CredentialsRotationStatus.ServiceAccountKey.LastInitiationTime = &newRotationTime
			})

			It("when a shoot service account key rotation is triggered for the first time (lastInitiationTime was nil)", func() {
				credentialStatusWithInitiatedRotation := values.CredentialsRotationStatus.ServiceAccountKey.DeepCopy()
				values.CredentialsRotationStatus.ServiceAccountKey = nil
				hash = Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)

				values.CredentialsRotationStatus.ServiceAccountKey = credentialStatusWithInitiatedRotation
			})

			It("when enabling node local dns via specification", func() {
				values.NodeLocalDNSEnabled = true
			})

			It("when changing kubeReserved CPU", func() {
				kubeletConfig.KubeReserved.CPU = new(resource.MustParse("100m"))
			})

			It("when changing kubeReserved memory", func() {
				kubeletConfig.KubeReserved.Memory = new(resource.MustParse("2Gi"))
			})

			It("when changing kubeReserved PID", func() {
				kubeletConfig.KubeReserved.PID = new(resource.MustParse("15k"))
			})

			It("when changing kubeReserved ephemeral storage", func() {
				kubeletConfig.KubeReserved.EphemeralStorage = new(resource.MustParse("42Gi"))
			})

			It("when changing evictionHard memory threshold", func() {
				kubeletConfig.EvictionHard.MemoryAvailable = new("200Mi")
			})

			It("when changing evictionHard image fs threshold", func() {
				kubeletConfig.EvictionHard.ImageFSAvailable = new("200Mi")
			})

			It("when changing evictionHard image fs inodes threshold", func() {
				kubeletConfig.EvictionHard.ImageFSInodesFree = new("1k")
			})

			It("when changing evictionHard node fs threshold", func() {
				kubeletConfig.EvictionHard.NodeFSAvailable = new("200Mi")
			})

			It("when changing evictionHard node fs inodes threshold", func() {
				kubeletConfig.EvictionHard.NodeFSInodesFree = new("1k")
			})

			It("when changing CPUManagerPolicy", func() {
				kubeletConfig.CPUManagerPolicy = new("test")
			})

			It("when node-local-dns gets enabled and kubernetes version is equal or larger than 1.34", func() {
				values.NodeLocalDNSEnabled = false
				kubernetesVersion = semver.MustParse("1.34")
				hash1 := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)
				values.NodeLocalDNSEnabled = true
				hash2 := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)
				Expect(hash1).To(Equal(hash2))
			})

			It("when node-local-dns gets disabled and kube-proxy runs in ipvs mode", func() {
				values.NodeLocalDNSEnabled = true
				kubernetesVersion = semver.MustParse("1.34")
				kubeProxyConfig := &gardencorev1beta1.KubeProxyConfig{
					Mode:    new(gardencorev1beta1.ProxyModeIPVS),
					Enabled: new(true),
				}
				hash1 := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, kubeProxyConfig)
				values.NodeLocalDNSEnabled = false
				hash2 := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, kubeProxyConfig)
				Expect(hash1).ToNot(Equal(hash2))
			})

			It("when node-local-dns gets enabled and kubernetes version is lower than 1.34", func() {
				values.NodeLocalDNSEnabled = false
				kubernetesVersion = semver.MustParse("1.31")
				hash1 := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)
				values.NodeLocalDNSEnabled = true
				hash2 := Key(kubernetesVersion, values.CredentialsRotationStatus, p, values.NodeLocalDNSEnabled, kubeletConfig, nil)
				Expect(hash1).ToNot(Equal(hash2))
			})
		})
	})
})
