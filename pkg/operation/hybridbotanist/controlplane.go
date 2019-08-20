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

package hybridbotanist

import (
	"context"
	"fmt"
	"path/filepath"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	audit_internal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
)

const (
	auditPolicyConfigMapDataKey = "policy"
)

var (
	chartPathControlPlane = filepath.Join(common.ChartPath, "seed-controlplane", "charts")
	runtimeScheme         = runtime.NewScheme()
	codecs                = serializer.NewCodecFactory(runtimeScheme)
	decoder               = codecs.UniversalDecoder()
)

func init() {
	_ = auditv1alpha1.AddToScheme(runtimeScheme)
	_ = auditv1beta1.AddToScheme(runtimeScheme)
	_ = auditv1.AddToScheme(runtimeScheme)
	_ = audit_internal.AddToScheme(runtimeScheme)
}

// getResourcesForAPIServer returns the cpu and memory requirements for API server based on nodeCount
func getResourcesForAPIServer(nodeCount int) (string, string, string, string) {
	var (
		cpuRequest    string
		memoryRequest string
		cpuLimit      string
		memoryLimit   string
	)

	switch {
	case nodeCount <= 2:
		cpuRequest = "800m"
		memoryRequest = "800Mi"

		cpuLimit = "1000m"
		memoryLimit = "1200Mi"
	case nodeCount <= 10:
		cpuRequest = "1000m"
		memoryRequest = "1100Mi"

		cpuLimit = "1200m"
		memoryLimit = "1900Mi"
	case nodeCount <= 50:
		cpuRequest = "1200m"
		memoryRequest = "1600Mi"

		cpuLimit = "1500m"
		memoryLimit = "3900Mi"
	case nodeCount <= 100:
		cpuRequest = "2500m"
		memoryRequest = "5200Mi"

		cpuLimit = "3000m"
		memoryLimit = "5900Mi"
	default:
		cpuRequest = "3000m"
		memoryRequest = "5200Mi"

		cpuLimit = "4000m"
		memoryLimit = "7800Mi"
	}

	return cpuRequest, memoryRequest, cpuLimit, memoryLimit
}

// DeployETCD deploys two etcd clusters via StatefulSets. The first etcd cluster (called 'main') is used for all the
// data the Shoot Kubernetes cluster needs to store, whereas the second etcd luster (called 'events') is only used to
// store the events data. The objectstore is also set up to store the backups.
func (b *HybridBotanist) DeployETCD() error {
	secretData, err := b.SeedCloudBotanist.GenerateEtcdBackupConfig()
	if err != nil {
		return err
	}

	// Some cloud botanists do not yet support backup and won't return secret data.
	if secretData != nil {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.BackupSecretName,
				Namespace: b.Shoot.SeedNamespace,
			},
		}

		if err := kutil.CreateOrUpdate(context.TODO(), b.K8sSeedClient.Client(), secret, func() error {
			secret.Type = corev1.SecretTypeOpaque
			secret.Data = secretData
			return nil
		}); err != nil {
			return err
		}
	}

	etcdConfig := map[string]interface{}{
		"podAnnotations": map[string]interface{}{
			"checksum/secret-etcd-ca":         b.CheckSums[gardencorev1alpha1.SecretNameCAETCD],
			"checksum/secret-etcd-server-tls": b.CheckSums["etcd-server-tls"],
			"checksum/secret-etcd-client-tls": b.CheckSums["etcd-client-tls"],
		},
		"storageCapacity": b.Seed.GetValidVolumeSize("10Gi"),
	}

	etcd, err := b.InjectSeedShootImages(etcdConfig, common.ETCDImageName)
	if err != nil {
		return err
	}

	for _, role := range []string{common.EtcdRoleMain, common.EtcdRoleEvents} {
		etcd["role"] = role
		if role == common.EtcdRoleMain {
			// etcd-main emits extensive (histogram) metrics
			etcd["metrics"] = "extensive"
		}

		if b.Shoot.HibernationEnabled {
			statefulset := &appsv1.StatefulSet{}
			if err := b.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(b.Shoot.SeedNamespace, fmt.Sprintf("etcd-%s", role)), statefulset); err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			if statefulset.Spec.Replicas == nil {
				etcd["replicas"] = 0
			} else {
				etcd["replicas"] = *statefulset.Spec.Replicas
			}
		}

		if err := b.ApplyChartSeed(filepath.Join(chartPathControlPlane, "etcd"), b.Shoot.SeedNamespace, fmt.Sprintf("etcd-%s", role), nil, etcd); err != nil {
			return err
		}
	}

	return nil
}

