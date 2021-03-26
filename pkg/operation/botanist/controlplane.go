// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	extensionscontrolplane "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/konnectivity"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/version"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var chartPathControlPlane = filepath.Join(charts.Path, "seed-controlplane", "charts")

// EnsureClusterIdentity ensures that Shoot cluster-identity ConfigMap exists and stores its data
// in the operation. Updates shoot.status.clusterIdentity if it doesn't exist already.
func (b *Botanist) EnsureClusterIdentity(ctx context.Context) error {
	if err := b.Shoot.Components.ClusterIdentity.Deploy(ctx); err != nil {
		return err
	}

	latestShoot := &gardencorev1beta1.Shoot{}
	if err := b.K8sGardenClient.APIReader().Get(ctx, kutil.Key(b.Shoot.Info.Namespace, b.Shoot.Info.Name), latestShoot); err != nil {
		return err
	}

	b.Shoot.Info = latestShoot
	return nil
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer(ctx context.Context) error {
	// invalidate shoot client here before deleting API server
	if err := b.ClientMap.InvalidateClient(keys.ForShoot(b.Shoot.Info)); err != nil {
		return err
	}
	b.K8sShootClient = nil

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameKubeAPIServer,
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptions...))
}

// DeployVerticalPodAutoscaler deploys the VPA into the shoot namespace in the seed.
func (b *Botanist) DeployVerticalPodAutoscaler(ctx context.Context) error {
	if !b.Shoot.WantsVerticalPodAutoscaler {
		return common.DeleteVpa(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, true)
	}

	var (
		podLabels = map[string]interface{}{
			v1beta1constants.LabelNetworkPolicyToDNS:            "allowed",
			v1beta1constants.LabelNetworkPolicyToShootAPIServer: "allowed",
		}
		admissionController = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-vpa-tls-certs":            b.CheckSums[common.VPASecretName],
				"checksum/secret-vpa-admission-controller": b.CheckSums["vpa-admission-controller"],
			},
			"podLabels": utils.MergeMaps(podLabels, map[string]interface{}{
				v1beta1constants.LabelNetworkPolicyFromShootAPIServer: "allowed",
			}),
			"enableServiceAccount": false,
		}
		exporter = map[string]interface{}{
			"enabled":  false,
			"replicas": 0,
		}
		recommender = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-vpa-recommender": b.CheckSums["vpa-recommender"],
			},
			"podLabels":                    podLabels,
			"enableServiceAccount":         false,
			"recommendationMarginFraction": gardencorev1beta1.DefaultRecommendationMarginFraction,
			"interval":                     gardencorev1beta1.DefaultRecommenderInterval,
		}
		updater = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-vpa-updater": b.CheckSums["vpa-updater"],
			},
			"podLabels":              podLabels,
			"enableServiceAccount":   false,
			"evictAfterOOMThreshold": gardencorev1beta1.DefaultEvictAfterOOMThreshold,
			"evictionRateBurst":      gardencorev1beta1.DefaultEvictionRateBurst,
			"evictionRateLimit":      gardencorev1beta1.DefaultEvictionRateLimit,
			"evictionTolerance":      gardencorev1beta1.DefaultEvictionTolerance,
			"interval":               gardencorev1beta1.DefaultUpdaterInterval,
		}
		defaultValues = map[string]interface{}{
			"admissionController": admissionController,
			"exporter":            exporter,
			"recommender":         recommender,
			"updater":             updater,
			"deploymentLabels": map[string]interface{}{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			},
			"clusterType": "shoot",
		}
	)

	if verticalPodAutoscaler := b.Shoot.Info.Spec.Kubernetes.VerticalPodAutoscaler; verticalPodAutoscaler != nil {
		if val := verticalPodAutoscaler.EvictAfterOOMThreshold; val != nil {
			updater["evictAfterOOMThreshold"] = *val
		}
		if val := verticalPodAutoscaler.EvictionRateBurst; val != nil {
			updater["evictionRateBurst"] = *val
		}
		if val := verticalPodAutoscaler.EvictionRateLimit; val != nil {
			updater["evictionRateLimit"] = *val
		}
		if val := verticalPodAutoscaler.EvictionTolerance; val != nil {
			updater["evictionTolerance"] = *val
		}
		if val := verticalPodAutoscaler.UpdaterInterval; val != nil {
			updater["interval"] = *val
		}
		if val := verticalPodAutoscaler.RecommendationMarginFraction; val != nil {
			recommender["recommendationMarginFraction"] = *val
		}
		if val := verticalPodAutoscaler.RecommenderInterval; val != nil {
			recommender["interval"] = *val
		}
	}

	values, err := b.InjectSeedShootImages(defaultValues, charts.ImageNameVpaAdmissionController, charts.ImageNameVpaExporter, charts.ImageNameVpaRecommender, charts.ImageNameVpaUpdater)
	if err != nil {
		return err
	}
	values["global"] = map[string]interface{}{"images": values["images"]}

	return b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "seed-bootstrap", "charts", "vpa", "charts", "runtime"), b.Shoot.SeedNamespace, "vpa", kubernetes.Values(values))
}

