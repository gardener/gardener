// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	imagePrometheus, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}

	values.Image = imagePrometheus.String()
	values.Version = ptr.Deref(imagePrometheus.Version, "v0.0.0")

	// TODO(rfranzke): Remove this block after all Prometheis have been migrated.
	{
		imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine)
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
