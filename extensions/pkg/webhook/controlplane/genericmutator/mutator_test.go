// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericmutator_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	extensionsmockgenericmutator "github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockkubelet "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet/mock"
	mockutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils/mock"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

const (
	oldServiceContent     = "new kubelet.service content"
	newServiceContent     = "old kubelet.service content"
	mutatedServiceContent = "mutated kubelet.service content"

	oldKubeletConfigData     = "old kubelet config data"
	newKubeletConfigData     = "new kubelet config data"
	mutatedKubeletConfigData = "mutated kubelet config data"

	oldKubernetesGeneralConfigData     = "# Increase the tcp-time-wait buckets pool size to prevent simple DOS attacks\nnet.ipv4.tcp_tw_reuse = 1\n# OLD Settings"
	newKubernetesGeneralConfigData     = "# Increase the tcp-time-wait buckets pool size to prevent simple DOS attacks\nnet.ipv4.tcp_tw_reuse = 1"
	mutatedKubernetesGeneralConfigData = "# Increase the tcp-time-wait buckets pool size to prevent simple DOS attacks\nnet.ipv4.tcp_tw_reuse = 1\n# Provider specific settings"

	encoding                 = "b64"
	cloudproviderconf        = "[Global]\nauth-url: whatever-url/keystone"
	cloudproviderconfEncoded = "W0dsb2JhbF1cbmF1dGgtdXJsOiBodHRwczovL2NsdXN0ZXIuZXUtZGUtMjAwLmNsb3VkLnNhcDo1MDAwL3Yz"
)

const (
	namespace = "test"
)

func TestControlPlane(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Webhook ControlPlane GenericMutator Suite")
}