// WakeUpKubeAPIServer creates a service and ensures API Server is scaled up
func (b *Botanist) WakeUpKubeAPIServer(ctx context.Context) error {
	sniPhase := b.Shoot.Components.ControlPlane.KubeAPIServerSNIPhase.Done()

	if err := b.DeployKubeAPIService(ctx, sniPhase); err != nil {
		return err
	}
	if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Wait(ctx); err != nil {
		return err
	}
	if b.APIServerSNIEnabled() {
		if err := b.DeployKubeAPIServerSNI(ctx); err != nil {
			return err
		}
	}
	if err := b.DeployKubeAPIServer(ctx); err != nil {
		return err
	}
	if err := kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), 1); err != nil {
		return err
	}
	if err := b.WaitUntilKubeAPIServerReady(ctx); err != nil {
		return err
	}

	return nil
}

// HibernateControlPlane hibernates the entire control plane if the shoot shall be hibernated.
func (b *Botanist) HibernateControlPlane(ctx context.Context) error {
	if b.K8sShootClient != nil {
		ctxWithTimeOut, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		// If a shoot is hibernated we only want to scale down the entire control plane if no nodes exist anymore. The node-lifecycle-controller
		// inside KCM is responsible for deleting Node objects of terminated/non-existing VMs, so let's wait for that before scaling down.
		if err := b.WaitUntilNodesDeleted(ctxWithTimeOut); err != nil {
			return err
		}

		// Also wait for all Pods to reflect the correct state before scaling down the control plane.
		// KCM should remove all Pods in the cluster that are bound to Nodes that no longer exist and
		// therefore there should be no Pods with state `Running` anymore.
		if err := b.WaitUntilNoPodRunning(ctxWithTimeOut); err != nil {
			return err
		}

		// Also wait for all Endpoints to not contain any IPs from the Shoot's PodCIDR.
		// This is to make sure that the Endpoints objects also reflect the correct state of the hibernated cluster.
		// Otherwise this could cause timeouts in user-defined webhooks for CREATE Pods or Nodes on wakeup.
		if err := b.WaitUntilEndpointsDoNotContainPodIPs(ctxWithTimeOut); err != nil {
			return err
		}

		// TODO: remove this mitigation once there is a garbage collection for VolumeAttachments (ref https://github.com/kubernetes/kubernetes/issues/77324)
		// Currently on hibernation Machines are forecefully deleted and machine-controller-manager does not wait volumes to be detached.
		// In this case kube-controller-manager cannot delete the corresponding VolumeAttachment objects and they are orphaned.
		// Such orphaned VolumeAttachments then prevent/block PV deletion. For more details see https://github.com/gardener/gardener-extension-provider-gcp/issues/172.
		// As the Nodes are already deleted, we can delete all VolumeAttachments.
		if err := DeleteVolumeAttachments(ctxWithTimeOut, b.K8sShootClient.Client()); err != nil {
			return err
		}

		if err := WaitUntilVolumeAttachmentsDeleted(ctxWithTimeOut, b.K8sShootClient.Client(), b.Logger); err != nil {
			return err
		}
	}

	// invalidate shoot client here before scaling down API server
	if err := b.ClientMap.InvalidateClient(keys.ForShoot(b.Shoot.Info)); err != nil {
		return err
	}
	b.K8sShootClient = nil

	deployments := []string{
		v1beta1constants.DeploymentNameGardenerResourceManager,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeAPIServer,
	}
	for _, deployment := range deployments {
		if err := kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, deployment), 0); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if err := b.K8sSeedClient.Client().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}}, kubernetes.DefaultDeleteOptions...); err != nil {
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return err
		}
	}

	if !b.Shoot.DisableDNS && !b.APIServerSNIEnabled() {
		if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Destroy(ctx); err != nil {
			return err
		}

		if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.WaitCleanup(ctx); err != nil {
			return err
		}
	}

	if err := b.Shoot.Components.ControlPlane.KubeAPIServerSNI.Destroy(ctx); err != nil {
		return err
	}

	return client.IgnoreNotFound(b.ScaleETCDToZero(ctx))
}

// ScaleKubeAPIServerToOne scales kube-apiserver replicas to one
func (b *Botanist) ScaleKubeAPIServerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), 1)
}