func (b *HybridBotanist) deployNetworkPolicies(ctx context.Context, denyAll bool) error {
	var (
		globalNetworkPoliciesValues = map[string]interface{}{
			"blockedAddresses": b.Seed.Info.Spec.BlockCIDRs,
			"denyAll":          denyAll,
		}
		excludeNets = []gardencorev1alpha1.CIDR{}

		values            = map[string]interface{}{}
		shootCIDRNetworks = []gardencorev1alpha1.CIDR{}
	)

	for _, addr := range b.Seed.Info.Spec.BlockCIDRs {
		excludeNets = append(excludeNets, addr)
	}

	networks, err := b.Shoot.GetK8SNetworks()
	if err != nil {
		return err
	}
	if networks != nil {
		if networks.Nodes != nil {
			shootCIDRNetworks = append(shootCIDRNetworks, *networks.Nodes)
		}
		if networks.Pods != nil {
			shootCIDRNetworks = append(shootCIDRNetworks, *networks.Pods)
		}
		if networks.Services != nil {
			shootCIDRNetworks = append(shootCIDRNetworks, *networks.Services)
		}
		shootNetworkValues, err := common.ExceptNetworks(shootCIDRNetworks, excludeNets...)
		if err != nil {
			return err
		}
		values["clusterNetworks"] = shootNetworkValues
	}

	seedNetworks := b.Seed.Info.Spec.Networks
	allCIDRNetworks := append([]gardencorev1alpha1.CIDR{seedNetworks.Nodes, seedNetworks.Pods, seedNetworks.Services}, shootCIDRNetworks...)
	allCIDRNetworks = append(allCIDRNetworks, excludeNets...)

	privateNetworks, err := common.ToExceptNetworks(common.AllPrivateNetworkBlocks(), allCIDRNetworks...)
	if err != nil {
		return err
	}
	globalNetworkPoliciesValues["privateNetworks"] = privateNetworks
	values["global-network-policies"] = globalNetworkPoliciesValues

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, "network-policies"), b.Shoot.SeedNamespace, "network-policies", values, nil)
}

// DeployNetworkPolicies creates a network policies in a Shoot cluster's namespace that
// deny all traffic and allow certain components to use annotations to declare their desire
// to transmit/receive traffic to/from other Pods/IP addresses.
func (b *HybridBotanist) DeployNetworkPolicies(ctx context.Context) error {
	return b.deployNetworkPolicies(ctx, true)
}

// DeployKubeAPIServerService deploys kube-apiserver service.
func (b *HybridBotanist) DeployKubeAPIServerService() error {
	var (
		name          = "kube-apiserver-service"
		defaultValues = map[string]interface{}{}
	)

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), b.Shoot.SeedNamespace, name, defaultValues, nil)
}