var _ = Describe("Mutator", func() {
	var (
		ctrl   *gomock.Controller
		logger = log.Log.WithName("test")
		mgr    *mockmanager.MockManager
		c      *mockclient.MockClient

		kubernetesVersion       = "1.28.4"
		kubernetesVersionSemver = semver.MustParse(kubernetesVersion)

		clusterKey = client.ObjectKey{Name: namespace}
		cluster    = &extensionscontroller.Cluster{
			CloudProfile: &gardencorev1beta1.CloudProfile{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "CloudProfile",
				},
			},
			Seed: &gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Seed",
				},
			},
			Shoot: &gardencorev1beta1.Shoot{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: kubernetesVersion,
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		// Create fake manager and client
		mgr = mockmanager.NewMockManager(ctrl)
		c = mockclient.NewMockClient(ctrl)
		mgr.EXPECT().GetClient().Return(c)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Mutate", func() {
		var (
			mutator extensionswebhook.Mutator
			kcc     *mockkubelet.MockConfigCodec
			ensurer *extensionsmockgenericmutator.MockEnsurer
			us      *mockutils.MockUnitSerializer
			fcic    *mockutils.MockFileContentInlineCodec

			oldObj, newObj client.Object
		)

		BeforeEach(func() {
			ensurer = extensionsmockgenericmutator.NewMockEnsurer(ctrl)
			kcc = mockkubelet.NewMockConfigCodec(ctrl)
			us = mockutils.NewMockUnitSerializer(ctrl)
			fcic = mockutils.NewMockFileContentInlineCodec(ctrl)
			mutator = genericmutator.NewMutator(mgr, ensurer, us, kcc, fcic, logger)
			oldObj = nil
			newObj = nil
		})

		DescribeTable("Should ignore", func(new, old client.Object) {
			err := mutator.Mutate(context.Background(), new, old)
			Expect(err).To(Not(HaveOccurred()))
		},
			Entry(
				"other services than kube-apiserver",
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
				nil,
			),
			Entry(
				"other deployments than kube-apiserver, kube-controller-manager, machine-controller-manager, and kube-scheduler",
				&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
				nil,
			),
			Entry(
				"other VPAs than machine-controller-manager",
				&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
				nil,
			),
			Entry(
				"other etcds than etcd-main and etcd-events",
				&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
				nil,
			),
		)

		DescribeTable("Should ensure", func(ensureFunc func()) {
			ensureFunc()

			err := mutator.Mutate(context.Background(), newObj, oldObj)
			Expect(err).To(Not(HaveOccurred()))
		},
			Entry(
				"EnsureKubeAPIServerDeployment with a kube-apiserver deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer}}
					ensurer.EXPECT().EnsureKubeAPIServerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureKubeAPIServerDeployment with a kube-apiserver deployment and existing deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer}}
					oldObj = newObj.DeepCopyObject().(client.Object)
					ensurer.EXPECT().EnsureKubeAPIServerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureKubeControllerManagerDeployment with a kube-controller-manager deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeControllerManager}}
					ensurer.EXPECT().EnsureKubeControllerManagerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureKubeControllerManagerDeployment with a kube-controller-manager deployment and existing deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeControllerManager}}
					oldObj = newObj.DeepCopyObject().(client.Object)
					ensurer.EXPECT().EnsureKubeControllerManagerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureKubeSchedulerDeployment with a kube-scheduler deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler}}
					ensurer.EXPECT().EnsureKubeSchedulerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureKubeSchedulerDeployment with a kube-scheduler deployment and existing deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler}}
					oldObj = newObj.DeepCopyObject().(client.Object)
					ensurer.EXPECT().EnsureKubeSchedulerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureClusterAutoscalerDeployment with a cluster-autoscaler deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameClusterAutoscaler}}
					ensurer.EXPECT().EnsureClusterAutoscalerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureMachineControllerManagerDeployment with a machine-controller-manager deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager}}
					ensurer.EXPECT().EnsureMachineControllerManagerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureMachineControllerManagerVPA with a machine-controller-manager VPA",
				func() {
					newObj = &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager-vpa"}}
					ensurer.EXPECT().EnsureMachineControllerManagerVPA(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureClusterAutoscalerDeployment with a cluster-autoscaler deployment and existing deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameClusterAutoscaler}}
					oldObj = newObj.DeepCopyObject().(client.Object)
					ensurer.EXPECT().EnsureClusterAutoscalerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureVPNSeedServerDeployment with a vpn-seed-server deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPNSeedServer}}
					ensurer.EXPECT().EnsureVPNSeedServerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
			Entry(
				"EnsureVPNSeedServerDeployment with a vpn-seed-server deployment and existing deployment",
				func() {
					newObj = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPNSeedServer}}
					oldObj = newObj.DeepCopyObject().(client.Object)
					ensurer.EXPECT().EnsureVPNSeedServerDeployment(context.Background(), gomock.Any(), newObj, oldObj).Return(nil)
				},
			),
		)

		DescribeTable("EnsureETCD", func(newObj, oldObj *druidv1alpha1.Etcd) {
			c.EXPECT().Get(context.Background(), clusterKey, &extensionsv1alpha1.Cluster{}).DoAndReturn(clientGet(clusterObject(cluster)))

			ensurer.EXPECT().EnsureETCD(context.Background(), gomock.Any(), newObj, oldObj).Return(nil).Do(func(ctx context.Context, gctx extensionscontextwebhook.GardenContext, _, _ *druidv1alpha1.Etcd) {
				_, err := gctx.GetCluster(ctx)
				if err != nil {
					logger.Error(err, "Failed to get cluster object")
				}
			})

			// Call Mutate method and check the result
			err := mutator.Mutate(context.Background(), newObj, oldObj)
			Expect(err).To(Not(HaveOccurred()))
		},
			Entry(
				"with a etcd-main",
				&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain, Namespace: namespace}},
				nil,
			),
			Entry(
				"with a etcd-main and existing druid",
				&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain, Namespace: namespace}},
				&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain, Namespace: namespace}},
			),
			Entry(
				"with a etcd-events",
				&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents, Namespace: namespace}},
				nil,
			),
			Entry(
				"with a etcd-events and existing druid",
				&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents, Namespace: namespace}},
				&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents, Namespace: namespace}},
			),
		)

		Context("OperatingSystemConfig mutation", func() {
			var newOSC *extensionsv1alpha1.OperatingSystemConfig

			Context("provision purpose", func() {
				var (
					additionalUnit = extensionsv1alpha1.Unit{Name: "custom-provision-unit.service"}
					additionalFile = extensionsv1alpha1.File{Path: "/test/provision"}
				)

				BeforeEach(func() {
					newOSC = &extensionsv1alpha1.OperatingSystemConfig{
						ObjectMeta: metav1.ObjectMeta{Name: "test-provision", Namespace: "test"},
						Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
							Purpose: extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
						},
					}
				})

				It("should invoke appropriate ensurer methods with OperatingSystemConfig", func() {
					oldProvisionOSC := newOSC.DeepCopy()

					// Create mock ensurer
					ensurer.EXPECT().EnsureAdditionalProvisionUnits(context.Background(), gomock.Any(), &newOSC.Spec.Units, &oldProvisionOSC.Spec.Units).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, oscUnits, _ *[]extensionsv1alpha1.Unit) error {
							*oscUnits = append(*oscUnits, additionalUnit)
							return nil
						})
					ensurer.EXPECT().EnsureAdditionalProvisionFiles(context.Background(), gomock.Any(), &newOSC.Spec.Files, &oldProvisionOSC.Spec.Files).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, oscFiles, _ *[]extensionsv1alpha1.File) error {
							*oscFiles = append(*oscFiles, additionalFile)
							return nil
						})

					// Call Mutate method and check the result
					err := mutator.Mutate(context.Background(), newOSC, oldProvisionOSC)
					Expect(err).To(Not(HaveOccurred()))
					checkProvisionOperatingSystemConfig(newOSC)
				})
			})

			Context("reconcile purpose", func() {
				var (
					oldUnitOptions       []*unit.UnitOption
					newUnitOptions       []*unit.UnitOption
					mutatedUnitOptions   []*unit.UnitOption
					oldKubeletConfig     *kubeletconfigv1beta1.KubeletConfiguration
					newKubeletConfig     *kubeletconfigv1beta1.KubeletConfiguration
					mutatedKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration
					additionalUnit       = extensionsv1alpha1.Unit{Name: "custom-mtu.service"}
					additionalFile       = extensionsv1alpha1.File{Path: "/test/path"}
				)

				BeforeEach(func() {
					newOSC = &extensionsv1alpha1.OperatingSystemConfig{
						ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
						Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
							Purpose: extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
							CRIConfig: &extensionsv1alpha1.CRIConfig{
								Name: "containerd",
								Containerd: &extensionsv1alpha1.ContainerdConfig{
									Registries: []extensionsv1alpha1.RegistryConfig{
										{Upstream: "registry.k8s.io"},
									},
								},
							},
							Units: []extensionsv1alpha1.Unit{
								{
									Name:    v1beta1constants.OperatingSystemConfigUnitNameKubeletService,
									Content: ptr.To(newServiceContent),
								},
							},
							Files: []extensionsv1alpha1.File{
								{
									Path: v1beta1constants.OperatingSystemConfigFilePathKubeletConfig,
									Content: extensionsv1alpha1.FileContent{
										Inline: &extensionsv1alpha1.FileContentInline{
											Data: newKubeletConfigData,
										},
									},
								},
								{
									Path: v1beta1constants.OperatingSystemConfigFilePathKernelSettings,
									Content: extensionsv1alpha1.FileContent{
										Inline: &extensionsv1alpha1.FileContentInline{
											Data: newKubernetesGeneralConfigData,
										},
									},
								},
							},
						},
					}
					oldUnitOptions = []*unit.UnitOption{
						{
							Section: "Service",
							Name:    "Foo",
							Value:   "old",
						},
					}
					newUnitOptions = []*unit.UnitOption{
						{
							Section: "Service",
							Name:    "Foo",
							Value:   "bar",
						},
					}
					mutatedUnitOptions = []*unit.UnitOption{
						{
							Section: "Service",
							Name:    "Foo",
							Value:   "baz",
						},
					}
					oldKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
						FeatureGates: map[string]bool{
							"Old": true,
						},
					}
					newKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
						FeatureGates: map[string]bool{
							"Foo": true,
							"Bar": true,
						},
					}
					mutatedKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
						FeatureGates: map[string]bool{
							"Foo": true,
						},
					}

					c.EXPECT().Get(context.Background(), clusterKey, &extensionsv1alpha1.Cluster{}).DoAndReturn(clientGet(clusterObject(cluster)))
				})

				It("should invoke appropriate ensurer methods with OperatingSystemConfig", func() {
					oldOSC := newOSC.DeepCopy()
					oldOSC.Spec.CRIConfig = nil
					oldOSC.Spec.Units[0].Content = ptr.To(oldServiceContent)
					oldOSC.Spec.Files[0].Content.Inline.Data = oldKubeletConfigData
					oldOSC.Spec.Files[1].Content.Inline.Data = oldKubernetesGeneralConfigData

					// Create mock ensurer
					ensurer.EXPECT().EnsureKubeletServiceUnitOptions(context.Background(), gomock.Any(), kubernetesVersionSemver, newUnitOptions, oldUnitOptions).Return(mutatedUnitOptions, nil)
					ensurer.EXPECT().EnsureKubeletConfiguration(context.Background(), gomock.Any(), kubernetesVersionSemver, newKubeletConfig, oldKubeletConfig).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, kubeletConfig, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
							*kubeletConfig = *mutatedKubeletConfig
							return nil
						},
					)
					ensurer.EXPECT().EnsureKubernetesGeneralConfiguration(context.Background(), gomock.Any(), ptr.To(newKubernetesGeneralConfigData), ptr.To(oldKubernetesGeneralConfigData)).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, newData, _ *string) error {
							*newData = mutatedKubernetesGeneralConfigData
							return nil
						},
					)
					ensurer.EXPECT().EnsureAdditionalUnits(context.Background(), gomock.Any(), &newOSC.Spec.Units, &oldOSC.Spec.Units).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, oscUnits, _ *[]extensionsv1alpha1.Unit) error {
							*oscUnits = append(*oscUnits, additionalUnit)
							return nil
						})
					ensurer.EXPECT().EnsureAdditionalFiles(context.Background(), gomock.Any(), &newOSC.Spec.Files, &oldOSC.Spec.Files).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, oscFiles, _ *[]extensionsv1alpha1.File) error {
							*oscFiles = append(*oscFiles, additionalFile)
							return nil
						})
					ensurer.EXPECT().EnsureCRIConfig(context.Background(), gomock.Any(), newOSC.Spec.CRIConfig, oldOSC.Spec.CRIConfig).Return(nil)

					ensurer.EXPECT().ShouldProvisionKubeletCloudProviderConfig(context.Background(), gomock.Any(), kubernetesVersionSemver).Return(true)
					ensurer.EXPECT().EnsureKubeletCloudProviderConfig(context.Background(), gomock.Any(), kubernetesVersionSemver, gomock.Any(), newOSC.Namespace).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, data *string, _ string) error {
							*data = cloudproviderconf
							return nil
						},
					)

					us.EXPECT().Deserialize(newServiceContent).Return(newUnitOptions, nil)
					us.EXPECT().Deserialize(oldServiceContent).Return(oldUnitOptions, nil)
					us.EXPECT().Serialize(mutatedUnitOptions).Return(mutatedServiceContent, nil)

					kcc.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: newKubeletConfigData}).Return(newKubeletConfig, nil)
					kcc.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: oldKubeletConfigData}).Return(oldKubeletConfig, nil)
					kcc.EXPECT().Encode(mutatedKubeletConfig, "").Return(&extensionsv1alpha1.FileContentInline{Data: mutatedKubeletConfigData}, nil)

					fcic.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: newKubernetesGeneralConfigData}).Return([]byte(newKubernetesGeneralConfigData), nil)
					fcic.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: oldKubernetesGeneralConfigData}).Return([]byte(oldKubernetesGeneralConfigData), nil)
					fcic.EXPECT().Encode([]byte(mutatedKubernetesGeneralConfigData), "").Return(&extensionsv1alpha1.FileContentInline{Data: mutatedKubernetesGeneralConfigData}, nil)
					fcic.EXPECT().Encode([]byte(cloudproviderconf), encoding).Return(&extensionsv1alpha1.FileContentInline{Data: cloudproviderconfEncoded, Encoding: encoding}, nil)

					// Call Mutate method and check the result
					err := mutator.Mutate(context.Background(), newOSC, oldOSC)
					Expect(err).To(Not(HaveOccurred()))
					checkOperatingSystemConfig(newOSC)
				})

				It("should not add invalid file content to OSC", func() {
					oldOSC := newOSC.DeepCopy()
					oldOSC.Spec.Units[0].Content = ptr.To(oldServiceContent)
					oldOSC.Spec.Files[0].Content.Inline.Data = oldKubeletConfigData
					oldOSC.Spec.Files[1].Content.Inline.Data = oldKubernetesGeneralConfigData

					// Create mock ensurer
					ensurer.EXPECT().EnsureKubeletServiceUnitOptions(context.Background(), gomock.Any(), kubernetesVersionSemver, newUnitOptions, oldUnitOptions).Return(mutatedUnitOptions, nil)
					ensurer.EXPECT().EnsureKubeletConfiguration(context.Background(), gomock.Any(), kubernetesVersionSemver, newKubeletConfig, oldKubeletConfig).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, kubeletConfig, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
							*kubeletConfig = *mutatedKubeletConfig
							return nil
						},
					)
					ensurer.EXPECT().EnsureKubernetesGeneralConfiguration(context.Background(), gomock.Any(), ptr.To(newKubernetesGeneralConfigData), ptr.To(oldKubernetesGeneralConfigData)).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, newData, _ *string) error {
							*newData = ""
							return nil
						},
					)
					ensurer.EXPECT().EnsureAdditionalUnits(context.Background(), gomock.Any(), &newOSC.Spec.Units, &oldOSC.Spec.Units).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, oscUnits, _ *[]extensionsv1alpha1.Unit) error {
							*oscUnits = append(*oscUnits, additionalUnit)
							return nil
						})
					ensurer.EXPECT().EnsureAdditionalFiles(context.Background(), gomock.Any(), &newOSC.Spec.Files, &oldOSC.Spec.Files).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, oscFiles, _ *[]extensionsv1alpha1.File) error {
							*oscFiles = append(*oscFiles, additionalFile)
							return nil
						})

					ensurer.EXPECT().EnsureCRIConfig(context.Background(), gomock.Any(), newOSC.Spec.CRIConfig, oldOSC.Spec.CRIConfig).Return(nil)

					ensurer.EXPECT().ShouldProvisionKubeletCloudProviderConfig(context.Background(), gomock.Any(), kubernetesVersionSemver).Return(true)
					ensurer.EXPECT().EnsureKubeletCloudProviderConfig(context.Background(), gomock.Any(), kubernetesVersionSemver, gomock.Any(), newOSC.Namespace).DoAndReturn(
						func(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, data *string, _ string) error {
							*data = ""
							return nil
						},
					)

					us.EXPECT().Deserialize(newServiceContent).Return(newUnitOptions, nil)
					us.EXPECT().Deserialize(oldServiceContent).Return(oldUnitOptions, nil)
					us.EXPECT().Serialize(mutatedUnitOptions).Return(mutatedServiceContent, nil)

					kcc.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: newKubeletConfigData}).Return(newKubeletConfig, nil)
					kcc.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: oldKubeletConfigData}).Return(oldKubeletConfig, nil)
					kcc.EXPECT().Encode(mutatedKubeletConfig, "").Return(&extensionsv1alpha1.FileContentInline{Data: mutatedKubeletConfigData}, nil)

					fcic.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: newKubernetesGeneralConfigData}).Return([]byte(newKubernetesGeneralConfigData), nil)
					fcic.EXPECT().Decode(&extensionsv1alpha1.FileContentInline{Data: oldKubernetesGeneralConfigData}).Return([]byte(oldKubernetesGeneralConfigData), nil)

					// Call Mutate method and check the result
					err := mutator.Mutate(context.Background(), newOSC, oldOSC)
					Expect(err).To(Not(HaveOccurred()))

					general := extensionswebhook.FileWithPath(newOSC.Spec.Files, v1beta1constants.OperatingSystemConfigFilePathKernelSettings)
					Expect(general).To(Not(BeNil()))
					Expect(general.Content.Inline).To(Equal(&extensionsv1alpha1.FileContentInline{Data: newKubernetesGeneralConfigData}))
					cloudProvider := extensionswebhook.FileWithPath(newOSC.Spec.Files, genericmutator.CloudProviderConfigPath)
					Expect(cloudProvider).To(BeNil())
				})
			})
		})
	})
})