// ScaleGardenerResourceManagerToOne scales the gardener-resource-manager deployment
func (b *Botanist) ScaleGardenerResourceManagerToOne(ctx context.Context) error {
	return kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameGardenerResourceManager), 1)
}

// PrepareKubeAPIServerForMigration deletes the kube-apiserver and deletes its hvpa
func (b *Botanist) PrepareKubeAPIServerForMigration(ctx context.Context) error {
	if err := b.K8sSeedClient.Client().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}}); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
		return err
	}

	return b.DeleteKubeAPIServer(ctx)
}

// DefaultControlPlane creates the default deployer for the ControlPlane custom resource with the given purpose.
func (b *Botanist) DefaultControlPlane(seedClient client.Client, purpose extensionsv1alpha1.Purpose) extensionscontrolplane.Interface {
	values := &extensionscontrolplane.Values{
		Name:      b.Shoot.Info.Name,
		Namespace: b.Shoot.SeedNamespace,
		Purpose:   purpose,
	}

	switch purpose {
	case extensionsv1alpha1.Normal:
		values.Type = b.Shoot.Info.Spec.Provider.Type
		values.ProviderConfig = b.Shoot.Info.Spec.Provider.ControlPlaneConfig
		values.Region = b.Shoot.Info.Spec.Region

	case extensionsv1alpha1.Exposure:
		values.Type = b.Seed.Info.Spec.Provider.Type
		values.Region = b.Seed.Info.Spec.Provider.Region
	}

	return extensionscontrolplane.New(
		b.Logger,
		seedClient,
		values,
		extensionscontrolplane.DefaultInterval,
		extensionscontrolplane.DefaultSevereThreshold,
		extensionscontrolplane.DefaultTimeout,
	)
}

// DeployControlPlane deploys or restores the ControlPlane custom resource (purpose normal).
func (b *Botanist) DeployControlPlane(ctx context.Context) error {
	b.Shoot.Components.Extensions.ControlPlane.SetInfrastructureProviderStatus(b.Shoot.Components.Extensions.Infrastructure.ProviderStatus())
	return b.deployOrRestoreControlPlane(ctx, b.Shoot.Components.Extensions.ControlPlane)
}

// DeployControlPlaneExposure deploys or restores the ControlPlane custom resource (purpose exposure).
func (b *Botanist) DeployControlPlaneExposure(ctx context.Context) error {
	return b.deployOrRestoreControlPlane(ctx, b.Shoot.Components.Extensions.ControlPlaneExposure)
}

func (b *Botanist) deployOrRestoreControlPlane(ctx context.Context, controlPlane extensionscontrolplane.Interface) error {
	if b.isRestorePhase() {
		return controlPlane.Restore(ctx, b.ShootState)
	}
	return controlPlane.Deploy(ctx)
}

const (
	auditPolicyConfigMapDataKey = "policy"
)

// getResourcesForAPIServer returns the cpu and memory requirements for API server based on nodeCount
func getResourcesForAPIServer(nodeCount int32, scalingClass string) (string, string, string, string) {
	var (
		validScalingClasses = sets.NewString("small", "medium", "large", "xlarge", "2xlarge")
		cpuRequest          string
		memoryRequest       string
		cpuLimit            string
		memoryLimit         string
	)

	if !validScalingClasses.Has(scalingClass) {
		switch {
		case nodeCount <= 2:
			scalingClass = "small"
		case nodeCount <= 10:
			scalingClass = "medium"
		case nodeCount <= 50:
			scalingClass = "large"
		case nodeCount <= 100:
			scalingClass = "xlarge"
		default:
			scalingClass = "2xlarge"
		}
	}

	switch {
	case scalingClass == "small":
		cpuRequest = "800m"
		memoryRequest = "800Mi"

		cpuLimit = "1000m"
		memoryLimit = "1200Mi"
	case scalingClass == "medium":
		cpuRequest = "1000m"
		memoryRequest = "1100Mi"

		cpuLimit = "1200m"
		memoryLimit = "1900Mi"
	case scalingClass == "large":
		cpuRequest = "1200m"
		memoryRequest = "1600Mi"

		cpuLimit = "1500m"
		memoryLimit = "3900Mi"
	case scalingClass == "xlarge":
		cpuRequest = "2500m"
		memoryRequest = "5200Mi"

		cpuLimit = "3000m"
		memoryLimit = "5900Mi"
	case scalingClass == "2xlarge":
		cpuRequest = "3000m"
		memoryRequest = "5200Mi"

		cpuLimit = "4000m"
		memoryLimit = "7800Mi"
	}

	return cpuRequest, memoryRequest, cpuLimit, memoryLimit
}

