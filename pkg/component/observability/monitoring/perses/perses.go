// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persesconfig "github.com/perses/perses/pkg/model/api/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	victorialogsconstants "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	labelApp = "perses"
)

func (p *perses) persesName() string {
	if p.values.IsGardenCluster {
		return "perses-garden"
	}
	return "perses-seed"
}

func (p *perses) perses() *persesv1alpha2.Perses {
	labels := p.getLabels()
	labels["app.kubernetes.io/instance"] = p.persesName()

	scrapeTargetAnnotationKey := resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix +
		v1beta1constants.LabelNetworkPolicySeedScrapeTargets +
		resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix
	serviceAnnotations := map[string]string{
		scrapeTargetAnnotationKey: `[{"protocol":"TCP","port":8080}]`,
	}
	if p.values.IsGardenCluster {
		gardenScrapeKey := resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix +
			v1beta1constants.LabelNetworkPolicyGardenScrapeTargets +
			resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix
		serviceAnnotations[gardenScrapeKey] = `[{"protocol":"TCP","port":8080}]`
	}

	obj := &persesv1alpha2.Perses{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.persesName(),
			Namespace: p.namespace,
			Labels:    labels,
		},
		Spec: persesv1alpha2.PersesSpec{
			Metadata: &persesv1alpha2.Metadata{
				Labels: p.getPodLabels(),
			},
			Service: &persesv1alpha2.PersesService{
				Annotations: serviceAnnotations,
			},
			Config: persesv1alpha2.PersesConfig{
				Config: persesconfig.Config{
					Security: persesconfig.Security{
						Readonly:   false,
						EnableAuth: false,
					},
					Database: persesconfig.Database{
						File: &persesconfig.File{
							Folder: "/perses",
						},
					},
					Frontend: persesconfig.Frontend{
						Explorer: persesconfig.Explorer{
							Enable: true,
						},
					},
				},
			},
			Replicas:      &p.values.Replicas,
			Image:         &p.values.Image,
			ContainerPort: new(int32(8080)),
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
			Storage: &persesv1alpha2.StorageConfiguration{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	return obj
}

func (p *perses) getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  labelApp,
		v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring,
	}
}

func (p *perses) getPodLabels() map[string]string {
	labels := map[string]string{
		v1beta1constants.LabelObservabilityApplication:        labelApp,
		v1beta1constants.LabelRole:                            v1beta1constants.LabelMonitoring,
		v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
	}

	if p.values.VictoriaLogsEnabled {
		labels[gardenerutils.NetworkPolicyLabel(victorialogsconstants.ServiceName, victorialogsconstants.VictoriaLogsPort)] = v1beta1constants.LabelNetworkPolicyAllowed
	}

	seedSpecificLabels := map[string]string{
		gardenerutils.NetworkPolicyLabel("prometheus-aggregate", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel("prometheus-seed", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
	}

	if p.values.IsGardenCluster {
		labels[gardenerutils.NetworkPolicyLabel("prometheus-garden", 9090)] = v1beta1constants.LabelNetworkPolicyAllowed
		labels[gardenerutils.NetworkPolicyLabel("prometheus-longterm", 9091)] = v1beta1constants.LabelNetworkPolicyAllowed
		// If the garden is also a seed, allow access to seed-specific Prometheus instances.
		for k, v := range seedSpecificLabels {
			labels[k] = v
		}
		return labels
	}

	switch p.values.ClusterType {
	case component.ClusterTypeSeed:
		for k, v := range seedSpecificLabels {
			labels[k] = v
		}
	}

	return labels
}
