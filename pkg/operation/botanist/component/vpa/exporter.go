// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpa

import (
	"context"

	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	exporter                  = "vpa-exporter"
	exporterPortMetrics int32 = 9570
)

// ValuesExporter is a set of configuration values for the vpa-exporter.
type ValuesExporter struct {
	// Image is the container image.
	Image string
}

func (v *vpa) deployExporterResources(ctx context.Context) error {
	service := v.emptyService(exporter)

	if v.values.ClusterType == ClusterTypeSeed {
		v.reconcileExporterService(service)

		return v.registry.Add(
			service,
		)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, service, func() error {
		v.reconcileExporterService(service)
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (v *vpa) destroyExporterResources(ctx context.Context) error {
	if v.values.ClusterType == ClusterTypeSeed {
		return nil
	}

	return kutil.DeleteObjects(ctx, v.client,
		v.emptyService(exporter),
	)
}

func (v *vpa) reconcileExporterService(service *corev1.Service) {
	service.Labels = getLabelsWithRole(exporter)
	service.Spec = corev1.ServiceSpec{
		Type:            corev1.ServiceTypeClusterIP,
		SessionAffinity: corev1.ServiceAffinityNone,
		Selector:        getLabels(exporter),
	}

	desiredPorts := []corev1.ServicePort{
		{
			Name:       "metrics",
			Protocol:   corev1.ProtocolTCP,
			Port:       exporterPortMetrics,
			TargetPort: intstr.FromInt(int(exporterPortMetrics)),
		},
	}
	service.Spec.Ports = kutil.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
}