func (b *Botanist) deployNetworkPolicies(ctx context.Context, denyAll bool) error {
	var (
		globalNetworkPoliciesValues = map[string]interface{}{
			"blockedAddresses":     b.Seed.Info.Spec.Networks.BlockCIDRs,
			"denyAll":              denyAll,
			"dnsServer":            b.Shoot.Networks.CoreDNS.String(),
			"nodeLocalIPVSAddress": NodeLocalIPVSAddress,
			"nodeLocalDNSEnabled":  b.Shoot.NodeLocalDNSEnabled,
		}
		excludeNets = []string{}
		values      = map[string]interface{}{}
	)

	switch b.Shoot.Components.ControlPlane.KubeAPIServerSNIPhase { // nolint:exhaustive
	case component.PhaseEnabled, component.PhaseEnabling, component.PhaseDisabling:
		// Enable network policies for SNI
		// When disabling SNI (previously enabled), the control plane is transitioning between states, thus
		// it needs to be ensured that the traffic from old clients can still reach the API server.
		globalNetworkPoliciesValues["sniEnabled"] = true
	default:
		globalNetworkPoliciesValues["sniEnabled"] = false
	}

	excludeNets = append(excludeNets, b.Seed.Info.Spec.Networks.BlockCIDRs...)

	var shootCIDRNetworks []string
	if v := b.Shoot.GetNodeNetwork(); v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	if v := b.Shoot.Info.Spec.Networking.Pods; v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	if v := b.Shoot.Info.Spec.Networking.Services; v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	shootNetworkValues, err := common.ExceptNetworks(shootCIDRNetworks, excludeNets...)
	if err != nil {
		return err
	}
	values["clusterNetworks"] = shootNetworkValues

	allCIDRNetworks := []string{b.Seed.Info.Spec.Networks.Pods, b.Seed.Info.Spec.Networks.Services}
	if v := b.Seed.Info.Spec.Networks.Nodes; v != nil {
		allCIDRNetworks = append(allCIDRNetworks, *v)
	}
	allCIDRNetworks = append(allCIDRNetworks, shootCIDRNetworks...)
	allCIDRNetworks = append(allCIDRNetworks, excludeNets...)

	privateNetworks, err := common.ToExceptNetworks(common.AllPrivateNetworkBlocks(), allCIDRNetworks...)
	if err != nil {
		return err
	}

	globalNetworkPoliciesValues["privateNetworks"] = privateNetworks
	values["global-network-policies"] = globalNetworkPoliciesValues

	return b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(chartPathControlPlane, "network-policies"), b.Shoot.SeedNamespace, "network-policies", kubernetes.Values(values))
}

// DeployNetworkPolicies creates a network policies in a Shoot cluster's namespace that
// deny all traffic and allow certain components to use annotations to declare their desire
// to transmit/receive traffic to/from other Pods/IP addresses.
func (b *Botanist) DeployNetworkPolicies(ctx context.Context) error {
	return b.deployNetworkPolicies(ctx, true)
}

