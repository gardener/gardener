// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"slices"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	timewindowutils "github.com/gardener/gardener/pkg/apis/utils/timewindow"
	hibernationutils "github.com/gardener/gardener/pkg/utils/hibernation"
)

// Collect all events inside [simStart, simEnd) and sort by time.
type hibernationScheduleEvent struct {
	at        time.Time
	operation hibernationutils.Operation
}

// IsShootAlwaysHibernatedDuringMaintenance returns true when the hibernation schedule of the shoot is such that the shoot is
// always hibernated during the maintenance window.
//
// Because the hibernation schedules are defined by cron schedules, we determine this by running a simulation of all
// wake-up and hibernation events and checking if any awake interval fully covers the maintenance window.
func IsShootAlwaysHibernatedDuringMaintenance(shoot *gardencorev1beta1.Shoot) bool {
	if shoot.Spec.Maintenance == nil || shoot.Spec.Maintenance.TimeWindow == nil {
		return false
	}
	if shoot.Spec.Hibernation == nil || len(shoot.Spec.Hibernation.Schedules) == 0 {
		return false
	}

	maintenanceWindow, err := timewindowutils.ParseMaintenanceTimeWindow(
		shoot.Spec.Maintenance.TimeWindow.Begin,
		shoot.Spec.Maintenance.TimeWindow.End,
	)
	if err != nil {
		return false
	}

	schedules, err := hibernationutils.Parse(shoot.Spec.Hibernation.Schedules)
	if err != nil || len(schedules) == 0 {
		return false
	}

	// If there are no wake-up events at all we assume it's never awake during maintenance and return true
	if hasWakeEvent := slices.ContainsFunc(schedules, func(s hibernationutils.ParsedSchedule) bool {
		return s.Operation == hibernationutils.WakeUp
	}); !hasWakeEvent {
		return true
	}

	events, ok := simulateHibernationSchedule(schedules)
	if !ok {
		return false
	}

	return isAlwaysHibernatedDuringMaintenance(maintenanceWindow, events)
}

func simulateHibernationSchedule(schedules []hibernationutils.ParsedSchedule) (events []hibernationScheduleEvent, ok bool) {
	const (
		maxCronIter    = 10_000
		simulationDays = 35 // should cover most realistic scenarios.
	)

	start := time.Date(2006, time.January, 2, 0, 0, 0, 0, time.UTC) // monday
	end := start.Add(simulationDays * 24 * time.Hour)

	// make sure there is a hibernate event end of the simulation window, so that the last interval is always closed
	events = []hibernationScheduleEvent{
		{at: end, operation: hibernationutils.Hibernate},
	}
	iter := 0
	for _, s := range schedules {
		for t := s.Next(start); t.Before(end); t = s.Next(t) {
			iter++
			if iter > maxCronIter {
				return nil, false // indicate that we reached the simulation limit
			}
			events = append(events, hibernationScheduleEvent{at: t, operation: s.Operation})
		}
	}
	slices.SortFunc(events, func(a, b hibernationScheduleEvent) int { return a.at.Compare(b.at) })
	events = slices.CompactFunc(events, func(e1, e2 hibernationScheduleEvent) bool {
		return e1.operation == e2.operation
	})
	return events, true
}

func isAlwaysHibernatedDuringMaintenance(maintenanceWindow *timewindowutils.MaintenanceTimeWindow, events []hibernationScheduleEvent) bool {
	maintenanceDuration := maintenanceWindow.Duration()

	// iterate over all awake intervals. Since we know the simulation always contains alternating events, we can iterate
	// with stepsize 2.
	start := 0
	if events[0].operation == hibernationutils.Hibernate {
		start = 1
	}
	for i := start; i < len(events)-1; i += 2 {
		awakeIntervalStart := events[i].at
		awakeIntervalEnd := events[i+1].at

		// Find the first maintenance window start on or after intervalStart.
		maintenanceStart := maintenanceWindow.AdjustedBegin(awakeIntervalStart)
		if maintenanceStart.Before(awakeIntervalStart) {
			maintenanceStart = maintenanceStart.Add(24 * time.Hour)
		}
		// If the maintenance window ends before the end of the awake interval, then the maintenance window fits
		// entirely inside the awake interval
		if maintenanceStart.Add(maintenanceDuration).Before(awakeIntervalEnd) {
			return false
		}
	}

	return true
}
