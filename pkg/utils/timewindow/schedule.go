// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package timewindow

import (
	"fmt"
	"hash/crc32"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// MutateScheduleFunc is a function for mutating the schedule based on the maintenance time window and UID.
type MutateScheduleFunc func(string, MaintenanceTimeWindow, types.UID) string

// DetermineSchedule determines a schedule based on the provided format and the creation timestamp. If both the begin
// and end of a maintenance time window are provided and different from the 'always time window' then the provided
// mutation function is applied.
func DetermineSchedule(
	scheduleFormat string,
	begin, end string,
	uid types.UID,
	creationTimestamp metav1.Time,
	mutate MutateScheduleFunc,
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
