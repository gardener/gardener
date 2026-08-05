// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	timewindowutils "github.com/gardener/gardener/pkg/apis/utils/timewindow"
	hibernationutils "github.com/gardener/gardener/pkg/utils/hibernation"
)

// IsShootHibernatedDuringNextMaintenanceWindow reports whether the shoot could be in a hibernated state at any point
// during its next maintenance window. Since Gardener only guarantees that maintenance starts at some point within the
// window (not at its beginning), we compare against the end of the window — if a hibernation event falls before the
// window closes, maintenance may never run. It only checks the very next hibernation and wake-up events, so it may
// return false even if the shoot will be hibernated during the next maintenance window.
func IsShootHibernatedDuringNextMaintenanceWindow(shoot *gardencorev1beta1.Shoot, now time.Time) bool {
	if shoot.Spec.Maintenance == nil || shoot.Spec.Maintenance.TimeWindow == nil {
		return false
	}

	maintenanceWindow, err := timewindowutils.ParseMaintenanceTimeWindow(shoot.Spec.Maintenance.TimeWindow.Begin, shoot.Spec.Maintenance.TimeWindow.End)
	if err != nil {
		return false
	}
	nextMaintenanceBegin := maintenanceWindow.AdjustedBegin(now)
	if !nextMaintenanceBegin.After(now) {
		nextMaintenanceBegin = nextMaintenanceBegin.AddDate(0, 0, 1)
	}
	nextMaintenanceEnd := maintenanceWindow.AdjustedEnd(now)
	if !nextMaintenanceEnd.After(nextMaintenanceBegin) {
		nextMaintenanceEnd = nextMaintenanceEnd.AddDate(0, 0, 1)
	}

	nextHibernateTime, nextWakeUpTime, err := parseNextHibernationEvents(shoot, now)
	if err != nil {
		return false
	}

	if shoot.Status.IsHibernated {
		if nextWakeUpTime == nil || nextWakeUpTime.After(nextMaintenanceBegin) {
			return true // never wakes up, or wakes up too late
		}

		return nextHibernateTime != nil &&
			nextHibernateTime.After(*nextWakeUpTime) &&
			nextHibernateTime.Before(nextMaintenanceEnd)
	}

	if nextHibernateTime == nil || nextHibernateTime.After(nextMaintenanceEnd) {
		return false // stays awake all the way to maintenance
	}

	return nextWakeUpTime == nil ||
		nextWakeUpTime.Before(*nextHibernateTime) ||
		nextWakeUpTime.After(nextMaintenanceBegin)
}

func parseNextHibernationEvents(shoot *gardencorev1beta1.Shoot, now time.Time) (nextHibernateTime *time.Time, nextWakeUpTime *time.Time, err error) {
	if shoot.Spec.Hibernation == nil || len(shoot.Spec.Hibernation.Schedules) == 0 {
		return nil, nil, nil
	}
	schedules, err := hibernationutils.Parse(shoot.Spec.Hibernation.Schedules)
	if err != nil {
		return nil, nil, err
	}

	for _, s := range schedules {
		t := s.Next(now)
		switch s.Operation {
		case hibernationutils.Hibernate:
			if nextHibernateTime == nil || t.Before(*nextHibernateTime) {
				nextHibernateTime = &t
			}
		case hibernationutils.WakeUp:
			if nextWakeUpTime == nil || t.Before(*nextWakeUpTime) {
				nextWakeUpTime = &t
			}
		}
	}
	return nextHibernateTime, nextWakeUpTime, nil
}
