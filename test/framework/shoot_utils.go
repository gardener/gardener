// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// ShootSeedNamespace gets the shoot namespace in the seed
func (f *ShootFramework) ShootSeedNamespace() string {
	return ComputeTechnicalID(f.Project.Name, f.Shoot)
}

// ShootKubeconfigSecretName gets the name of the secret with the kubeconfig of the shoot
func (f *ShootFramework) ShootKubeconfigSecretName() string {
	return fmt.Sprintf("%s.kubeconfig", f.Shoot.GetName())
}

// GetValiLogs gets logs from the last 1 hour for <key>, <value> from the vali instance in <valiNamespace>
func (f *ShootFramework) GetValiLogs(ctx context.Context, valiLabels map[string]string, valiNamespace, key, value string, client kubernetes.Interface) (*SearchResponse, error) {
	valiLabelsSelector := labels.SelectorFromSet(valiLabels)

	query := fmt.Sprintf("query=count_over_time({%s=~\"%s\"}[1h])", key, value)

	log := f.Logger.WithValues("namespace", valiNamespace, "labels", valiLabels, "q", query)

	var stdout io.Reader
	log.Info("Fetching logs")
	if err := retry.Until(ctx, defaultPollInterval, func(ctx context.Context) (bool, error) {
		var err error
		stdout, _, err = PodExecByLabel(ctx, client, valiNamespace, valiLabelsSelector, valiLogging,
			"wget", "http://localhost:"+strconv.Itoa(valiPort)+"/vali/api/v1/query", "-O-", "--post-data="+query,
		)
		if err != nil {
			log.Error(err, "Error fetching logs")
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return nil, err
	}

	search := &SearchResponse{}

	if err := json.NewDecoder(stdout).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}

// DumpState dumps the state of a shoot
// The state includes all k8s components running in the shoot itself as well as the controlplane
func (f *ShootFramework) DumpState(ctx context.Context) {
	if f.DisableStateDump {
		return
	}

	if f.Shoot != nil {
		log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(f.Shoot))
		if err := PrettyPrintObject(f.Shoot); err != nil {
			f.Logger.Error(err, "Cannot decode shoot")
		}

		isRunning, err := f.IsAPIServerRunning(ctx)
		if f.ShootClient != nil && isRunning && err == nil {
			if err := f.DumpDefaultResourcesInAllNamespaces(ctx, f.ShootClient); err != nil {
				f.Logger.Error(err, "Unable to dump resources from all namespaces in shoot")
			}
			if err := f.dumpNodes(ctx, log, f.ShootClient); err != nil {
				f.Logger.Error(err, "Unable to dump information of nodes from shoot")
			}
		} else {
			errMsg := ""
			if err != nil {
				errMsg = ": " + err.Error()
			}
			f.Logger.Error(err, "Unable to dump resources from shoot because API server is currently not running", "reason", errMsg)
		}
	}

	// dump controlplane in the shoot namespace
	if f.Seed != nil && f.SeedClient != nil {
		if err := f.dumpControlplaneInSeed(ctx, f.Seed, f.ShootSeedNamespace()); err != nil {
			f.Logger.Error(err, "Unable to dump controlplane in seed", "namespace", f.ShootSeedNamespace())
		}
	}

	if f.Shoot != nil {
		log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(f.Shoot))

		project, err := f.GetShootProject(ctx, f.Shoot.GetNamespace())
		if err != nil {
			log.Error(err, "Unable to get project namespace of shoot")
			return
		}

		// dump seed status if seed is available
		if f.Shoot.Spec.SeedName != nil {
			seed := &gardencorev1beta1.Seed{}
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: *f.Shoot.Spec.SeedName}, seed); err != nil {
				log.Error(err, "Unable to get seed", "seedName", *f.Shoot.Spec.SeedName)
				return
			}
			f.dumpSeed(seed)
		}

		err = f.dumpEventsInNamespace(ctx, log, f.GardenClient, *project.Spec.Namespace, func(event corev1.Event) bool {
			return event.InvolvedObject.Name == f.Shoot.Name
		})
		if err != nil {
			log.Error(err, "Unable to dump Events from project namespace in gardener", "namespace", *project.Spec.Namespace)
		}
	}
}

// CreateShootTestArtifacts creates a shoot object from the given path and sets common attributes (test-individual settings like workers have to be handled by each test).
func CreateShootTestArtifacts(cfg *ShootCreationConfig, projectNamespace string, clearDNS bool, clearExtensions bool) (string, *gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if cfg.shootYamlPath != "" {
		if err := ReadObject(cfg.shootYamlPath, shoot); err != nil {
			return "", nil, err
		}
	}

	if err := setShootMetadata(shoot, cfg, projectNamespace); err != nil {
		return "", nil, err
	}

	setShootGeneralSettings(shoot, cfg, clearExtensions)

	setShootNetworkingSettings(shoot, cfg, clearDNS)

	setShootTolerations(shoot)

	setShootControlPlaneHighAvailability(shoot, cfg)

	setShootControlPlaneAutoscaling(shoot, cfg)

	return shoot.Name, shoot, nil
}

