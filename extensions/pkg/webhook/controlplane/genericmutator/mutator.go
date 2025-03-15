// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericmutator

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
)

// Ensurer ensures that various standard Kubernetes control plane objects conform to the provider requirements.
// If they don't initially, they are mutated accordingly.
type Ensurer interface {
	// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeAPIServerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *appsv1.Deployment) error
	// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeControllerManagerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *appsv1.Deployment) error
	// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeSchedulerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *appsv1.Deployment) error
	// EnsureClusterAutoscalerDeployment ensures that the cluster-autoscaler deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureClusterAutoscalerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *appsv1.Deployment) error
	// EnsureMachineControllerManagerDeployment ensures that the machine-controller-manager deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureMachineControllerManagerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *appsv1.Deployment) error
	// EnsureMachineControllerManagerVPA ensures that the machine-controller-manager VPA settings conform to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureMachineControllerManagerVPA(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *vpaautoscalingv1.VerticalPodAutoscaler) error
	// EnsureETCD ensures that the etcds conform to the respective provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureETCD(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *druidcorev1alpha1.Etcd) error
	// EnsureVPNSeedServerDeployment ensures that the vpn-seed-server deployment conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureVPNSeedServerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *appsv1.Deployment) error
	// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
	EnsureKubeletServiceUnitOptions(ctx context.Context, gctx extensionscontextwebhook.GardenContext, kubeletVersion *semver.Version, new, old []*unit.UnitOption) ([]*unit.UnitOption, error)
	// EnsureKubeletConfiguration ensures that the kubelet configuration conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubeletConfiguration(ctx context.Context, gctx extensionscontextwebhook.GardenContext, kubeletVersion *semver.Version, new, old *kubeletconfigv1beta1.KubeletConfiguration) error
	// ShouldProvisionKubeletCloudProviderConfig returns true if the cloud provider config file should be added to the kubelet configuration.
	ShouldProvisionKubeletCloudProviderConfig(ctx context.Context, gctx extensionscontextwebhook.GardenContext, kubeletVersion *semver.Version) bool
	// EnsureKubeletCloudProviderConfig ensures that the cloud provider config file content conforms to the provider requirements.
	EnsureKubeletCloudProviderConfig(ctx context.Context, gctx extensionscontextwebhook.GardenContext, kubeletVersion *semver.Version, configContent *string, namespace string) error
	// EnsureKubernetesGeneralConfiguration ensures that the kubernetes general configuration conforms to the provider requirements.
	// "old" might be "nil" and must always be checked.
	EnsureKubernetesGeneralConfiguration(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *string) error
	// EnsureAdditionalUnits ensures additional systemd units
	// "old" might be "nil" and must always be checked.
	EnsureAdditionalUnits(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *[]extensionsv1alpha1.Unit) error
	// EnsureAdditionalFiles ensures additional systemd files
	// "old" might be "nil" and must always be checked.
	EnsureAdditionalFiles(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *[]extensionsv1alpha1.File) error
	// EnsureAdditionalProvisionUnits ensures additional systemd units for the 'provision' OSC
	// "old" might be "nil" and must always be checked.
	EnsureAdditionalProvisionUnits(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *[]extensionsv1alpha1.Unit) error
	// EnsureAdditionalProvisionFiles ensures additional systemd files for the 'provision' OSC
	// "old" might be "nil" and must always be checked.
	EnsureAdditionalProvisionFiles(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *[]extensionsv1alpha1.File) error
	// EnsureCRIConfig ensures the CRI config.
	// "old" might be "nil" and must always be checked.
	EnsureCRIConfig(ctx context.Context, gctx extensionscontextwebhook.GardenContext, new, old *extensionsv1alpha1.CRIConfig) error
}

// NewMutator creates a new controlplane mutator.
func NewMutator(
	mgr manager.Manager,
	ensurer Ensurer,
	unitSerializer utils.UnitSerializer,
	kubeletConfigCodec kubelet.ConfigCodec,
	fciCodec utils.FileContentInlineCodec,
	logger logr.Logger,
) extensionswebhook.Mutator {
	return &mutator{
		client:             mgr.GetClient(),
		ensurer:            ensurer,
		unitSerializer:     unitSerializer,
		kubeletConfigCodec: kubeletConfigCodec,
		fciCodec:           fciCodec,
		logger:             logger.WithName("mutator"),
	}
}

type mutator struct {
	client             client.Client
	ensurer            Ensurer
	unitSerializer     utils.UnitSerializer
	kubeletConfigCodec kubelet.ConfigCodec
	fciCodec           utils.FileContentInlineCodec
	logger             logr.Logger
}