// DeployKubeAPIServer deploys kube-apiserver deployment.
func (b *HybridBotanist) DeployKubeAPIServer() error {
	defaultValues := map[string]interface{}{
		"etcdServicePort":   2379,
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"shootNetworks": map[string]interface{}{
			"service": b.Shoot.GetServiceNetwork(),
			"pod":     b.Shoot.GetPodNetwork(),
			"node":    b.Shoot.GetNodeNetwork(),
		},
		"seedNetworks": map[string]interface{}{
			"service": b.Seed.Info.Spec.Networks.Services,
			"pod":     b.Seed.Info.Spec.Networks.Pods,
			"node":    b.Seed.Info.Spec.Networks.Nodes,
		},
		"maxReplicas":      3,
		"securePort":       443,
		"probeCredentials": utils.EncodeBase64([]byte(fmt.Sprintf("%s:%s", b.Secrets[common.KubecfgSecretName].Data[secrets.DataKeyUserName], b.Secrets[common.KubecfgSecretName].Data[secrets.DataKeyPassword]))),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                        b.CheckSums[gardencorev1alpha1.SecretNameCACluster],
			"checksum/secret-ca-front-proxy":            b.CheckSums[gardencorev1alpha1.SecretNameCAFrontProxy],
			"checksum/secret-kube-apiserver":            b.CheckSums[gardencorev1alpha1.DeploymentNameKubeAPIServer],
			"checksum/secret-kube-aggregator":           b.CheckSums["kube-aggregator"],
			"checksum/secret-kube-apiserver-kubelet":    b.CheckSums["kube-apiserver-kubelet"],
			"checksum/secret-kube-apiserver-basic-auth": b.CheckSums[common.BasicAuthSecretName],
			"checksum/secret-static-token":              b.CheckSums[common.StaticTokenSecretName],
			"checksum/secret-vpn-seed":                  b.CheckSums["vpn-seed"],
			"checksum/secret-vpn-seed-tlsauth":          b.CheckSums["vpn-seed-tlsauth"],
			"checksum/secret-service-account-key":       b.CheckSums["service-account-key"],
			"checksum/secret-etcd-ca":                   b.CheckSums[gardencorev1alpha1.SecretNameCAETCD],
			"checksum/secret-etcd-client-tls":           b.CheckSums["etcd-client-tls"],
		},
	}
	enableEtcdEncryption, err := utils.CheckVersionMeetsConstraint(b.Shoot.Info.Spec.Kubernetes.Version, ">= 1.13")
	if err != nil {
		return err
	}
	if enableEtcdEncryption {
		defaultValues["enableEtcdEncryption"] = true
		podAnotationMap := defaultValues["podAnnotations"].(map[string]interface{})
		podAnotationMap["checksum/secret-etcd-encryption"] = b.CheckSums[common.EtcdEncryptionSecretName]
	}

	if b.ShootedSeed != nil {
		var (
			apiServer  = b.ShootedSeed.APIServer
			autoscaler = apiServer.Autoscaler
		)
		defaultValues["replicas"] = *apiServer.Replicas
		defaultValues["minReplicas"] = *autoscaler.MinReplicas
		defaultValues["maxReplicas"] = autoscaler.MaxReplicas
		defaultValues["apiServerResources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "2000m",
				"memory": "7000Mi",
			},
		}
	} else {
		deployment := &appsv1.Deployment{}
		if err := b.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeAPIServer), deployment); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
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

		cpuRequest, memoryRequest, cpuLimit, memoryLimit := getResourcesForAPIServer(b.Shoot.GetNodeCount())
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

	var (
		apiServerConfig  = b.Shoot.Info.Spec.Kubernetes.KubeAPIServer
		admissionPlugins = kubernetes.GetAdmissionPluginsForVersion(b.Shoot.Info.Spec.Kubernetes.Version)
	)

	if apiServerConfig != nil {
		defaultValues["featureGates"] = apiServerConfig.FeatureGates
		defaultValues["runtimeConfig"] = apiServerConfig.RuntimeConfig

		if apiServerConfig.OIDCConfig != nil {
			defaultValues["oidcConfig"] = apiServerConfig.OIDCConfig
		}

		if serviceAccountConfig := apiServerConfig.ServiceAccountConfig; serviceAccountConfig != nil {
			config := make(map[string]interface{})

			if issuer := serviceAccountConfig.Issuer; issuer != nil {
				config["issuer"] = *issuer
			}

			if signingKeySecret := serviceAccountConfig.SigningKeySecret; signingKeySecret != nil {
				signingKey, err := common.GetServiceAccountSigningKeySecret(context.TODO(), b.K8sGardenClient.Client(), b.Shoot.Info.Namespace, signingKeySecret.Name)
				if err != nil {
					return err
				}

				config["signingKey"] = signingKey
			}

			defaultValues["serviceAccountConfig"] = config
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
			auditPolicy, err := b.getAuditPolicy(apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name, b.Shoot.Info.Namespace)
			if err != nil {
				return fmt.Errorf("Retrieving audit policy from the ConfigMap '%v' failed with reason '%v'", apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name, err)
			}
			defaultValues["auditConfig"] = map[string]interface{}{
				"auditPolicy": auditPolicy,
			}
		}
	}
	defaultValues["admissionPlugins"] = admissionPlugins

	values, err := b.InjectSeedShootImages(defaultValues,
		common.HyperkubeImageName,
		common.VPNSeedImageName,
		common.BlackboxExporterImageName,
		common.AlpineIptablesImageName,
	)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, gardencorev1alpha1.DeploymentNameKubeAPIServer), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeAPIServer, values, nil)
}

