// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	nodeagentcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	operatingsystemconfigcontroller "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// DeployOperatingSystemConfigSecretForBootstrap deploys the OperatingSystemConfig resource and adds its content into
// a Secret so that gardener-node-agent can read it and reconcile its content.
func (b *AutonomousBotanist) DeployOperatingSystemConfigSecretForBootstrap(ctx context.Context) error {
	if err := b.deployControlPlaneDeployments(ctx); err != nil {
		return fmt.Errorf("failed deploying control plane deployments: %w", err)
	}

	oscData, controlPlaneWorkerPoolName, err := b.deployOperatingSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed deploying OperatingSystemConfig: %w", err)
	}

	return b.createOperatingSystemConfigSecretForNodeAgent(ctx, oscData.Object, oscData.GardenerNodeAgentSecretName, controlPlaneWorkerPoolName)
}

func (b *AutonomousBotanist) createOperatingSystemConfigSecretForNodeAgent(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, secretName, poolName string) error {
	var err error

	b.operatingSystemConfigSecret, err = nodeagentcomponent.OperatingSystemConfigSecret(ctx, b.SeedClientSet.Client(), osc, secretName, poolName)
	if err != nil {
		return fmt.Errorf("failed computing the OperatingSystemConfig secret for gardener-node-agent for pool %q: %w", poolName, err)
	}

	return b.SeedClientSet.Client().Create(ctx, b.operatingSystemConfigSecret)
}

func (b *AutonomousBotanist) appendAdminKubeconfigToFiles(files []extensionsv1alpha1.File) ([]extensionsv1alpha1.File, error) {
	userKubeconfigSecret, ok := b.SecretsManager.Get(kubeapiserver.SecretNameUserKubeconfig)
	if !ok {
		return nil, fmt.Errorf("failed fetching secret %q", kubeapiserver.SecretNameUserKubeconfig)
	}

	return append(files, extensionsv1alpha1.File{
		Path:        PathKubeconfig,
		Permissions: ptr.To[uint32](0600),
		Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(userKubeconfigSecret.Data[secretsutils.DataKeyKubeconfig])}},
	}), nil
}

func (b *AutonomousBotanist) deployOperatingSystemConfig(ctx context.Context) (*operatingsystemconfig.Data, string, error) {
	files, err := b.filesForStaticControlPlanePods(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed computing files for static control plane pods: %w", err)
	}

	files, err = b.appendAdminKubeconfigToFiles(files)
	if err != nil {
		return nil, "", fmt.Errorf("failed appending admin kubeconfig to list of files: %w", err)
	}

	if err := b.DeployOperatingSystemConfig(ctx); err != nil {
		return nil, "", fmt.Errorf("failed deploying OperatingSystemConfig resource: %w", err)
	}

	controlPlaneWorkerPool := v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo())
	if controlPlaneWorkerPool == nil {
		return nil, "", fmt.Errorf("failed fetching the control plane worker pool for the shoot")
	}

	oscData, ok := b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerPoolNameToOperatingSystemConfigsMap()[controlPlaneWorkerPool.Name]
	if !ok {
		return nil, "", fmt.Errorf("failed fetching the generated OperatingSystemConfig data for the control plane worker pool %q", controlPlaneWorkerPool.Name)
	}
	osc := oscData.Original.Object

	patch := client.MergeFrom(osc.DeepCopy())
	osc.Spec.Files = append(osc.Spec.Files, files...)
	if err := b.SeedClientSet.Client().Patch(ctx, osc, patch); err != nil {
		return nil, "", fmt.Errorf("failed patching OperatingSystemConfig with additional files for static control plane pods: %w", err)
	}

	return &oscData.Original, controlPlaneWorkerPool.Name, nil
}

// ApplyOperatingSystemConfig runs gardener-node-agent's reconciliation logic in order to apply the
// OperatingSystemConfig.
func (b *AutonomousBotanist) ApplyOperatingSystemConfig(ctx context.Context) error {
	if b.operatingSystemConfigSecret == nil {
		return fmt.Errorf("operating system config secret is nil, make sure to call createOperatingSystemConfigSecretForNodeAgent() first")
	}

	if err := b.ensureGardenerNodeAgentDirectories(); err != nil {
		return fmt.Errorf("failed ensuring gardener-node-agent directories exist: %w", err)
	}

	node, err := nodeagent.FetchNodeByHostName(ctx, b.SeedClientSet.Client(), b.HostName)
	if err != nil {
		return fmt.Errorf("failed fetching node object by hostname %q: %w", b.HostName, err)
	}

	reconcilerCtx, cancelFunc := context.WithCancel(ctx)
	reconcilerCtx = log.IntoContext(reconcilerCtx, b.Logger.WithName("operatingsystemconfig-reconciler").WithValues("secret", client.ObjectKeyFromObject(b.operatingSystemConfigSecret)))

	_, err = (&operatingsystemconfigcontroller.Reconciler{
		Client: b.SeedClientSet.Client(),
		Config: nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig{
			SyncPeriod:        &metav1.Duration{Duration: time.Minute},
			SecretName:        b.operatingSystemConfigSecret.Name,
			KubernetesVersion: b.Shoot.KubernetesVersion,
		},
		CancelContext:         cancelFunc,
		Recorder:              &record.FakeRecorder{},
		Extractor:             registry.NewExtractor(),
		HostName:              b.HostName,
		NodeName:              ptr.Deref(node, corev1.Node{}).Name,
		DBus:                  b.DBus,
		FS:                    b.FS,
		SkipWritingStateFiles: true,
	}).Reconcile(reconcilerCtx, reconcile.Request{NamespacedName: types.NamespacedName{Name: b.operatingSystemConfigSecret.Name, Namespace: b.operatingSystemConfigSecret.Namespace}})
	return err
}