// Mutate validates and if needed mutates the given object.
func (m *mutator) Mutate(ctx context.Context, new, old client.Object) error {
	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if new.GetDeletionTimestamp() != nil {
		return nil
	}
	gctx := extensionscontextwebhook.NewGardenContext(m.client, new)

	switch x := new.(type) {
	case *appsv1.Deployment:
		var oldDep *appsv1.Deployment
		if old != nil {
			var ok bool
			oldDep, ok = old.(*appsv1.Deployment)
			if !ok {
				return errors.New("could not cast old object to appsv1.Deployment")
			}
		}

		switch x.Name {
		case v1beta1constants.DeploymentNameKubeAPIServer:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeAPIServerDeployment(ctx, gctx, x, oldDep)
		case v1beta1constants.DeploymentNameKubeControllerManager:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeControllerManagerDeployment(ctx, gctx, x, oldDep)
		case v1beta1constants.DeploymentNameKubeScheduler:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureKubeSchedulerDeployment(ctx, gctx, x, oldDep)
		case v1beta1constants.DeploymentNameMachineControllerManager:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureMachineControllerManagerDeployment(ctx, gctx, x, oldDep)
		case v1beta1constants.DeploymentNameClusterAutoscaler:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureClusterAutoscalerDeployment(ctx, gctx, x, oldDep)
		case v1beta1constants.DeploymentNameVPNSeedServer:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureVPNSeedServerDeployment(ctx, gctx, x, oldDep)
		}
	case *vpaautoscalingv1.VerticalPodAutoscaler:
		var oldVPA *vpaautoscalingv1.VerticalPodAutoscaler
		if old != nil {
			var ok bool
			oldVPA, ok = old.(*vpaautoscalingv1.VerticalPodAutoscaler)
			if !ok {
				return errors.New("could not cast old object to vpaautoscalingv1.VerticalPodAutoscaler")
			}
		}

		switch x.Name {
		case machinecontrollermanager.VPAName:
			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureMachineControllerManagerVPA(ctx, gctx, x, oldVPA)
		}
	case *druidcorev1alpha1.Etcd:
		switch x.Name {
		case v1beta1constants.ETCDMain, v1beta1constants.ETCDEvents:
			var oldEtcd *druidcorev1alpha1.Etcd
			if old != nil {
				var ok bool
				oldEtcd, ok = old.(*druidcorev1alpha1.Etcd)
				if !ok {
					return errors.New("could not cast old object to druidcorev1alpha1.Etcd")
				}
			}

			extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)
			return m.ensurer.EnsureETCD(ctx, gctx, x, oldEtcd)
		}
	case *extensionsv1alpha1.OperatingSystemConfig:
		var oldOSC *extensionsv1alpha1.OperatingSystemConfig
		if old != nil {
			var ok bool
			oldOSC, ok = old.(*extensionsv1alpha1.OperatingSystemConfig)
			if !ok {
				return errors.New("could not cast old object to extensionsv1alpha1.OperatingSystemConfig")
			}
		}

		extensionswebhook.LogMutation(m.logger, x.Kind, x.Namespace, x.Name)

		switch x.Spec.Purpose {
		case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
			return m.mutateOperatingSystemConfigProvision(ctx, gctx, x, oldOSC)
		case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
			return m.mutateOperatingSystemConfigReconcile(ctx, gctx, x, oldOSC)
		default:
			return nil
		}
	}
	return nil
}

func getKubeletService(osc *extensionsv1alpha1.OperatingSystemConfig) *string {
	if osc != nil {
		if u := extensionswebhook.UnitWithName(osc.Spec.Units, v1beta1constants.OperatingSystemConfigUnitNameKubeletService); u != nil {
			return u.Content
		}
	}

	return nil
}

func getKubeletConfigFile(osc *extensionsv1alpha1.OperatingSystemConfig) *extensionsv1alpha1.FileContentInline {
	return findFileWithPath(osc, v1beta1constants.OperatingSystemConfigFilePathKubeletConfig)
}

func getKubernetesGeneralConfiguration(osc *extensionsv1alpha1.OperatingSystemConfig) *extensionsv1alpha1.FileContentInline {
	return findFileWithPath(osc, v1beta1constants.OperatingSystemConfigFilePathKernelSettings)
}

func findFileWithPath(osc *extensionsv1alpha1.OperatingSystemConfig, path string) *extensionsv1alpha1.FileContentInline {
	if osc != nil {
		if f := extensionswebhook.FileWithPath(osc.Spec.Files, path); f != nil {
			return f.Content.Inline
		}
	}

	return nil
}

