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
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
)

// NewAlertmanager creates a new alertmanager deployer.
func NewAlertmanager(log logr.Logger, c client.Client, namespace string, values alertmanager.Values) (alertmanager.Interface, error) {
	imageAlertmanager, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlertmanager)
	if err != nil {
		return nil, err
	}

	values.Image = imageAlertmanager.String()
	values.Version = ptr.Deref(imageAlertmanager.Version, "v0.0.0")

	// TODO(rfranzke): Remove this block after v1.93 has been released.
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
			values.DataMigration.StatefulSetName = "alertmanager"
		}
		if values.DataMigration.FullName == "" {
			values.DataMigration.FullName = "alertmanager-" + values.Name
		}
		if values.DataMigration.PVCNames == nil {
			values.DataMigration.PVCNames = []string{"alertmanager-db-alertmanager-0"}
		}
	}

	return alertmanager.New(log, c, namespace, values), nil
}