// DeployKubeAPIServer deploys kube-apiserver deployment.
func (b *Botanist) DeployKubeAPIServer(ctx context.Context) error {
	var (
		hvpaEnabled               = gardenletfeatures.FeatureGate.Enabled(features.HVPA)
		mountHostCADirectories    = gardenletfeatures.FeatureGate.Enabled(features.MountHostCADirectories)
		memoryMetricForHpaEnabled = false
	)

	if b.ManagedSeed != nil {
		// Override for shooted seeds
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
		memoryMetricForHpaEnabled = true
	}

	var (
		podAnnotations = map[string]interface{}{
			"checksum/secret-ca":                     b.CheckSums[v1beta1constants.SecretNameCACluster],
			"checksum/secret-ca-front-proxy":         b.CheckSums[v1beta1constants.SecretNameCAFrontProxy],
			"checksum/secret-kube-apiserver":         b.CheckSums[v1beta1constants.DeploymentNameKubeAPIServer],
			"checksum/secret-kube-aggregator":        b.CheckSums["kube-aggregator"],
			"checksum/secret-kube-apiserver-kubelet": b.CheckSums["kube-apiserver-kubelet"],
			"checksum/secret-static-token":           b.CheckSums[common.StaticTokenSecretName],
			"checksum/secret-service-account-key":    b.CheckSums["service-account-key"],
			"checksum/secret-etcd-ca":                b.CheckSums[etcd.SecretNameCA],
			"checksum/secret-etcd-client-tls":        b.CheckSums[etcd.SecretNameClient],
			"networkpolicy/konnectivity-enabled":     strconv.FormatBool(b.Shoot.KonnectivityTunnelEnabled),
		}
		defaultValues = map[string]interface{}{
			"etcdServicePort":           etcd.PortEtcdClient,
			"kubernetesVersion":         b.Shoot.Info.Spec.Kubernetes.Version,
			"priorityClassName":         v1beta1constants.PriorityClassNameShootControlPlane,
			"enableBasicAuthentication": gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info),
			"probeCredentials":          b.APIServerHealthCheckToken,
			"securePort":                443,
			"podAnnotations":            podAnnotations,
			"konnectivityTunnel": map[string]interface{}{
				"enabled":    b.Shoot.KonnectivityTunnelEnabled,
				"name":       konnectivity.ServerName,
				"serverPort": konnectivity.ServerHTTPSPort,
			},
			"hvpa": map[string]interface{}{
				"enabled": hvpaEnabled,
			},
			"hpa": map[string]interface{}{
				"memoryMetricForHpaEnabled": memoryMetricForHpaEnabled,
			},
		}
		minReplicas int32 = 1
		maxReplicas int32 = 4

		shootNetworks = map[string]interface{}{
			"services": b.Shoot.Networks.Services.String(),
			"pods":     b.Shoot.Networks.Pods.String(),
		}
	)

	if b.Shoot.KonnectivityTunnelEnabled {
		if b.APIServerSNIEnabled() {
			podAnnotations["checksum/secret-"+konnectivity.SecretNameServerTLSClient] = b.CheckSums[konnectivity.SecretNameServerTLSClient]
		} else {
			podAnnotations["checksum/secret-konnectivity-server"] = b.CheckSums[konnectivity.ServerName]
		}
	} else {
		podAnnotations["checksum/secret-vpn-seed"] = b.CheckSums["vpn-seed"]
		podAnnotations["checksum/secret-vpn-seed-tlsauth"] = b.CheckSums["vpn-seed-tlsauth"]
	}

	if v := b.Shoot.GetNodeNetwork(); v != nil {
		shootNetworks["nodes"] = *v
	}
	defaultValues["shootNetworks"] = shootNetworks

	if b.APIServerSNIEnabled() {
		defaultValues["sni"] = map[string]interface{}{
			"enabled":           true,
			"advertiseIP":       b.APIServerClusterIP,
			"apiserverFQDN":     b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true),
			"podMutatorEnabled": b.APIServerSNIPodMutatorEnabled(),
		}
	}

	enableEtcdEncryption, err := version.CheckVersionMeetsConstraint(b.Shoot.Info.Spec.Kubernetes.Version, ">= 1.13")
	if err != nil {
		return err
	}
	if enableEtcdEncryption {
		defaultValues["enableEtcdEncryption"] = true
		defaultValues["podAnnotations"].(map[string]interface{})["checksum/secret-etcd-encryption"] = b.CheckSums[common.EtcdEncryptionSecretName]
	}

	if gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info) {
		defaultValues["podAnnotations"].(map[string]interface{})["checksum/secret-"+common.BasicAuthSecretName] = b.CheckSums[common.BasicAuthSecretName]
	}

	foundDeployment := true
	deployment := &appsv1.Deployment{}
	if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		foundDeployment = false
	}

	if b.ManagedSeed != nil && b.ManagedSeedAPIServer != nil {
		autoscaler := b.ManagedSeedAPIServer.Autoscaler
		minReplicas = *autoscaler.MinReplicas
		maxReplicas = autoscaler.MaxReplicas
	}

	if b.Shoot.Purpose == gardencorev1beta1.ShootPurposeProduction {
		minReplicas = 2
	}

	if b.ManagedSeed != nil && b.ManagedSeedAPIServer != nil && !hvpaEnabled {
		defaultValues["replicas"] = *b.ManagedSeedAPIServer.Replicas
		defaultValues["apiServerResources"] = map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    "1750m",
				"memory": "2Gi",
			},
			"limits": map[string]interface{}{
				"cpu":    "4000m",
				"memory": "8Gi",
			},
		}
	} else {
		replicas := deployment.Spec.Replicas

		// As kube-apiserver HPA manages the number of replicas, we have to maintain current number of replicas
		// otherwise keep the value to default
		if replicas != nil && *replicas > 0 {
			defaultValues["replicas"] = *replicas
		}
		// If the shoot is hibernated then we want to keep the number of replicas (scale down happens later).
		if b.Shoot.HibernationEnabled && (replicas == nil || *replicas == 0) {
			defaultValues["replicas"] = 0
		}

		var cpuRequest, memoryRequest, cpuLimit, memoryLimit string
		if hvpaEnabled {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(b.Shoot.GetMinNodeCount(), b.Shoot.Info.Annotations[common.ShootAlphaScalingAPIServerClass])
		} else {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(b.Shoot.GetMaxNodeCount(), b.Shoot.Info.Annotations[common.ShootAlphaScalingAPIServerClass])
		}
		defaultValues["apiServerResources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    cpuLimit,
				"memory": memoryLimit,
			},
			"requests": map[string]interface{}{
				"cpu":    cpuRequest,
				"memory": memoryRequest,
			},
		}
	}

	if foundDeployment && hvpaEnabled {
		// Deployment is already created AND is controlled by HVPA
		// Keep the "resources" as it is.
		for k := range deployment.Spec.Template.Spec.Containers {
			v := &deployment.Spec.Template.Spec.Containers[k]
			if v.Name == "kube-apiserver" {
				defaultValues["apiServerResources"] = v.Resources.DeepCopy()
				break
			}
		}
	}

	defaultValues["minReplicas"] = minReplicas
	defaultValues["maxReplicas"] = maxReplicas

	var (
		apiServerConfig              = b.Shoot.Info.Spec.Kubernetes.KubeAPIServer
		admissionPlugins             = kubernetes.GetAdmissionPluginsForVersion(b.Shoot.Info.Spec.Kubernetes.Version)
		serviceAccountTokenIssuerURL = fmt.Sprintf("https://%s", b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true))
		serviceAccountConfigVals     = map[string]interface{}{}
	)

	if apiServerConfig != nil {
		defaultValues["featureGates"] = apiServerConfig.FeatureGates
		defaultValues["runtimeConfig"] = apiServerConfig.RuntimeConfig

		if apiServerConfig.OIDCConfig != nil {
			defaultValues["oidcConfig"] = apiServerConfig.OIDCConfig
		}

		if serviceAccountConfig := apiServerConfig.ServiceAccountConfig; serviceAccountConfig != nil {
			if issuer := serviceAccountConfig.Issuer; issuer != nil {
				serviceAccountTokenIssuerURL = *issuer
			}

			if signingKeySecret := serviceAccountConfig.SigningKeySecret; signingKeySecret != nil {
				signingKey, err := common.GetServiceAccountSigningKeySecret(ctx, b.K8sGardenClient.Client(), b.Shoot.Info.Namespace, signingKeySecret.Name)
				if err != nil {
					return err
				}

				serviceAccountConfigVals["signingKey"] = signingKey
			}
		}

		if apiServerConfig.APIAudiences != nil {
			defaultValues["apiAudiences"] = apiServerConfig.APIAudiences
		}

		for _, plugin := range apiServerConfig.AdmissionPlugins {
			pluginOverwritesDefault := false

			for i, defaultPlugin := range admissionPlugins {
				if defaultPlugin.Name == plugin.Name {
					pluginOverwritesDefault = true
					admissionPlugins[i] = plugin
					break
				}
			}

			if !pluginOverwritesDefault {
				admissionPlugins = append(admissionPlugins, plugin)
			}
		}

		if apiServerConfig.AuditConfig != nil &&
			apiServerConfig.AuditConfig.AuditPolicy != nil &&
			apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil {

			auditPolicy, err := b.getAuditPolicy(ctx, apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name, b.Shoot.Info.Namespace)
			if err != nil {
				// Ignore missing audit configuration on shoot deletion to prevent failing redeployments of the
				// kube-apiserver in case the end-user deleted the configmap before/simultaneously to the shoot
				// deletion.
				if !apierrors.IsNotFound(err) || b.Shoot.Info.DeletionTimestamp == nil {
					return fmt.Errorf("retrieving audit policy from the ConfigMap '%v' failed with reason '%v'", apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name, err)
				}
			} else {
				defaultValues["auditConfig"] = map[string]interface{}{
					"auditPolicy": auditPolicy,
				}
			}
		}

		if watchCacheSizes := apiServerConfig.WatchCacheSizes; watchCacheSizes != nil {
			defaultValues["watchCacheSizes"] = watchCacheSizes
		}

		if apiServerConfig.Requests != nil {
			if v := apiServerConfig.Requests.MaxNonMutatingInflight; v != nil {
				defaultValues["maxNonMutatingRequestsInflight"] = *v
			}
			if v := apiServerConfig.Requests.MaxMutatingInflight; v != nil {
				defaultValues["maxMutatingRequestsInflight"] = *v
			}
		}
	}

	serviceAccountConfigVals["issuer"] = serviceAccountTokenIssuerURL
	defaultValues["serviceAccountConfig"] = serviceAccountConfigVals
	defaultValues["admissionPlugins"] = admissionPlugins

	defaultValues["mountHostCADirectories"] = map[string]interface{}{
		"enabled": mountHostCADirectories,
	}

	tunnelComponentImageName := charts.ImageNameVpnSeed
	if b.Shoot.KonnectivityTunnelEnabled {
		tunnelComponentImageName = charts.ImageNameKonnectivityServer
	}

	values, err := b.InjectSeedShootImages(defaultValues,
		tunnelComponentImageName,
		charts.ImageNameKubeApiserver,
		charts.ImageNameAlpineIptables,
		charts.ImageNameApiserverProxyPodWebhook,
	)
	if err != nil {
		return err
	}

	// If HVPA feature gate is enabled then we should delete the old HPA and VPA resources as
	// the HVPA controller will create its own for the kube-apiserver deployment.
	if hvpaEnabled {
		objects := []client.Object{
			&autoscalingv1beta2.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: b.Shoot.SeedNamespace,
					Name:      v1beta1constants.DeploymentNameKubeAPIServer + "-vpa",
				},
			},
		}

		seedVersionGE112, err := version.CompareVersions(b.K8sSeedClient.Version(), ">=", "1.12")
		if err != nil {
			return err
		}

		hpaObjectMeta := kutil.ObjectMeta(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer)

		// autoscaling/v2beta1 is deprecated in favor of autoscaling/v2beta2 beginning with v1.19
		// ref https://github.com/kubernetes/kubernetes/pull/90463
		if seedVersionGE112 {
			objects = append(objects, &autoscalingv2beta2.HorizontalPodAutoscaler{ObjectMeta: hpaObjectMeta})
		} else {
			objects = append(objects, &autoscalingv2beta1.HorizontalPodAutoscaler{ObjectMeta: hpaObjectMeta})
		}

		if err := kutil.DeleteObjects(ctx, b.K8sSeedClient.Client(), objects...); err != nil {
			return err
		}
	} else {
		// If HVPA is disabled, delete any HVPA that was already deployed
		hvpa := &hvpav1alpha1.Hvpa{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      v1beta1constants.DeploymentNameKubeAPIServer,
			},
		}
		if err := b.K8sSeedClient.Client().Delete(ctx, hvpa); err != nil {
			if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
				return err
			}
		}
	}

	return b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(chartPathControlPlane, v1beta1constants.DeploymentNameKubeAPIServer), b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer, kubernetes.Values(values))
}

