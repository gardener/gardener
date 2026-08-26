// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package hibernationschedule provides helpers for parsing and iterating over Shoot
// hibernation schedules expressed as cron expressions.
package hibernation

import (
	"time"

	"github.com/robfig/cron"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// Operation defines the type of operation that is scheduled by a hibernation schedule.
type Operation uint8

const (
	// Hibernate indicates that the cluster should be hibernated.
	Hibernate Operation = iota
	// WakeUp indicates that the cluster should be woken up.
	WakeUp
)

// ParsedSchedule holds the loaded location, parsed cron schedule and information whether
// the cluster should be hibernated or woken up.
type ParsedSchedule struct {
	Schedule  cron.Schedule
	Location  time.Location
	Operation Operation
}

// Next returns the time in UTC from the schedule, that is immediately after the input time 't'.
// The input 't' is converted in the schedule's location before any calculations are done.
func (s *ParsedSchedule) Next(t time.Time) time.Time {
	return s.Schedule.Next(t.In(&s.Location)).UTC()
}

// Previous returns the time in UTC from the schedule that is immediately before 'to' and after 'from'.
// Nil is returned if no such time can be found.
// The input times - 'to' and 'from' are converted in the schedule's location before any calculation is done.
func (s *ParsedSchedule) Previous(from, to time.Time) *time.Time {
	var last *time.Time
	for t := s.Schedule.Next(from.In(&s.Location)); !t.UTC().After(to.UTC()); t = s.Schedule.Next(t) {
		inUTC := t.UTC()
		last = &inUTC
	}
	return last
}

// Parse parses the given HibernationSchedules and returns an array of ParsedSchedules
// If the Location of a HibernationSchedule is `nil`, it is defaulted to UTC.
func Parse(schedules []gardencorev1beta1.HibernationSchedule) ([]ParsedSchedule, error) {
	var out []ParsedSchedule

	for _, sched := range schedules {
		locationID := time.UTC.String()
		if sched.Location != nil && *sched.Location != "" {
			locationID = *sched.Location
		}

		loc, err := time.LoadLocation(locationID)
		if err != nil {
			return nil, err
		}

		if sched.Start != nil {
			parsed, err := cron.ParseStandard(*sched.Start)
			if err != nil {
				return nil, err
			}
			out = append(out, ParsedSchedule{
				Schedule:  parsed,
				Location:  *loc,
				Operation: Hibernate,
			})
		}

		if sched.End != nil {
			parsed, err := cron.ParseStandard(*sched.End)
			if err != nil {
				return nil, err
			}
			out = append(out, ParsedSchedule{
				Schedule:  parsed,
				Location:  *loc,
				Operation: WakeUp,
			})
		}
	}

	return out, nil
}
