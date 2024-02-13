// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
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
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("OperatingSystemConfig", func() {
	Describe("> Interface", func() {
		const namespace = "test-namespace"

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

			apiServerURL                = "https://url-to-apiserver"
			caBundle                    = ptr.To("ca-bundle")
			clusterCASecretName         = "ca"
			clusterDNSAddress           = "cluster-dns"
			clusterDomain               = "cluster-domain"
			images                      = map[string]*imagevector.Image{"gardener-node-agent": {}}
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

			ccdUnitContent = "ccd-unit-content"

			//nolint:unparam
			initConfigFn = func(worker gardencorev1beta1.Worker, nodeAgentImage string, config *nodeagentv1alpha1.NodeAgentConfiguration) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
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
			downloaderConfigFn = func(cloudConfigUserDataSecretName, apiServerURL, clusterCASecretName string) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
				return []extensionsv1alpha1.Unit{
						{Name: cloudConfigUserDataSecretName},
						{
							Name:    "cloud-config-downloader.service",
							Content: &ccdUnitContent,
						},
					},
					[]extensionsv1alpha1.File{
						{Path: apiServerURL},
						{Path: clusterCASecretName},
					},
					nil
			}
			//nolint:unparam
			originalConfigFn = func(cctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
				return []extensionsv1alpha1.Unit{
						{Name: cctx.Key},
						{Name: *cctx.CABundle},
						{Name: cctx.ClusterDNSAddress},
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
					},
					nil
			}

			worker1Name = "worker1"
			worker2Name = "worker2"
			workers     = []gardencorev1beta1.Worker{
				{
					Name: worker1Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:           "type1",
							ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
						},
					},
					KubeletDataVolumeName: &kubeletDataVolumeName,
				},
				{
					Name: worker2Name,
					Machine: gardencorev1beta1.Machine{
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image: &gardencorev1beta1.ShootMachineImage{
							Name: "type2",
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
			empty    *extensionsv1alpha1.OperatingSystemConfig
			expected []*extensionsv1alpha1.OperatingSystemConfig
		)

		computeExpectedOperatingSystemConfigs := func(useGardenerNodeAgent, sshAccessEnabled bool) []*extensionsv1alpha1.OperatingSystemConfig {
			expected := make([]*extensionsv1alpha1.OperatingSystemConfig, 0, 2*len(workers))
			for _, worker := range workers {
				var (
					criName   = extensionsv1alpha1.CRINameContainerD
					criConfig *extensionsv1alpha1.CRIConfig
				)

				if worker.CRI != nil {
					criName = extensionsv1alpha1.CRIName(worker.CRI.Name)
					criConfig = &extensionsv1alpha1.CRIConfig{Name: extensionsv1alpha1.CRIName(worker.CRI.Name)}
				}

				k8sVersion := values.KubernetesVersion
				if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
					k8sVersion = semver.MustParse(*worker.Kubernetes.Version)
				}

				key := Key(worker.Name, k8sVersion, worker.CRI)

				imagesCopy := make(map[string]*imagevector.Image, len(images))
				for imageName, image := range images {
					imagesCopy[imageName] = image
				}

				if useGardenerNodeAgent {
					imagesCopy["hyperkube"] = &imagevector.Image{Repository: "europe-docker.pkg.dev/gardener-project/releases/hyperkube", Tag: ptr.To("v" + k8sVersion.String())}
				}

				downloaderUnits, downloaderFiles, _ := downloaderConfigFn(
					key,
					apiServerURL,
					clusterCASecretName,
				)
				initUnits, initFiles, _ := initConfigFn(
					worker,
					imagesCopy["gardener-node-agent"].String(),
					&nodeagentv1alpha1.NodeAgentConfiguration{APIServer: nodeagentv1alpha1.APIServer{
						Server:   apiServerURL,
						CABundle: []byte(*caBundle),
					}},
				)
				componentsContext := components.Context{
					Key:               key,
					CABundle:          caBundle,
					ClusterDNSAddress: clusterDNSAddress,
					ClusterDomain:     clusterDomain,
					CRIName:           criName,
					Images:            imagesCopy,
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
				originalUnits, originalFiles, _ := originalConfigFn(componentsContext)

				initSuffix := "-downloader"
				if useGardenerNodeAgent {
					initSuffix = "-init"
				}

				oscInit := &extensionsv1alpha1.OperatingSystemConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      key + "-" + worker.Machine.Image.Name + initSuffix,
						Namespace: namespace,
						Annotations: map[string]string{
							v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
							v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						},
						Labels: map[string]string{
							"worker.gardener.cloud/pool": worker.Name,
						},
					},
					Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type:           worker.Machine.Image.Name,
							ProviderConfig: worker.Machine.Image.ProviderConfig,
						},
						Purpose:   extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
						CRIConfig: criConfig,
					},
				}

				if useGardenerNodeAgent {
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
				} else {
					oscInit.Spec.Units = downloaderUnits
					oscInit.Spec.Files = downloaderFiles
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
							"worker.gardener.cloud/pool": worker.Name,
						},
					},
					Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type:           worker.Machine.Image.Name,
							ProviderConfig: worker.Machine.Image.ProviderConfig,
						},
						Purpose:              extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
						CRIConfig:            criConfig,
						ReloadConfigFilePath: ptr.To("/var/lib/cloud-config-downloader/downloads/cloud_config"),
						Units:                originalUnits,
						Files:                originalFiles,
					},
				}

				if useGardenerNodeAgent {
					oscOriginal.Spec.Units = append(oscOriginal.Spec.Units, initUnits...)
					oscOriginal.Spec.Files = append(oscOriginal.Spec.Files, extensionsv1alpha1.File{Path: "/var/lib/gardener-node-agent/init.sh"})
				} else {
					oscOriginal.Spec.Files = append(append(oscOriginal.Spec.Files, downloaderFiles...), extensionsv1alpha1.File{
						Path:        "/etc/systemd/system/cloud-config-downloader.service",
						Permissions: ptr.To(int32(0644)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data:     utils.EncodeBase64([]byte(ccdUnitContent)),
							},
						},
					})
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
			Expect(kubernetesfake.AddToScheme(s)).To(Succeed())
			c = fakeclient.NewClientBuilder().WithScheme(s).Build()

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
			sm = fakesecretsmanager.New(fakeClient, namespace)

			By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-kubelet", Namespace: namespace}})).To(Succeed())

			values = &Values{
				Namespace:         namespace,
				Workers:           workers,
				KubernetesVersion: kubernetesVersion,
				InitValues: InitValues{
					APIServerURL: apiServerURL,
				},
				OriginalValues: OriginalValues{
					CABundle:          caBundle,
					ClusterDNSAddress: clusterDNSAddress,
					ClusterDomain:     clusterDomain,
					Images:            images,
					KubeletConfig:     kubeletConfig,
					MachineTypes:      machineTypes,
					SSHPublicKeys:     sshPublicKeys,
					ValitailEnabled:   valitailEnabled,
				},
			}

			empty = &extensionsv1alpha1.OperatingSystemConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
				},
			}

			expected = computeExpectedOperatingSystemConfigs(true, false)
			defaultDepWaiter = New(log, c, sm, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#Deploy", func() {
			It("should successfully deploy the shoot access secret for the cloud config downloader", func() {
				defer test.WithVars(
					&DownloaderConfigFn, downloaderConfigFn,
					&OriginalConfigFn, originalConfigFn,
				)()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Name: "shoot-access-cloud-config-downloader", Namespace: namespace}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				}))
				Expect(secret.Annotations).To(Equal(map[string]string{
					"serviceaccount.resources.gardener.cloud/name":                      "cloud-config-downloader",
					"serviceaccount.resources.gardener.cloud/namespace":                 "kube-system",
					"serviceaccount.resources.gardener.cloud/token-expiration-duration": "720h",
					"token-requestor.resources.gardener.cloud/target-secret-name":       "cloud-config-downloader",
					"token-requestor.resources.gardener.cloud/target-secret-namespace":  "kube-system",
				}))
			})

			It("should successfully deploy the shoot access secret for the gardener-node-agent", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, true))
				DeferCleanup(test.WithVars(
					&DownloaderConfigFn, downloaderConfigFn,
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

			It("should successfully deploy all extensions resources when UseGardenerNodeAgent feature gate is off", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, false))
				DeferCleanup(test.WithVars(
					&TimeNow, mockNow.Do,
					&DownloaderConfigFn, downloaderConfigFn,
					&OriginalConfigFn, originalConfigFn,
				))

				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range computeExpectedOperatingSystemConfigs(false, false) {
					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

					obj := e.DeepCopy()
					obj.TypeMeta.APIVersion = extensionsv1alpha1.SchemeGroupVersion.String()
					obj.TypeMeta.Kind = extensionsv1alpha1.OperatingSystemConfigResource
					obj.ResourceVersion = "1"

					Expect(actual).To(Equal(obj))
				}
			})

			It("should successfully deploy all extensions resources when UseGardenerNodeAgent feature gate is on", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, true))
				DeferCleanup(test.WithVars(
					&TimeNow, mockNow.Do,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
				))

				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range computeExpectedOperatingSystemConfigs(true, false) {
					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

					obj := e.DeepCopy()
					obj.TypeMeta.APIVersion = extensionsv1alpha1.SchemeGroupVersion.String()
					obj.TypeMeta.Kind = extensionsv1alpha1.OperatingSystemConfigResource
					obj.ResourceVersion = "1"

					Expect(actual).To(Equal(obj))
				}
			})

			It("should successfully deploy all extensions resources when UseGardenerNodeAgent feature gate is on and SSH access is enabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, true))
				DeferCleanup(test.WithVars(
					&TimeNow, mockNow.Do,
					&InitConfigFn, initConfigFn,
					&OriginalConfigFn, originalConfigFn,
					&values.SSHAccessEnabled, true,
				))

				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

				Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

				for _, e := range computeExpectedOperatingSystemConfigs(true, true) {
					actual := &extensionsv1alpha1.OperatingSystemConfig{}
					Expect(c.Get(ctx, client.ObjectKey{Name: e.Name, Namespace: e.Namespace}, actual)).To(Succeed())

					obj := e.DeepCopy()
					obj.TypeMeta.APIVersion = extensionsv1alpha1.SchemeGroupVersion.String()
					obj.TypeMeta.Kind = extensionsv1alpha1.OperatingSystemConfigResource
					obj.ResourceVersion = "1"

					Expect(actual).To(Equal(obj))
				}
			})

			It("should exclude the bootstrap token file if purpose is not provision", func() {
				bootstrapTokenFile := extensionsv1alpha1.File{Path: "/var/lib/gardener-node-agent/credentials/bootstrap-token"}
				initConfigFnWithBootstrapToken := func(worker gardencorev1beta1.Worker, nodeAgentImage string, config *nodeagentv1alpha1.NodeAgentConfiguration) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
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

					obj.TypeMeta.APIVersion = extensionsv1alpha1.SchemeGroupVersion.String()
					obj.TypeMeta.Kind = extensionsv1alpha1.OperatingSystemConfigResource
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
					key := Key(worker.Name, k8sVersion, worker.CRI)

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

			It("should properly restore the extensions state if it exists", func() {
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

				mc.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "shoot-access-cloud-config-downloader"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
				mc.EXPECT().Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-access-cloud-config-downloader",
						Namespace: namespace,
						Annotations: map[string]string{
							"serviceaccount.resources.gardener.cloud/name":                      "cloud-config-downloader",
							"serviceaccount.resources.gardener.cloud/namespace":                 "kube-system",
							"serviceaccount.resources.gardener.cloud/token-expiration-duration": "720h",
							"token-requestor.resources.gardener.cloud/target-secret-name":       "cloud-config-downloader",
							"token-requestor.resources.gardener.cloud/target-secret-namespace":  "kube-system",
						},
						Labels: map[string]string{
							"resources.gardener.cloud/purpose": "token-requestor",
							"resources.gardener.cloud/class":   "shoot",
						},
					},
					Type: corev1.SecretTypeOpaque,
				})

				mc.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "shoot-access-gardener-node-agent"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
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
						DoAndReturn(func(ctx context.Context, actual client.Object, opts ...client.CreateOption) error {
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

				defaultDepWaiter = New(log, mc, sm, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
				Expect(defaultDepWaiter.Restore(ctx, shootState)).To(Succeed())
			})
		})

		Describe("#Wait", func() {
			It("should return error when no resources are found", func() {
				Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should return error when resource is not ready", func() {
				errDescription := "Some error"

				for i := range expected {
					expected[i].Status.LastError = &gardencorev1beta1.LastError{
						Description: errDescription,
					}
					Expect(c.Create(ctx, expected[i])).To(Succeed())
				}

				Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("error during reconciliation: " + errDescription)))
			})

			It("should return error when status does not contain cloud config information", func() {
				for i := range expected {
					// remove operation annotation
					expected[i].ObjectMeta.Annotations = map[string]string{}
					// set last operation
					expected[i].Status.LastOperation = &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					}
					Expect(c.Create(ctx, expected[i])).To(Succeed())
				}

				Expect(defaultDepWaiter.Wait(ctx)).To(MatchError(ContainSubstring("no cloud config information provided in status")))
			})

			It("should return error if we haven't observed the latest timestamp annotation", func() {
				defer test.WithVars(
					&TimeNow, mockNow.Do,
					&DownloaderConfigFn, downloaderConfigFn,
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
					expected[i].ObjectMeta.Annotations = map[string]string{
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
					// set other status fields
					expected[i].Status.Command = ptr.To("foo-" + expected[i].Name)
					expected[i].Status.Units = []string{"bar-" + expected[i].Name, "baz-" + expected[i].Name}
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
					&DownloaderConfigFn, downloaderConfigFn,
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
					expected[i].ObjectMeta.Annotations = map[string]string{
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
					// set other status fields
					expected[i].Status.Command = ptr.To("foo-" + expected[i].Name)
					expected[i].Status.Units = []string{"bar-" + expected[i].Name, "baz-" + expected[i].Name}
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
			It("should return the correct result from the Wait operation", func() {
				for i := range expected {
					// remove operation annotation
					expected[i].ObjectMeta.Annotations = map[string]string{}
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
					// set other status fields
					expected[i].Status.Command = ptr.To("foo-" + expected[i].Name)
					expected[i].Status.Units = []string{"bar-" + expected[i].Name, "baz-" + expected[i].Name}
					Expect(c.Create(ctx, expected[i])).To(Succeed())

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

				worker1OSCDownloader := expected[0]
				worker1OSCDownloader.TypeMeta = metav1.TypeMeta{APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(), Kind: "OperatingSystemConfig"}
				worker1OSCDownloader.Annotations = nil

				worker1OSCOriginal := expected[1]
				worker1OSCOriginal.TypeMeta = metav1.TypeMeta{APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(), Kind: "OperatingSystemConfig"}
				worker1OSCOriginal.Annotations = nil

				worker2OSCDownloader := expected[2]
				worker2OSCDownloader.TypeMeta = metav1.TypeMeta{APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(), Kind: "OperatingSystemConfig"}
				worker2OSCDownloader.Annotations = nil

				worker2OSCOriginal := expected[3]
				worker2OSCOriginal.TypeMeta = metav1.TypeMeta{APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(), Kind: "OperatingSystemConfig"}
				worker2OSCOriginal.Annotations = nil

				Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())
				Expect(defaultDepWaiter.WorkerNameToOperatingSystemConfigsMap()).To(Equal(map[string]*OperatingSystemConfigs{
					worker1Name: {
						Downloader: Data{
							Content: "foobar-gardener-node-agent-" + worker1Name + "-77ac3-type1-init",
							Command: ptr.To("foo-gardener-node-agent-" + worker1Name + "-77ac3-type1-init"),
							Units: []string{
								"bar-gardener-node-agent-" + worker1Name + "-77ac3-type1-init",
								"baz-gardener-node-agent-" + worker1Name + "-77ac3-type1-init",
							},
							Object: worker1OSCDownloader,
						},
						Original: Data{
							Content: "foobar-gardener-node-agent-" + worker1Name + "-77ac3-type1-original",
							Command: ptr.To("foo-gardener-node-agent-" + worker1Name + "-77ac3-type1-original"),
							Units: []string{
								"bar-gardener-node-agent-" + worker1Name + "-77ac3-type1-original",
								"baz-gardener-node-agent-" + worker1Name + "-77ac3-type1-original",
							},
							Object: worker1OSCOriginal,
						},
					},
					worker2Name: {
						Downloader: Data{
							Content: "foobar-gardener-node-agent-" + worker2Name + "-d9e53-type2-init",
							Command: ptr.To("foo-gardener-node-agent-" + worker2Name + "-d9e53-type2-init"),
							Units: []string{
								"bar-gardener-node-agent-" + worker2Name + "-d9e53-type2-init",
								"baz-gardener-node-agent-" + worker2Name + "-d9e53-type2-init",
							},
							Object: worker2OSCDownloader,
						},
						Original: Data{
							Content: "foobar-gardener-node-agent-" + worker2Name + "-d9e53-type2-original",
							Command: ptr.To("foo-gardener-node-agent-" + worker2Name + "-d9e53-type2-original"),
							Units: []string{
								"bar-gardener-node-agent-" + worker2Name + "-d9e53-type2-original",
								"baz-gardener-node-agent-" + worker2Name + "-d9e53-type2-original",
							},
							Object: worker2OSCOriginal,
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
							gardenerutils.ConfirmationDeletion: "true",
							v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
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
				Expect(defaultDepWaiter.Destroy(ctx)).To(MatchError(multierror.Append(fakeErr)))
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

				for _, e := range expected {
					Expect(c.Create(ctx, e)).To(Succeed())
				}

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
				Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should not return error if wanted resources exist", func() {
				Expect(c.Create(ctx, expected[0])).To(Succeed())
				Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should return error if stale resources still exist", func() {
				staleOSC := expected[0].DeepCopy()
				staleOSC.Name = "new-name"
				Expect(c.Create(ctx, staleOSC)).To(Succeed(), "creating stale OSC succeeds")

				Expect(defaultDepWaiter.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("OperatingSystemConfig test-namespace/new-name is still present")))
			})
		})
	})

	Describe("#Key", func() {
		var (
			workerName        = "foo"
			kubernetesVersion = "1.2.3"
		)

		It("should return an empty string", func() {
			Expect(Key(workerName, nil, nil)).To(BeEmpty())
		})

		It("should return the expected key when UseGardenerNodeAgent feature gate is off", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, false))
			Expect(Key(workerName, semver.MustParse(kubernetesVersion), nil)).To(Equal("cloud-config-" + workerName + "-77ac3"))
		})

		It("should return the expected key when UseGardenerNodeAgent feature gate is on", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, true))
			Expect(Key(workerName, semver.MustParse(kubernetesVersion), nil)).To(Equal("gardener-node-agent-" + workerName + "-77ac3"))
		})

		It("is different for different worker.cri configurations", func() {
			containerDKey := Key(workerName, semver.MustParse("1.2.3"), &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD})
			otherKey := Key(workerName, semver.MustParse("1.2.3"), &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRIName("other")})
			Expect(containerDKey).NotTo(Equal(otherKey))
		})
	})

	Describe("#LegacyKey", func() {
		var (
			workerName        = "foo"
			kubernetesVersion = "1.2.3"
		)

		It("should return an empty string", func() {
			Expect(LegacyKey(workerName, nil, nil)).To(BeEmpty())
		})

		It("should return the expected key", func() {
			Expect(LegacyKey(workerName, semver.MustParse(kubernetesVersion), nil)).To(Equal("cloud-config-" + workerName + "-77ac3"))
		})

		It("is different for different worker.cri configurations", func() {
			containerDKey := LegacyKey(workerName, semver.MustParse("1.2.3"), &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD})
			otherKey := LegacyKey(workerName, semver.MustParse("1.2.3"), &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRIName("other")})
			Expect(containerDKey).NotTo(Equal(otherKey))
		})
	})
})