func (b *Botanist) getAuditPolicy(ctx context.Context, name, namespace string) (string, error) {
	auditPolicyCm := &corev1.ConfigMap{}
	if err := b.K8sGardenClient.APIReader().Get(ctx, kutil.Key(namespace, name), auditPolicyCm); err != nil {
		return "", err
	}
	auditPolicy, ok := auditPolicyCm.Data[auditPolicyConfigMapDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data.policy' in audit policy configmap %v/%v", namespace, name)
	}
	return auditPolicy, nil
}

// DefaultKubeAPIServerService returns a deployer for kube-apiserver service.
func (b *Botanist) DefaultKubeAPIServerService(sniPhase component.Phase) component.DeployWaiter {
	return b.kubeAPIServiceService(sniPhase)
}

func (b *Botanist) kubeAPIServiceService(sniPhase component.Phase) component.DeployWaiter {
	return controlplane.NewKubeAPIService(
		&controlplane.KubeAPIServiceValues{
			Annotations:               b.Seed.LoadBalancerServiceAnnotations,
			KonnectivityTunnelEnabled: b.Shoot.KonnectivityTunnelEnabled,
			SNIPhase:                  sniPhase,
		},
		client.ObjectKey{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace},
		client.ObjectKey{Name: *b.Config.SNI.Ingress.ServiceName, Namespace: *b.Config.SNI.Ingress.Namespace},
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
		b.Logger,
		b.K8sSeedClient.DirectClient(),
		nil,
		b.setAPIServerServiceClusterIP,
		func(address string) { b.setAPIServerAddress(address, b.K8sSeedClient.DirectClient()) },
	)
}

