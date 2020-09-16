// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"

	schedulerapi "github.com/gardener/gardener/pkg/scheduler/apis/config"
)

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *schedulerapi.SchedulerConfiguration) error {
	for _, strategy := range schedulerapi.Strategies {
		if strategy == config.Schedulers.Shoot.Strategy {
			return nil
		}
	}
	return fmt.Errorf("unknown seed determination strategy configured in gardener scheduler. Strategy: '%s' does not exist. Valid strategies are: %v", config.Schedulers.Shoot.Strategy, schedulerapi.Strategies)
}