func (b *HybridBotanist) getAuditPolicy(name, namespace string) (string, error) {
	auditPolicyCm := &corev1.ConfigMap{}
	if err := b.K8sGardenClient.Client().Get(context.TODO(), kutil.Key(namespace, name), auditPolicyCm); err != nil {
		return "", err
	}
	auditPolicy, ok := auditPolicyCm.Data[auditPolicyConfigMapDataKey]
	if !ok {
		return "", fmt.Errorf("Missing '.data.policy' in audit policy configmap %v/%v", namespace, name)
	}
	if len(auditPolicy) == 0 {
		return "", fmt.Errorf("Empty audit policy. Provide non-empty audit policy")
	}
	auditPolicyObj, schemaVersion, err := decoder.Decode([]byte(auditPolicy), nil, nil)
	if err != nil {
		return "", fmt.Errorf("Failed to decode the provided audit policy err=%v", err)
	}

	if isValidVersion, err := IsValidAuditPolicyVersion(b.ShootVersion(), schemaVersion); err != nil {
		return "", err
	} else if !isValidVersion {
		return "", fmt.Errorf("Your shoot cluster version %q is not compatible with audit policy version %q", b.ShootVersion(), schemaVersion.GroupVersion().String())
	}

	auditPolicyInternal, ok := auditPolicyObj.(*audit_internal.Policy)
	if !ok {
		return "", fmt.Errorf("Failure to cast to audit Policy type: %v", schemaVersion)
	}
	errList := auditvalidation.ValidatePolicy(auditPolicyInternal)
	if len(errList) != 0 {
		return "", fmt.Errorf("Provided invalid audit policy err=%v", errList)
	}
	return auditPolicy, nil
}

// IsValidAuditPolicyVersion checks whether the api server support the provided audit policy apiVersion
func IsValidAuditPolicyVersion(shootVersion string, schemaVersion *schema.GroupVersionKind) (bool, error) {
	auditGroupVersion := schemaVersion.GroupVersion().String()

	if auditGroupVersion == "audit.k8s.io/v1" {
		return utils.CheckVersionMeetsConstraint(shootVersion, ">= v1.12")
	}
	return true, nil
}

// DeployKubeControllerManager deploys kube-controller-manager deployment.
func (b *HybridBotanist) DeployKubeControllerManager() error {
	defaultValues := map[string]interface{}{
		"clusterName":       b.Shoot.SeedNamespace,
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":        b.Shoot.GetPodNetwork(),
		"serviceNetwork":    b.Shoot.GetServiceNetwork(),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                             b.CheckSums[gardencorev1alpha1.SecretNameCACluster],
			"checksum/secret-kube-controller-manager":        b.CheckSums[gardencorev1alpha1.DeploymentNameKubeControllerManager],
			"checksum/secret-kube-controller-manager-server": b.CheckSums[common.KubeControllerManagerServerName],
			"checksum/secret-service-account-key":            b.CheckSums["service-account-key"],
		},
		"objectCount": b.Shoot.GetNodeCount(),
	}

	if b.Shoot.HibernationEnabled {
		replicaCount, err := common.CurrentReplicaCount(b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeControllerManager)
		if err != nil {
			return err
		}
		defaultValues["replicas"] = replicaCount
	}

	controllerManagerConfig := b.Shoot.Info.Spec.Kubernetes.KubeControllerManager
	if controllerManagerConfig != nil {
		defaultValues["featureGates"] = controllerManagerConfig.FeatureGates

		if controllerManagerConfig.HorizontalPodAutoscalerConfig != nil {
			defaultValues["horizontalPodAutoscaler"] = controllerManagerConfig.HorizontalPodAutoscalerConfig
		}

		if controllerManagerConfig.NodeCIDRMaskSize != nil {
			defaultValues["nodeCIDRMaskSize"] = *controllerManagerConfig.NodeCIDRMaskSize
		}
	}

	values, err := b.InjectSeedShootImages(defaultValues, common.HyperkubeImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, gardencorev1alpha1.DeploymentNameKubeControllerManager), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeControllerManager, values, nil)
}

// DeployKubeScheduler deploys kube-scheduler deployment.
func (b *HybridBotanist) DeployKubeScheduler() error {
	defaultValues := map[string]interface{}{
		"replicas":          b.Shoot.GetReplicas(1),
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-kube-scheduler":        b.CheckSums[gardencorev1alpha1.DeploymentNameKubeScheduler],
			"checksum/secret-kube-scheduler-server": b.CheckSums[common.KubeSchedulerServerName],
		},
	}

	if b.ShootedSeed != nil {
		defaultValues["resources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "300m",
				"memory": "512Mi",
			},
		}
	}

	schedulerConfig := b.Shoot.Info.Spec.Kubernetes.KubeScheduler
	if schedulerConfig != nil {
		defaultValues["featureGates"] = schedulerConfig.FeatureGates
	}

	values, err := b.InjectSeedShootImages(defaultValues, common.HyperkubeImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, gardencorev1alpha1.DeploymentNameKubeScheduler), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeScheduler, values, nil)
}