func parseAnnotationCfg(cfg string) (map[string]string, error) {
	if !StringSet(cfg) {
		return nil, nil
	}
	result := make(map[string]string)
	annotations := strings.Split(cfg, ",")
	for _, annotation := range annotations {
		annotation = strings.TrimSpace(annotation)
		if !StringSet(annotation) {
			continue
		}
		keyValue := strings.Split(annotation, "=")
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("annotation %s could not be parsed into key and value", annotation)
		}
		result[keyValue[0]] = keyValue[1]
	}

	return result, nil
}

// setShootMetadata sets the Shoot's metadata from the given config and project namespace
func setShootMetadata(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig, projectNamespace string) error {
	if StringSet(cfg.testShootName) {
		shoot.Name = cfg.testShootName
	} else {
		integrationTestName, err := generateRandomShootName(cfg.testShootPrefix, 8)
		if err != nil {
			return err
		}
		shoot.Name = integrationTestName
	}

	if StringSet(projectNamespace) {
		shoot.Namespace = projectNamespace
	}

	if err := setConfiguredShootAnnotations(shoot, cfg); err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "true")

	return nil
}

// setConfiguredShootAnnotations sets annotations from the given config on the given shoot
func setConfiguredShootAnnotations(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig) error {
	annotations, err := parseAnnotationCfg(cfg.shootAnnotations)
	if err != nil {
		return err
	}
	for k, v := range annotations {
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, k, v)
	}
	return nil
}

// setShootGeneralSettings sets the Shoot's general settings from the given config
func setShootGeneralSettings(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig, clearExtensions bool) {
	if StringSet(cfg.shootRegion) {
		shoot.Spec.Region = cfg.shootRegion
	}

	if StringSet(cfg.cloudProfileName) {
		if shoot.Spec.CloudProfile == nil {
			shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{}
		}
		shoot.Spec.CloudProfile.Name = cfg.cloudProfileName
	}

	if StringSet(cfg.cloudProfileKind) {
		if shoot.Spec.CloudProfile == nil {
			shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{}
		}
		shoot.Spec.CloudProfile.Kind = cfg.cloudProfileKind
	}

	if StringSet(cfg.secretBinding) {
		shoot.Spec.SecretBindingName = ptr.To(cfg.secretBinding)
	}

	if StringSet(cfg.shootProviderType) {
		shoot.Spec.Provider.Type = cfg.shootProviderType
	}

	if StringSet(cfg.shootK8sVersion) {
		shoot.Spec.Kubernetes.Version = cfg.shootK8sVersion
	}

	if StringSet(cfg.seedName) {
		shoot.Spec.SeedName = &cfg.seedName
	}

	if cfg.startHibernated {
		if shoot.Spec.Hibernation == nil {
			shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{}
		}
		shoot.Spec.Hibernation.Enabled = &cfg.startHibernated
	}

	if clearExtensions {
		shoot.Spec.Extensions = nil
	}
}

// setShootNetworkingSettings sets the Shoot's networking settings from the given config
func setShootNetworkingSettings(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig, clearDNS bool) {
	if StringSet(cfg.externalDomain) {
		shoot.Spec.DNS = &gardencorev1beta1.DNS{Domain: &cfg.externalDomain}
		clearDNS = false
	}

	if strings.Contains(cfg.ipFamilies, ",") {
		shoot.Spec.Networking.IPFamilies = nil
		for _, part := range strings.Split(cfg.ipFamilies, ",") {
			shoot.Spec.Networking.IPFamilies = append(shoot.Spec.Networking.IPFamilies, gardencorev1beta1.IPFamily(part))
		}
	} else if StringSet(cfg.ipFamilies) {
		shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamily(cfg.ipFamilies)}
	}

	if gardencorev1beta1.IsIPv6SingleStack(shoot.Spec.Networking.IPFamilies) {
		// Node IP range will be set by the infrastructure and should not be provided
		shoot.Spec.Networking.Nodes = nil
	} else if StringSet(cfg.networkingNodes) {
		shoot.Spec.Networking.Nodes = &cfg.networkingNodes
	}

	if StringSet(cfg.networkingType) {
		shoot.Spec.Networking.Type = ptr.To(cfg.networkingType)
	}

	if StringSet(cfg.networkingPods) {
		shoot.Spec.Networking.Pods = &cfg.networkingPods
	}

	if StringSet(cfg.networkingServices) {
		shoot.Spec.Networking.Services = &cfg.networkingServices
	}

	if clearDNS {
		shoot.Spec.DNS = &gardencorev1beta1.DNS{}
	}
}

// setShootTolerations sets the Shoot's tolerations
func setShootTolerations(shoot *gardencorev1beta1.Shoot) {
	shoot.Spec.Tolerations = []gardencorev1beta1.Toleration{
		{
			Key:   SeedTaintTestRun,
			Value: ptr.To(GetTestRunID()),
		},
	}
}