func (m *mutator) mutateOperatingSystemConfigProvision(ctx context.Context, gctx extensionscontextwebhook.GardenContext, osc, oldOSC *extensionsv1alpha1.OperatingSystemConfig) error {
	var (
		oldFiles *[]extensionsv1alpha1.File
		oldUnits *[]extensionsv1alpha1.Unit
	)

	if oldOSC != nil {
		oldFiles = &oldOSC.Spec.Files
		oldUnits = &oldOSC.Spec.Units
	}

	if err := m.ensurer.EnsureAdditionalProvisionFiles(ctx, gctx, &osc.Spec.Files, oldFiles); err != nil {
		return err
	}

	return m.ensurer.EnsureAdditionalProvisionUnits(ctx, gctx, &osc.Spec.Units, oldUnits)
}

func (m *mutator) mutateOperatingSystemConfigReconcile(ctx context.Context, gctx extensionscontextwebhook.GardenContext, osc, oldOSC *extensionsv1alpha1.OperatingSystemConfig) error {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	// Calculate effective kubelet version for the worker pool this OperatingSystemConfig belongs to
	controlPlaneVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	var workerKubernetes *gardencorev1beta1.WorkerKubernetes
	if poolName, ok := osc.Labels[v1beta1constants.LabelWorkerPool]; ok {
		for _, worker := range cluster.Shoot.Spec.Provider.Workers {
			if worker.Name == poolName {
				workerKubernetes = worker.Kubernetes
				break
			}
		}
	}

	kubeletVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(controlPlaneVersion, workerKubernetes)
	if err != nil {
		return err
	}

	// Mutate kubelet.service unit, if present
	if content := getKubeletService(osc); content != nil {
		if err := m.ensureKubeletServiceUnitContent(ctx, gctx, kubeletVersion, content, getKubeletService(oldOSC)); err != nil {
			return err
		}
	}

	// Mutate kubelet configuration file, if present
	if content := getKubeletConfigFile(osc); content != nil {
		if err := m.ensureKubeletConfigFileContent(ctx, gctx, kubeletVersion, content, getKubeletConfigFile(oldOSC)); err != nil {
			return err
		}
	}

	// Mutate 99 kubernetes general configuration file, if present
	if content := getKubernetesGeneralConfiguration(osc); content != nil {
		if err := m.ensureKubernetesGeneralConfiguration(ctx, gctx, content, getKubernetesGeneralConfiguration(oldOSC), client.ObjectKeyFromObject(osc)); err != nil {
			return err
		}
	}

	// Check if cloud provider config needs to be ensured
	if m.ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, gctx, kubeletVersion) {
		if err := m.ensureKubeletCloudProviderConfig(ctx, gctx, kubeletVersion, osc); err != nil {
			return err
		}
	}

	var (
		oldFiles     *[]extensionsv1alpha1.File
		oldUnits     *[]extensionsv1alpha1.Unit
		oldCRIConfig *extensionsv1alpha1.CRIConfig
	)

	if oldOSC != nil {
		oldFiles = &oldOSC.Spec.Files
		oldUnits = &oldOSC.Spec.Units
		oldCRIConfig = oldOSC.Spec.CRIConfig
	}

	if err := m.ensurer.EnsureAdditionalFiles(ctx, gctx, &osc.Spec.Files, oldFiles); err != nil {
		return err
	}

	if err := m.ensurer.EnsureAdditionalUnits(ctx, gctx, &osc.Spec.Units, oldUnits); err != nil {
		return err
	}

	return m.ensurer.EnsureCRIConfig(ctx, gctx, osc.Spec.CRIConfig, oldCRIConfig)
}

func (m *mutator) ensureKubeletServiceUnitContent(ctx context.Context, gctx extensionscontextwebhook.GardenContext, kubeletVersion *semver.Version, content, oldContent *string) error {
	var (
		opts, oldOpts []*unit.UnitOption
		err           error
	)

	// Deserialize unit options
	if opts, err = m.unitSerializer.Deserialize(*content); err != nil {
		return fmt.Errorf("could not deserialize kubelet.service unit content: %w", err)
	}

	if oldContent != nil {
		// Deserialize old unit options
		if oldOpts, err = m.unitSerializer.Deserialize(*oldContent); err != nil {
			return fmt.Errorf("could not deserialize old kubelet.service unit content: %w", err)
		}
	}

	if opts, err = m.ensurer.EnsureKubeletServiceUnitOptions(ctx, gctx, kubeletVersion, opts, oldOpts); err != nil {
		return err
	}

	// Serialize unit options
	if *content, err = m.unitSerializer.Serialize(opts); err != nil {
		return fmt.Errorf("could not serialize kubelet.service unit options: %w", err)
	}

	return nil
}