func (b *AutonomousBotanist) ensureGardenerNodeAgentDirectories() error {
	if err := b.FS.MkdirAll(nodeagentconfigv1alpha1.TempDir, os.ModeDir); err != nil {
		return fmt.Errorf("failed creating temporary directory (%q): %w", nodeagentconfigv1alpha1.TempDir, err)
	}
	if err := b.FS.MkdirAll(nodeagentconfigv1alpha1.CredentialsDir, os.ModeDir); err != nil {
		return fmt.Errorf("failed creating credentials directory (%q): %w", nodeagentconfigv1alpha1.CredentialsDir, err)
	}
	return nil
}

// PrepareGardenerNodeInitConfiguration creates a Secret containing an OperatingSystemConfig with the gardener-node-init
// unit.
func (b *AutonomousBotanist) PrepareGardenerNodeInitConfiguration(ctx context.Context, secretName, controlPlaneAddress string, caBundle []byte, bootstrapToken string) error {
	osc, err := b.generateGardenerNodeInitOperatingSystemConfig(secretName, controlPlaneAddress, bootstrapToken, caBundle)
	if err != nil {
		return fmt.Errorf("failed computing units and files for gardener-node-init: %w", err)
	}

	return b.createOperatingSystemConfigSecretForNodeAgent(ctx, osc, secretName, "")
}

func (b *AutonomousBotanist) generateGardenerNodeInitOperatingSystemConfig(secretName, controlPlaneAddress, bootstrapToken string, caBundle []byte) (*extensionsv1alpha1.OperatingSystemConfig, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerNodeAgent)
	if err != nil {
		return nil, fmt.Errorf("failed finding image %q: %w", imagevector.ContainerImageNameGardenerNodeAgent, err)
	}
	image.WithOptionalTag(version.Get().GitVersion)

	units, files, err := nodeinit.Config(
		gardencorev1beta1.Worker{},
		image.String(),
		nodeagentcomponent.ComponentConfig(secretName, b.Shoot.KubernetesVersion, controlPlaneAddress, caBundle, nil),
	)
	if err != nil {
		return nil, fmt.Errorf("failed computing units and files for gardener-node-init: %w", err)
	}

	for i, file := range files {
		if file.Path == nodeagentconfigv1alpha1.BootstrapTokenFilePath {
			files[i].Content.Inline.Data = bootstrapToken
		}
		if file.Path == nodeagentconfigv1alpha1.MachineNameFilePath {
			files[i].Content.Inline.Data = b.HostName
		}
	}

	return &extensionsv1alpha1.OperatingSystemConfig{
		Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
			Files: files,
			Units: units,
		},
	}, nil
}

// IsGardenerNodeAgentInitialized returns true if the gardener-node-agent systemd unit exists.
func (b *AutonomousBotanist) IsGardenerNodeAgentInitialized(ctx context.Context) (bool, error) {
	unitStatuses, err := b.DBus.List(ctx)
	if err != nil {
		return false, fmt.Errorf("failed listing systemd units: %w", err)
	}

	if !slices.ContainsFunc(unitStatuses, func(status systemddbus.UnitStatus) bool {
		return status.Name == nodeagentconfigv1alpha1.UnitName
	}) {
		return false, nil
	}

	exists, err := b.FS.Exists(nodeagentconfigv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		return false, fmt.Errorf("failed checking whether bootstrap token file %s still exists: %w", nodeagentconfigv1alpha1.BootstrapTokenFilePath, err)
	}

	return !exists, nil
}

// ControlPlaneBootstrapOperatingSystemConfig creates the deployer for the OperatingSystemConfig custom resource that is
// used for bootstrapping control plane nodes in `gardenadm bootstrap`.
func (b *AutonomousBotanist) ControlPlaneBootstrapOperatingSystemConfig() (operatingsystemconfig.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenadm)
	if err != nil {
		return nil, fmt.Errorf("failed finding image %q: %w", imagevector.ContainerImageNameGardenadm, err)
	}
	image.WithOptionalTag(version.Get().GitVersion)

	worker := v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo())
	if worker == nil {
		return nil, fmt.Errorf("did not find the control plane worker pool of the shoot")
	}

	return operatingsystemconfig.NewControlPlaneBootstrap(
		b.Logger,
		b.SeedClientSet.Client(),
		&operatingsystemconfig.ControlPlaneBootstrapValues{
			Namespace:      b.Shoot.ControlPlaneNamespace,
			Worker:         worker,
			GardenadmImage: image.String(),
		},
		operatingsystemconfig.DefaultInterval,
		operatingsystemconfig.DefaultSevereThreshold,
		operatingsystemconfig.DefaultTimeout,
	), nil
}
