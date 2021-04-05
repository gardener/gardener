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

package etcd

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// DependencyWatchdogConfiguration returns the configuration for the dependency watchdog ensuring that its dependant
// pods are restarted as soon as it recovers from a crash loop.
func DependencyWatchdogConfiguration(role string) (string, error) {
	return ServiceName(role) + `:
  dependantPods:
  - name: ` + v1beta1constants.GardenRoleControlPlane + `
    selector:
      matchExpressions:
      - key: ` + v1beta1constants.GardenRole + `
        operator: In
        values:
        - ` + v1beta1constants.GardenRoleControlPlane + `
      - key: ` + v1beta1constants.LabelRole + `
        operator: In
        values:
        - ` + v1beta1constants.LabelAPIServer + `
`, nil
}
