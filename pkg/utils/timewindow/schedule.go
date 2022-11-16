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

package timewindow

import (
	"fmt"
	"hash/crc32"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// DetermineSchedule determines a schedule based on the provided format and the creation timestamp. If both the begin
// and end of a maintenance time window are provided and different from the 'always time window' then the provided
// mutation function is applied.
func DetermineSchedule(
	scheduleFormat string,
	begin, end string,
	uid types.UID,
	creationTimestamp metav1.Time,
	mutate func(string, MaintenanceTimeWindow, types.UID) string,
) (
	string,
	error,
) {
	if len(begin) != 0 && len(end) != 0 {
		maintenanceTimeWindow, err := ParseMaintenanceTimeWindow(begin, end)
		if err != nil {
			return "", err
		}

		if !maintenanceTimeWindow.Equal(AlwaysTimeWindow) {
			return mutate(scheduleFormat, *maintenanceTimeWindow, uid), nil
		}
	}

	return fmt.Sprintf(scheduleFormat, creationTimestamp.Minute(), creationTimestamp.Hour()), nil
}

// RandomizeWithinTimeWindow computes a random time (based on the provided UID) within the provided time window.
func RandomizeWithinTimeWindow(scheduleFormat string, window MaintenanceTimeWindow, uid types.UID) string {
	var (
		windowBegin     = window.Begin()
		windowInMinutes = uint32(window.Duration().Minutes())
		randomMinutes   = int(crc32.ChecksumIEEE([]byte(uid)) % windowInMinutes)
		randomTime      = windowBegin.Add(0, randomMinutes, 0)
	)

	return fmt.Sprintf(scheduleFormat, randomTime.Minute(), randomTime.Hour())
}

// RandomizeWithinFirstHourOfTimeWindow computes a random time (based on the provided UID) within the first hour of the
// provided time window. It adds a 15 minutes time buffer before the start.
func RandomizeWithinFirstHourOfTimeWindow(scheduleFormat string, window MaintenanceTimeWindow, uid types.UID) string {
	var (
		windowBegin   = window.Begin().Add(0, -15, 0)
		randomMinutes = int(crc32.ChecksumIEEE([]byte(uid)) % 60)
		randomTime    = windowBegin.Add(-1, randomMinutes, 0)
	)

	return fmt.Sprintf(scheduleFormat, randomTime.Minute(), randomTime.Hour())
}
