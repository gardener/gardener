// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerscheduler

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/logger"
	schedulerv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	configMapSchedulerPrefix = "gardener-scheduler-config"
	dataConfigKey            = "schedulerconfiguration.yaml"
)

var schedulerCodec runtime.Codec

func init() {
	schedulerScheme := runtime.NewScheme()
	utilruntime.Must(schedulerv1alpha1.AddToScheme(schedulerScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, schedulerScheme, schedulerScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{
			schedulerv1alpha1.SchemeGroupVersion,
		})
	)

	schedulerCodec = serializer.NewCodecFactory(schedulerScheme).CodecForVersions(ser, ser, versions, versions)
}

func (g *gardenerScheduler) configMapSchedulerConfig() (*corev1.ConfigMap, error) {
	schedulerConfig := &schedulerv1alpha1.SchedulerConfiguration{
		ClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			QPS:        100,
			Burst:      130,
			Kubeconfig: gardenerutils.PathGenericKubeconfig,
		},
		LeaderElection: &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
			LeaderElect:       pointer.Bool(true),
			ResourceName:      schedulerv1alpha1.SchedulerDefaultLockObjectName,
			ResourceNamespace: metav1.NamespaceSystem,
		},
		LogLevel:  g.values.LogLevel,
		LogFormat: logger.FormatJSON,
		Server: schedulerv1alpha1.ServerConfiguration{
			HealthProbes: &schedulerv1alpha1.Server{Port: probePort},
			Metrics:      &schedulerv1alpha1.Server{Port: metricsPort},
		},
		Schedulers: schedulerv1alpha1.SchedulerControllerConfiguration{
			Shoot: &schedulerv1alpha1.ShootSchedulerConfiguration{
				Strategy: schedulerv1alpha1.MinimalDistance,
			},
		},
		FeatureGates: g.values.FeatureGates,
	}

	data, err := runtime.Encode(schedulerCodec, schedulerConfig)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapSchedulerPrefix,
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Data: map[string]string{
			dataConfigKey: string(data),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}
