// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"github.com/go-logr/logr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
)

// NewPrometheus creates a new prometheus deployer.
func NewPrometheus(log logr.Logger, c client.Client, namespace string, values prometheus.Values) (prometheus.Interface, error) {
	imagePrometheus, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePrometheus)
	if err != nil {
		return nil, err
	}

	values.Image = imagePrometheus.String()
	values.Version = ptr.Deref(imagePrometheus.Version, "v0.0.0")

	// TODO(rfranzke): Remove this block after v1.97 has been released.
	{
		imageAlpine, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameAlpine)
		if err != nil {
			return nil, err
		}

		values.DataMigration.Client = c
		values.DataMigration.Namespace = namespace
		values.DataMigration.StorageCapacity = values.StorageCapacity
		values.DataMigration.ImageAlpine = imageAlpine.String()
		if values.DataMigration.StatefulSetName == "" {
			values.DataMigration.StatefulSetName = "prometheus"
		}
		if values.DataMigration.FullName == "" {
			values.DataMigration.FullName = "prometheus-" + values.Name
		}
		if values.DataMigration.PVCNames == nil {
			values.DataMigration.PVCNames = []string{"prometheus-db-" + values.DataMigration.StatefulSetName + "-0"}
		}
	}

	return prometheus.New(log, c, namespace, values), nil
}