// SNIPhase returns the current phase of the SNI enablement of kube-apiserver's service.
func (b *Botanist) SNIPhase(ctx context.Context) (component.Phase, error) {
	var (
		svc        = &corev1.Service{}
		sniEnabled = b.APIServerSNIEnabled()
	)

	if err := b.K8sSeedClient.APIReader().Get(
		ctx,
		client.ObjectKey{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace},
		svc,
	); client.IgnoreNotFound(err) != nil {
		return component.PhaseUnknown, err
	}

	switch {
	case svc.Spec.Type == corev1.ServiceTypeLoadBalancer && sniEnabled:
		return component.PhaseEnabling, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP && sniEnabled:
		return component.PhaseEnabled, nil
	case svc.Spec.Type == corev1.ServiceTypeClusterIP && !sniEnabled:
		return component.PhaseDisabling, nil
	default:
		if sniEnabled {
			// initial cluster creation with SNI enabled (enabling only relevant for migration).
			return component.PhaseEnabled, nil
		}
		// initial cluster creation with SNI disabled.
		return component.PhaseDisabled, nil
	}
}

// DeployKubeAPIService deploys for kube-apiserver service.
func (b *Botanist) DeployKubeAPIService(ctx context.Context, sniPhase component.Phase) error {
	return b.kubeAPIServiceService(sniPhase).Deploy(ctx)
}

