// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplane

import (
	"path/filepath"

	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/helm"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// LabelScheduler is a constant for the value of a label with key 'role' whose value is 'scheduler'.
	LabelScheduler = "scheduler"
	// KubeSchedulerDataKeyComponentConfig is a constant for the key of the data map in a ConfigMap whose value is the
	// component configuration of the kube-scheduler.
	KubeSchedulerDataKeyComponentConfig = "config.yaml"
	// KubeSchedulerPortNameMetrics is a constant for the name of the metrics port of the kube-scheduler.
	KubeSchedulerPortNameMetrics = "metrics"

	kubeSchedulerVolumeMountPathKubeconfig = "/var/lib/kube-scheduler"
	kubeSchedulerVolumeMountPathServer     = "/var/lib/kube-scheduler-server"
	kubeSchedulerVolumeMountPathConfig     = "/var/lib/kube-scheduler-config"
)

// KubeScheduler contains function for a kube-scheduler deployer.
type KubeScheduler interface {
	component.DeployWaiter
	KubeSchedulerValues
}

// KubeSchedulerValues contains functions to manipulate the kube-scheduler's values.
type KubeSchedulerValues interface {
	// SetSecrets sets the secrets for the kube-scheduler.
	SetSecrets(KubeSchedulerSecrets)
}

// NewKubeScheduler creates a new instance of DeployWaiter for the kube-scheduler.
func NewKubeScheduler(
	chartApplier kubernetes.ChartApplier,
	controlPlaneChartPath string,
	namespace string,
	version string,
	images map[string]*imagevector.Image,
	replicas int32,
	config *gardencorev1beta1.KubeSchedulerConfig,
) (KubeScheduler, error) {
	values, err := defaultValues(version, images, replicas, config)
	if err != nil {
		return nil, err
	}

	return &kubeScheduler{
		DeployWaiter: helm.NewChartComponent(
			chartApplier,
			filepath.Join(controlPlaneChartPath, v1beta1constants.DeploymentNameKubeScheduler),
			namespace,
			v1beta1constants.DeploymentNameKubeScheduler,
			values,
		),
		KubeSchedulerValues: values,
	}, nil
}

type kubeScheduler struct {
	component.DeployWaiter
	KubeSchedulerValues
}

type kubeSchedulerValues struct {
	KubernetesVersion string                       `json:"kubernetesVersion,omitempty"`
	Replicas          int32                        `json:"replicas,omitempty"`
	Labels            map[string]string            `json:"labels,omitempty"`
	PodLabels         map[string]string            `json:"podLabels,omitempty"`
	PodAnnotations    map[string]string            `json:"podAnnotations,omitempty"`
	FeatureGates      map[string]bool              `json:"featureGates,omitempty"`
	Images            map[string]string            `json:"images,omitempty"`
	Resources         *corev1.ResourceRequirements `json:"resources,omitempty"`
	KubeMaxPDVols     *string                      `json:"kubeMaxPDVols,omitempty"`
	Ports             Ports                        `json:"ports,omitempty"`
	Volumes           Volumes                      `json:"volumes,omitempty"`
}

type Ports map[string]Port

type Port struct {
	Name          string          `json:"name,omitempty"`
	Protocol      corev1.Protocol `json:"protocol,omitempty"`
	ContainerPort int32           `json:"containerPort,omitempty"`
	HostPort      int32           `json:"hostPort,omitempty"`
	ServicePort   int32           `json:"servicePort,omitempty"`
	NodePort      int32           `json:"nodePort,omitempty"`
}

type Volumes map[string]Volume

type Volume struct {
	Name          string  `json:"name,omitempty"`
	MountPath     string  `json:"mountPath,omitempty"`
	SecretName    *string `json:"secretName,omitempty"`
	ConfigMapName *string `json:"configMapName,omitempty"`
	DataKey       *string `json:"dataKey,omitempty"`
}

func defaultValues(
	version string,
	images map[string]*imagevector.Image,
	replicas int32,
	config *gardencorev1beta1.KubeSchedulerConfig,
) (KubeSchedulerValues, error) {
	return &kubeSchedulerValues{
		KubernetesVersion: version,
		Replicas:          replicas,
		Labels: map[string]string{
			v1beta1constants.LabelApp:             v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole:            LabelScheduler,
			v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
		},
		PodLabels: map[string]string{
			v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyFromPrometheus:   v1beta1constants.LabelNetworkPolicyAllowed,
		},
		PodAnnotations: nil,
		FeatureGates: func() map[string]bool {
			if config == nil {
				return nil
			}
			return config.FeatureGates
		}(),
		Images: chart.ImageMapToValues(images),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("23m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("400m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		KubeMaxPDVols: func() *string {
			if config == nil {
				return nil
			}
			return config.KubeMaxPDVols
		}(),
		Ports: Ports{
			"metrics": Port{
				Name:     KubeSchedulerPortNameMetrics,
				Protocol: corev1.ProtocolTCP,
			},
		},
		Volumes: Volumes{
			"kubeconfig": Volume{
				Name:       "kubeconfig-secret",
				SecretName: pointer.StringPtr("kube-scheduler"),
				MountPath:  kubeSchedulerVolumeMountPathKubeconfig,
			},
			"server": Volume{
				Name:       "server-secret",
				SecretName: pointer.StringPtr("kube-scheduler-server"),
				MountPath:  kubeSchedulerVolumeMountPathServer,
			},
			"config": Volume{
				Name:          "kube-scheduler-config",
				ConfigMapName: pointer.StringPtr("kube-scheduler-config"),
				MountPath:     kubeSchedulerVolumeMountPathConfig,
				DataKey:       pointer.StringPtr(KubeSchedulerDataKeyComponentConfig),
			},
		},
	}, nil
}

func (v *kubeSchedulerValues) SetSecrets(secrets KubeSchedulerSecrets) {
	v.PodAnnotations = utils.MergeStringMaps(v.PodAnnotations, map[string]string{
		"checksum/secret-" + secrets.Kubeconfig.Name: secrets.Kubeconfig.Checksum,
		"checksum/secret-" + secrets.Server.Name:     secrets.Server.Checksum,
	})
}

// KubeSchedulerSecrets is collection of secrets for the kube-scheduler.
type KubeSchedulerSecrets struct {
	// Kubeconfig is a secret which can be used by the kube-scheduler to communicate to the kube-apiserver.
	Kubeconfig Secret
	// Server is a secret for the HTTPS server inside the kube-scheduler (which is used for metrics and health checks).
	Server Secret
}

// Secret is a structure that contains information about a Kubernetes secret which is managed externally.
type Secret struct {
	// Name is the name of the Kubernetes secret object.
	Name string
	// Checksum is the checksum of the secret's data.
	Checksum string
}
