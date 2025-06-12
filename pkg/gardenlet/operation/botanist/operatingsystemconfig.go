// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/utils/flow"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// SecretLabelKeyManagedResource is a key for a label on a secret with the value 'managed-resource'.
const SecretLabelKeyManagedResource = "managed-resource"

// DefaultOperatingSystemConfig creates the default deployer for the OperatingSystemConfig custom resource.
func (b *Botanist) DefaultOperatingSystemConfig() (operatingsystemconfig.Interface, error) {
	oscImages, err := imagevectorutils.FindImages(imagevector.Containers(), []string{imagevector.ContainerImageNamePauseContainer, imagevector.ContainerImageNameValitail}, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	// This image is intentionally not part of the above FindImages call because this function defaults the image tag to
	// the ShootVersion (which is the Kubernetes version of the shoot cluster) in case the image tag in the image vector
	// is not set. This is true for gardener-node-agent because gardenlet always deploys it with its own version (ref
	// WithOptionalTag call a few lines below).
	// See also: https://github.com/gardener/gardener/issues/9577
	oscImages[imagevector.ContainerImageNameGardenerNodeAgent], err = imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerNodeAgent)
	if err != nil {
		return nil, fmt.Errorf("failed finding image %q: %w", imagevector.ContainerImageNameGardenerNodeAgent, err)
	}
	oscImages[imagevector.ContainerImageNameGardenerNodeAgent].WithOptionalTag(version.Get().GitVersion)

	valitailEnabled, valiIngressHost := false, ""
	if b.isShootNodeLoggingEnabled() {
		valitailEnabled, valiIngressHost = true, b.ComputeValiHost()
	}

	return operatingsystemconfig.New(
		b.Logger,
		b.SeedClientSet.Client(),
		b.SecretsManager,
		&operatingsystemconfig.Values{
			Namespace:         b.Shoot.ControlPlaneNamespace,
			KubernetesVersion: b.Shoot.KubernetesVersion,
			Workers:           b.Shoot.GetInfo().Spec.Provider.Workers,
			OriginalValues: operatingsystemconfig.OriginalValues{
				ClusterDomain:          gardencorev1beta1.DefaultDomain,
				Images:                 oscImages,
				KubeletConfig:          b.Shoot.GetInfo().Spec.Kubernetes.Kubelet,
				KubeProxyEnabled:       v1beta1helper.KubeProxyEnabled(b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy),
				MachineTypes:           b.Shoot.CloudProfile.Spec.MachineTypes,
				SSHAccessEnabled:       v1beta1helper.ShootEnablesSSHAccess(b.Shoot.GetInfo()),
				ValitailEnabled:        valitailEnabled,
				ValiIngressHostName:    valiIngressHost,
				NodeLocalDNSEnabled:    v1beta1helper.IsNodeLocalDNSEnabled(b.Shoot.GetInfo().Spec.SystemComponents),
				NodeMonitorGracePeriod: *b.Shoot.GetInfo().Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod,
				PrimaryIPFamily:        b.Shoot.GetInfo().Spec.Networking.IPFamilies[0],
			},
		},
		operatingsystemconfig.DefaultInterval,
		operatingsystemconfig.DefaultSevereThreshold,
		operatingsystemconfig.DefaultTimeout,
	), nil
}

// DeployOperatingSystemConfig deploys the OperatingSystemConfig custom resource and triggers the restore operation in
// case the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployOperatingSystemConfig(ctx context.Context) error {
	clusterCASecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}
	clusterCABundle, found := clusterCASecret.Data[secretsutils.DataKeyCertificateBundle]
	if !found {
		return fmt.Errorf("key %q not found in secret %q", secretsutils.DataKeyCertificateBundle, v1beta1constants.SecretNameCACluster)
	}

	b.Shoot.Components.Extensions.OperatingSystemConfig.SetAPIServerURL(fmt.Sprintf("https://%s", b.Shoot.ComputeOutOfClusterAPIServerAddress(true)))
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetCABundle(b.getOperatingSystemConfigCABundle(clusterCABundle))

	shoot := b.Shoot.GetInfo()
	if shoot.Status.Credentials != nil {
		b.Shoot.Components.Extensions.OperatingSystemConfig.SetCredentialsRotationStatus(shoot.Status.Credentials.Rotation)
	}

	if v1beta1helper.ShootEnablesSSHAccess(b.Shoot.GetInfo()) {
		sshKeypairSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
		}
		publicKeys := []string{string(sshKeypairSecret.Data[secretsutils.DataKeySSHAuthorizedKeys])}

		if sshKeypairSecretOld, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair, secretsmanager.Old); found {
			publicKeys = append(publicKeys, string(sshKeypairSecretOld.Data[secretsutils.DataKeySSHAuthorizedKeys]))
		}

		b.Shoot.Components.Extensions.OperatingSystemConfig.SetSSHPublicKeys(publicKeys)
	}

	var clusterDNSAddresses []string
	for _, ip := range b.Shoot.Networks.CoreDNS {
		clusterDNSAddresses = append(clusterDNSAddresses, ip.String())
	}
	if b.Shoot.NodeLocalDNSEnabled && b.Shoot.IPVSEnabled() {
		// If IPVS is enabled then instruct the kubelet to create pods resolving DNS to the `nodelocaldns` network
		// interface link-local ip address. For more information checkout the usage documentation under
		// https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/.
		ipFamiliesSet := sets.New(b.Shoot.GetInfo().Spec.Networking.IPFamilies...)
		if ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv4) && !ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv6) {
			clusterDNSAddresses = []string{nodelocaldnsconstants.IPVSAddress}
		}
		if ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv6) && !ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv4) {
			clusterDNSAddresses = []string{nodelocaldnsconstants.IPVSIPv6Address}
		}
	}
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetClusterDNSAddresses(clusterDNSAddresses)

	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.OperatingSystemConfig.Restore(ctx, b.Shoot.GetShootState())
	}

	return b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy(ctx)
}