// DeployKubeAPIServerSNI deploys the kube-apiserver-sni chart.
func (b *Botanist) DeployKubeAPIServerSNI(ctx context.Context) error {
	return b.Shoot.Components.ControlPlane.KubeAPIServerSNI.Deploy(ctx)
}

// DefaultKubeAPIServerSNI returns a deployer for kube-apiserver SNI.
func (b *Botanist) DefaultKubeAPIServerSNI() component.DeployWaiter {
	return component.OpDestroy(controlplane.NewKubeAPIServerSNI(
		&controlplane.KubeAPIServerSNIValues{
			Name: v1beta1constants.DeploymentNameKubeAPIServer,
			IstioIngressGateway: controlplane.IstioIngressGateway{
				Namespace: *b.Config.SNI.Ingress.Namespace,
				Labels:    b.Config.SNI.Ingress.Labels,
			},
		},
		b.Shoot.SeedNamespace,
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
	))
}

func (b *Botanist) setAPIServerServiceClusterIP(clusterIP string) {
	if b.Shoot.Components.ControlPlane.KubeAPIServerSNIPhase == component.PhaseDisabled {
		return
	}

	b.APIServerClusterIP = clusterIP

	b.Shoot.Components.ControlPlane.KubeAPIServerSNI = controlplane.NewKubeAPIServerSNI(
		&controlplane.KubeAPIServerSNIValues{
			ApiserverClusterIP: clusterIP,
			NamespaceUID:       b.SeedNamespaceObject.UID,
			Hosts: []string{
				gutil.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
				gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
			},
			Name: v1beta1constants.DeploymentNameKubeAPIServer,
			IstioIngressGateway: controlplane.IstioIngressGateway{
				Namespace: *b.Config.SNI.Ingress.Namespace,
				Labels:    b.Config.SNI.Ingress.Labels,
			},
		},
		b.Shoot.SeedNamespace,
		b.K8sSeedClient.ChartApplier(),
		b.ChartsRootPath,
	)
}

// setAPIServerAddress sets the IP address of the API server's LoadBalancer.
func (b *Botanist) setAPIServerAddress(address string, seedClient client.Client) {
	b.Operation.APIServerAddress = address

	if b.NeedsInternalDNS() {
		ownerID := *b.Shoot.Info.Status.ClusterIdentity + "-" + DNSInternalName
		b.Shoot.Components.Extensions.DNS.InternalOwner = dns.NewOwner(
			seedClient,
			b.Shoot.SeedNamespace,
			&dns.OwnerValues{
				Name:    DNSInternalName,
				Active:  pointer.BoolPtr(true),
				OwnerID: ownerID,
			},
		)
		b.Shoot.Components.Extensions.DNS.InternalEntry = dns.NewEntry(
			b.Logger,
			seedClient,
			b.Shoot.SeedNamespace,
			&dns.EntryValues{
				Name:    DNSInternalName,
				DNSName: gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
				Targets: []string{b.APIServerAddress},
				OwnerID: ownerID,
				TTL:     *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
			},
			nil,
		)
	}

	if b.NeedsExternalDNS() {
		ownerID := *b.Shoot.Info.Status.ClusterIdentity + "-" + DNSExternalName
		b.Shoot.Components.Extensions.DNS.ExternalOwner = dns.NewOwner(
			seedClient,
			b.Shoot.SeedNamespace,
			&dns.OwnerValues{
				Name:    DNSExternalName,
				Active:  pointer.BoolPtr(true),
				OwnerID: ownerID,
			},
		)
		b.Shoot.Components.Extensions.DNS.ExternalEntry = dns.NewEntry(
			b.Logger,
			seedClient,
			b.Shoot.SeedNamespace,
			&dns.EntryValues{
				Name:    DNSExternalName,
				DNSName: gutil.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain),
				Targets: []string{b.APIServerAddress},
				OwnerID: ownerID,
				TTL:     *b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
			},
			nil,
		)
	}
}

// CheckTunnelConnection checks if the tunnel connection between the control plane and the shoot networks
// is established.
func (b *Botanist) CheckTunnelConnection(ctx context.Context, logger *logrus.Entry, tunnelName string) (bool, error) {
	return health.CheckTunnelConnection(ctx, b.K8sShootClient, logger, tunnelName)
}

// RestartControlPlanePods restarts (deletes) pods of the shoot control plane.
func (b *Botanist) RestartControlPlanePods(ctx context.Context) error {
	return b.K8sSeedClient.Client().DeleteAllOf(
		ctx,
		&corev1.Pod{},
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels{v1beta1constants.LabelPodMaintenanceRestart: "true"},
	)
}