func checkOperatingSystemConfig(osc *extensionsv1alpha1.OperatingSystemConfig) {
	kubeletUnit := extensionswebhook.UnitWithName(osc.Spec.Units, v1beta1constants.OperatingSystemConfigUnitNameKubeletService)
	ExpectWithOffset(1, kubeletUnit).To(Not(BeNil()))
	ExpectWithOffset(1, kubeletUnit.Content).To(Equal(ptr.To(mutatedServiceContent)))

	customMTU := extensionswebhook.UnitWithName(osc.Spec.Units, "custom-mtu.service")
	ExpectWithOffset(1, customMTU).To(Not(BeNil()))

	customFile := extensionswebhook.FileWithPath(osc.Spec.Files, "/test/path")
	ExpectWithOffset(1, customFile).To(Not(BeNil()))

	kubeletFile := extensionswebhook.FileWithPath(osc.Spec.Files, v1beta1constants.OperatingSystemConfigFilePathKubeletConfig)
	ExpectWithOffset(1, kubeletFile).To(Not(BeNil()))
	ExpectWithOffset(1, kubeletFile.Content.Inline).To(Equal(&extensionsv1alpha1.FileContentInline{Data: mutatedKubeletConfigData}))

	general := extensionswebhook.FileWithPath(osc.Spec.Files, v1beta1constants.OperatingSystemConfigFilePathKernelSettings)
	ExpectWithOffset(1, general).To(Not(BeNil()))
	ExpectWithOffset(1, general.Content.Inline).To(Equal(&extensionsv1alpha1.FileContentInline{Data: mutatedKubernetesGeneralConfigData}))

	cloudProvider := extensionswebhook.FileWithPath(osc.Spec.Files, genericmutator.CloudProviderConfigPath)
	ExpectWithOffset(1, cloudProvider).To(Not(BeNil()))
	ExpectWithOffset(1, cloudProvider.Path).To(Equal(genericmutator.CloudProviderConfigPath))
	ExpectWithOffset(1, cloudProvider.Permissions).To(PointTo(Equal(uint32(0644))))
	ExpectWithOffset(1, cloudProvider.Content.Inline).To(Equal(&extensionsv1alpha1.FileContentInline{Data: cloudproviderconfEncoded, Encoding: encoding}))
}

func checkProvisionOperatingSystemConfig(osc *extensionsv1alpha1.OperatingSystemConfig) {
	customUnit := extensionswebhook.UnitWithName(osc.Spec.Units, "custom-provision-unit.service")
	ExpectWithOffset(1, customUnit).To(Not(BeNil()))

	customFile := extensionswebhook.FileWithPath(osc.Spec.Files, "/test/provision")
	ExpectWithOffset(1, customFile).To(Not(BeNil()))
}

func clientGet(result client.Object) any {
	return func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
		switch obj.(type) {
		case *extensionsv1alpha1.Cluster:
			*obj.(*extensionsv1alpha1.Cluster) = *result.(*extensionsv1alpha1.Cluster)
		}
		return nil
	}
}

func clusterObject(cluster *extensionscontroller.Cluster) *extensionsv1alpha1.Cluster {
	return &extensionsv1alpha1.Cluster{
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{
				Raw: encode(cluster.CloudProfile),
			},
			Seed: runtime.RawExtension{
				Raw: encode(cluster.Seed),
			},
			Shoot: runtime.RawExtension{
				Raw: encode(cluster.Shoot),
			},
		},
	}
}

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