func (b *Botanist) getOperatingSystemConfigCABundle(clusterCABundle []byte) string {
	caBundle := string(clusterCABundle)

	if cloudProfileCaBundle := b.Shoot.CloudProfile.Spec.CABundle; cloudProfileCaBundle != nil {
		caBundle = fmt.Sprintf("%s\n%s", *cloudProfileCaBundle, caBundle)
	}

	return caBundle
}

// exposed for testing
var (
	// NodeAgentOSCSecretFn is a function for computing the operating system config secret for gardener-node-agent.
	NodeAgentOSCSecretFn = nodeagent.OperatingSystemConfigSecret
	// NodeAgentRBACResourcesDataFn is a function for generating the RBAC resources data map for the
	// gardener-node-agent.
	NodeAgentRBACResourcesDataFn = nodeagent.RBACResourcesData
)

// GardenerNodeAgentManagedResourceName is a constant for the name of a ManagedResource in the seed cluster in the shoot
// namespace which contains resources for gardener-node-agent.
const GardenerNodeAgentManagedResourceName = "shoot-gardener-node-agent"

// DeployManagedResourceForGardenerNodeAgent creates the ManagedResource that contains:
// - A secret containing the raw original OperatingSystemConfig for each worker pool.
// - A secret containing some shared RBAC resources for downloading the OSC secrets + bootstrapping the node.
func (b *Botanist) DeployManagedResourceForGardenerNodeAgent(ctx context.Context) error {
	var (
		managedResource                  = managedresources.NewForShoot(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, GardenerNodeAgentManagedResourceName, managedresources.LabelValueGardener, false)
		managedResourceSecretsCount      = len(b.Shoot.GetInfo().Spec.Provider.Workers) + 1
		managedResourceSecretLabels      = map[string]string{SecretLabelKeyManagedResource: GardenerNodeAgentManagedResourceName}
		managedResourceSecretNamesWanted = sets.New[string]()
		managedResourceSecretNameToData  = make(map[string]map[string][]byte, managedResourceSecretsCount)

		secretNames                           []string
		workerNameToOperatingSystemConfigMaps = b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerPoolNameToOperatingSystemConfigsMap()

		fns = make([]flow.TaskFn, 0, managedResourceSecretsCount)
	)

	// Generate operating system config secrets for all worker pools.
	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		oscData, ok := workerNameToOperatingSystemConfigMaps[worker.Name]
		if !ok {
			return fmt.Errorf("did not find osc data for worker pool %q", worker.Name)
		}

		secretName, data, err := b.generateOperatingSystemConfigSecretForWorker(ctx, worker, oscData.Original)
		if err != nil {
			return err
		}

		secretNames = append(secretNames, secretName)
		managedResourceSecretNameToData["shoot-gardener-node-agent-"+worker.Name] = data
	}

	rbacResourcesData, err := NodeAgentRBACResourcesDataFn(secretNames)
	if err != nil {
		return err
	}
	managedResourceSecretNameToData["shoot-gardener-node-agent-rbac"] = rbacResourcesData

	// Create Secrets for the ManagedResource containing the configs for all worker pools well as the RBAC resources.
	for secretName, data := range managedResourceSecretNameToData {
		var (
			keyValues                                        = data
			managedResourceSecretName, managedResourceSecret = managedresources.NewSecret(
				b.SeedClientSet.Client(),
				b.Shoot.ControlPlaneNamespace,
				secretName,
				keyValues,
				true,
			)
		)

		managedResource.WithSecretRef(managedResourceSecretName)
		managedResourceSecretNamesWanted.Insert(managedResourceSecretName)

		fns = append(fns, func(ctx context.Context) error {
			return managedResourceSecret.
				AddLabels(managedResourceSecretLabels).
				Reconcile(ctx)
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return err
	}

	if err := managedResource.Reconcile(ctx); err != nil {
		return err
	}

	// Cleanup no longer required Secrets for the ManagedResource (e.g., those for removed worker pools).
	secretList := &corev1.SecretList{}
	if err := b.SeedClientSet.Client().List(ctx, secretList, client.InNamespace(b.Shoot.ControlPlaneNamespace), client.MatchingLabels(managedResourceSecretLabels)); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjectsFromListConditionally(ctx, b.SeedClientSet.Client(), secretList, func(obj runtime.Object) bool {
		acc, err := meta.Accessor(obj)
		if err != nil {
			return false
		}
		return !managedResourceSecretNamesWanted.Has(acc.GetName())
	})
}

func (b *Botanist) generateOperatingSystemConfigSecretForWorker(
	ctx context.Context,
	worker gardencorev1beta1.Worker,
	oscDataOriginal operatingsystemconfig.Data,
) (
	string,
	map[string][]byte,
	error,
) {
	oscSecret, err := NodeAgentOSCSecretFn(ctx, b.SeedClientSet.Client(), oscDataOriginal.Object, oscDataOriginal.GardenerNodeAgentSecretName, worker.Name)
	if err != nil {
		return "", nil, fmt.Errorf("failed computing the OperatingSystemConfig secret for gardener-node-agent for pool %q: %w", worker.Name, err)
	}

	resources, err := managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(oscSecret)
	if err != nil {
		return "", nil, fmt.Errorf("failed adding gardener-node-agent secret for pool %q to the registry and serializing it: %w", worker.Name, err)
	}

	return oscSecret.Name, resources, nil
}