// SetProviderConfigsFromFilepath parses the infrastructure, controlPlane and networking provider-configs and sets them on the shoot
func SetProviderConfigsFromFilepath(shoot *gardencorev1beta1.Shoot, infrastructureConfigPath, controlPlaneConfigPath, networkingConfigPath string) error {
	// clear provider configs first
	shoot.Spec.Provider.InfrastructureConfig = nil
	shoot.Spec.Provider.ControlPlaneConfig = nil
	shoot.Spec.Networking.ProviderConfig = nil

	if StringSet(infrastructureConfigPath) {
		infrastructureProviderConfig, err := ParseFileAsProviderConfig(infrastructureConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Provider.InfrastructureConfig = infrastructureProviderConfig
	}

	if StringSet(controlPlaneConfigPath) {
		controlPlaneProviderConfig, err := ParseFileAsProviderConfig(controlPlaneConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Provider.ControlPlaneConfig = controlPlaneProviderConfig
	}

	if StringSet(networkingConfigPath) {
		networkingProviderConfig, err := ParseFileAsProviderConfig(networkingConfigPath)
		if err != nil {
			return err
		}
		shoot.Spec.Networking.ProviderConfig = networkingProviderConfig
	}

	return nil
}

func generateRandomShootName(prefix string, length int) (string, error) {
	randomString, err := utils.GenerateRandomString(length)
	if err != nil {
		return "", err
	}

	if len(prefix) > 0 {
		return prefix + strings.ToLower(randomString), nil
	}

	return IntegrationTestPrefix + strings.ToLower(randomString), nil
}

// PrettyPrintObject prints a object as pretty printed yaml to stdout
func PrettyPrintObject(obj runtime.Object) error {
	d, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	fmt.Print(string(d))
	return nil
}

func setShootControlPlaneHighAvailability(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig) {
	if StringSet(cfg.controlPlaneFailureTolerance) {
		if shoot.Spec.ControlPlane == nil {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{
					FailureTolerance: gardencorev1beta1.FailureTolerance{},
				},
			}
		}

		if shoot.Spec.ControlPlane.HighAvailability == nil {
			shoot.Spec.ControlPlane.HighAvailability = &gardencorev1beta1.HighAvailability{
				FailureTolerance: gardencorev1beta1.FailureTolerance{},
			}
		}
		shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type = gardencorev1beta1.FailureToleranceType(cfg.controlPlaneFailureTolerance)
	}
}

// setShootControlPlaneAutoscaling sets autoscaling settings for kube API server and etcd
func setShootControlPlaneAutoscaling(shoot *gardencorev1beta1.Shoot, cfg *ShootCreationConfig) {
	if StringSet(cfg.kubeApiserverMinAllowedCPU) || StringSet(cfg.kubeApiserverMinAllowedMemory) {
		minAllowed := make(map[corev1.ResourceName]resource.Quantity)
		if StringSet(cfg.kubeApiserverMinAllowedCPU) {
			minAllowed[corev1.ResourceCPU] = resource.MustParse(cfg.kubeApiserverMinAllowedCPU)
		}
		if StringSet(cfg.kubeApiserverMinAllowedMemory) {
			minAllowed[corev1.ResourceMemory] = resource.MustParse(cfg.kubeApiserverMinAllowedMemory)
		}

		if shoot.Spec.Kubernetes.KubeAPIServer == nil {
			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{}
		}
		shoot.Spec.Kubernetes.KubeAPIServer.Autoscaling = &gardencorev1beta1.ControlPlaneAutoscaling{MinAllowed: minAllowed}
	}
	if StringSet(cfg.etcdMinAllowedCPU) || StringSet(cfg.etcdMinAllowedMemory) {
		minAllowed := make(map[corev1.ResourceName]resource.Quantity)
		if StringSet(cfg.etcdMinAllowedCPU) {
			minAllowed[corev1.ResourceCPU] = resource.MustParse(cfg.etcdMinAllowedCPU)
		}
		if StringSet(cfg.etcdMinAllowedMemory) {
			minAllowed[corev1.ResourceMemory] = resource.MustParse(cfg.etcdMinAllowedMemory)
		}

		if shoot.Spec.Kubernetes.ETCD == nil {
			shoot.Spec.Kubernetes.ETCD = &gardencorev1beta1.ETCD{}
		}
		if shoot.Spec.Kubernetes.ETCD.Events == nil {
			shoot.Spec.Kubernetes.ETCD.Events = &gardencorev1beta1.ETCDConfig{}
		}
		if shoot.Spec.Kubernetes.ETCD.Main == nil {
			shoot.Spec.Kubernetes.ETCD.Main = &gardencorev1beta1.ETCDConfig{}
		}
		shoot.Spec.Kubernetes.ETCD.Events.Autoscaling = &gardencorev1beta1.ControlPlaneAutoscaling{MinAllowed: minAllowed}
		shoot.Spec.Kubernetes.ETCD.Main.Autoscaling = &gardencorev1beta1.ControlPlaneAutoscaling{MinAllowed: minAllowed}
	}
}
