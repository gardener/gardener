// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
)

// GetWarnings returns warnings for the provided seed.
func GetWarnings(_ context.Context, seed, _ *core.Seed) []string {
	if seed == nil {
		return nil
	}

	var warnings []string

	warnings = append(warnings, getWarningsForDeprecatedDependencyWatchdogFields(seed)...)

	return warnings
}

func getWarningsForDeprecatedDependencyWatchdogFields(seed *core.Seed) []string {
	var warnings []string
	if seed.Spec.Settings.DependencyWatchdog.Endpoint != nil {
		warnings = append(warnings, "you are setting spec.settings.dependencyWatchdog.endpoint field. This field is deprecated and will be removed in future versions of Gardener. Instead, use the spec.settings.dependencyWatchdog.weeder field.")
	}

	if seed.Spec.Settings.DependencyWatchdog.Probe != nil {
		warnings = append(warnings, "you are setting spec.settings.dependencyWatchdog.probe field. This field is deprecated and will be removed in future versions of Gardener. Instead, use the spec.settings.dependencyWatchdog.prober field.")
	}
	return warnings
}