func (m *mutator) ensureKubeletConfigFileContent(ctx context.Context, gctx extensionscontextwebhook.GardenContext, kubeletVersion *semver.Version, fci, oldFCI *extensionsv1alpha1.FileContentInline) error {
	var (
		kubeletConfig, oldKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration
		err                             error
	)

	// Decode kubelet configuration from inline content
	if kubeletConfig, err = m.kubeletConfigCodec.Decode(fci); err != nil {
		return fmt.Errorf("could not decode kubelet configuration: %w", err)
	}

	if oldFCI != nil {
		// Decode old kubelet configuration from inline content
		if oldKubeletConfig, err = m.kubeletConfigCodec.Decode(oldFCI); err != nil {
			return fmt.Errorf("could not decode old kubelet configuration: %w", err)
		}
	}

	if err = m.ensurer.EnsureKubeletConfiguration(ctx, gctx, kubeletVersion, kubeletConfig, oldKubeletConfig); err != nil {
		return err
	}

	// Encode kubelet configuration into inline content
	var newFCI *extensionsv1alpha1.FileContentInline
	if newFCI, err = m.kubeletConfigCodec.Encode(kubeletConfig, fci.Encoding); err != nil {
		return fmt.Errorf("could not encode kubelet configuration: %w", err)
	}
	*fci = *newFCI

	return nil
}

func (m *mutator) ensureKubernetesGeneralConfiguration(ctx context.Context, gctx extensionscontextwebhook.GardenContext, fci, oldFCI *extensionsv1alpha1.FileContentInline, key client.ObjectKey) error {
	var (
		data, oldData []byte
		err           error
	)

	// Decode kubernetes general configuration from inline content
	if data, err = m.fciCodec.Decode(fci); err != nil {
		return fmt.Errorf("could not decode kubernetes general configuration: %w", err)
	}

	if oldFCI != nil {
		// Decode kubernetes general configuration from inline content
		if oldData, err = m.fciCodec.Decode(oldFCI); err != nil {
			return fmt.Errorf("could not decode old kubernetes general configuration: %w", err)
		}
	}

	s := string(data)
	oldS := string(oldData)
	if err = m.ensurer.EnsureKubernetesGeneralConfiguration(ctx, gctx, &s, &oldS); err != nil {
		return err
	}

	if len(s) == 0 {
		// File entries with empty content are not valid, so we do not add them to the OperatingSystemConfig resource.
		m.logger.Info("Skipping modification of kubernetes general configuration file entry because the new content is empty", "operatingsystemconfig", key)
		return nil
	}

	// Encode kubernetes general configuration into inline content
	var newFCI *extensionsv1alpha1.FileContentInline
	if newFCI, err = m.fciCodec.Encode([]byte(s), fci.Encoding); err != nil {
		return fmt.Errorf("could not encode kubernetes general configuration: %w", err)
	}
	*fci = *newFCI

	return nil
}

// CloudProviderConfigPath is the path to the cloudprovider.conf kubelet configuration file.
const CloudProviderConfigPath = "/var/lib/kubelet/cloudprovider.conf"

func (m *mutator) ensureKubeletCloudProviderConfig(ctx context.Context, gctx extensionscontextwebhook.GardenContext, kubeletVersion *semver.Version, osc *extensionsv1alpha1.OperatingSystemConfig) error {
	var err error

	// Ensure kubelet cloud provider config
	var s string
	if err = m.ensurer.EnsureKubeletCloudProviderConfig(ctx, gctx, kubeletVersion, &s, osc.Namespace); err != nil {
		return err
	}

	if len(s) == 0 {
		// File entries with empty content are not valid, so we do not add them to the OperatingSystemConfig resource.
		m.logger.Info("Skipping addition of kubelet cloud provider config file entry because its content is empty", "operatingsystemconfig", client.ObjectKeyFromObject(osc))
		return nil
	}

	// Encode cloud provider config into inline content
	var fci *extensionsv1alpha1.FileContentInline
	if fci, err = m.fciCodec.Encode([]byte(s), string(extensionsv1alpha1.B64FileCodecID)); err != nil {
		return fmt.Errorf("could not encode kubelet cloud provider config: %w", err)
	}

	// Ensure the cloud provider config file is part of the OperatingSystemConfig
	osc.Spec.Files = extensionswebhook.EnsureFileWithPath(osc.Spec.Files, extensionsv1alpha1.File{
		Path:        CloudProviderConfigPath,
		Permissions: ptr.To[uint32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: fci,
		},
	})
	return nil
}
