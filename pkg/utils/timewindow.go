// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/rand"
)

const maintenanceTimeLayout = "150405-0700"

// MaintenanceTime is a structure holding a maintenance time.
type MaintenanceTime struct {
	hour   int
	minute int
	second int
}

// NewMaintenanceTime returns a maintenance time structure based on the given hour, minute, and second.
func NewMaintenanceTime(hour, minute, second int) *MaintenanceTime {
	if hour >= 24 {
		panic(fmt.Sprintf("invalid hour %d", hour))
	}
	if minute >= 60 {
		panic(fmt.Sprintf("invalid minute %d", minute))
	}
	if second >= 60 {
		panic(fmt.Sprintf("invalid second %d", second))
	}
	return &MaintenanceTime{hour, minute, second}
}

// ParseMaintenanceTime parses the given value and returns it as MaintenanceTime object. In case the parsing fails, an
// error is returned. The time object is converted to UTC zone.
func ParseMaintenanceTime(value string) (*MaintenanceTime, error) {
	t, err := time.Parse(maintenanceTimeLayout, value)
	if err != nil {
		return nil, fmt.Errorf("Could not parse the value into the maintenanceTime format: %s", err.Error())
	}
	return timeToMaintenanceTime(t), nil
}

func timeToMaintenanceTime(t time.Time) *MaintenanceTime {
	t = t.UTC()
	return NewMaintenanceTime(t.Hour(), t.Minute(), t.Second())
}

// RandomMaintenanceTimeWindow computes a random time window and returns both in the format HHMMSS+ZONE.
func RandomMaintenanceTimeWindow() *MaintenanceTimeWindow {
	var (
		hour  = rand.IntnRange(0, 23)
		begin = NewMaintenanceTime(hour, 0, 0)
		end   = NewMaintenanceTime(hour+1, 0, 0)
	)
	return NewMaintenanceTimeWindow(begin, end)
}

// String returns the string representation of the maintenance time.
func (m *MaintenanceTime) String() string {
	return fmt.Sprintf("%.02d:%.02d:%.02d", m.hour, m.minute, m.second)
}

// Formatted formats the maintenance time object to the maintenance time format.
func (m *MaintenanceTime) Formatted() string {
	return m.zeroTime().Format(maintenanceTimeLayout)
}

func (m *MaintenanceTime) zeroTime() time.Time {
	return time.Date(1, time.January, 1, m.hour, m.minute, m.second, 0, time.UTC)
}

// Hour returns the hour of the maintenance time.
func (m *MaintenanceTime) Hour() int {
	return m.hour
}

// Minute returns the minute of the maintenance time.
func (m *MaintenanceTime) Minute() int {
	return m.minute
}

// Second returns the second of the maintenance time.
func (m *MaintenanceTime) Second() int {
	return m.second
}

// Add adds hour, minute and second to <m> and returns a new maintenance time.
func (m *MaintenanceTime) Add(hour, minute, second int) *MaintenanceTime {
	t := m.zeroTime().Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second)
	return timeToMaintenanceTime(t)
}

// Compare compares the two times <m> and <other>. It returns
// * i < 0 if <m> is before other
// * i = 0 if <m> is equal other
// * i > 0 if <m> is after other
func (m *MaintenanceTime) Compare(other *MaintenanceTime) int {
	if hourDiff := m.hour - other.hour; hourDiff != 0 {
		return hourDiff
	}
	if minuteDiff := m.minute - other.minute; minuteDiff != 0 {
		return minuteDiff
	}
	return m.second - other.second
}

func (m *MaintenanceTime) adjust(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), m.hour, m.minute, m.second, 0, t.Location())
}

// MaintenanceTimeWindow contains the beginning and the end of a time window in which maintenance operations can be performed.
type MaintenanceTimeWindow struct {
	begin *MaintenanceTime
	end   *MaintenanceTime
}

// AlwaysTimeWindow is a MaintenanceTimeWindow that contains all durations.
var AlwaysTimeWindow = NewMaintenanceTimeWindow(NewMaintenanceTime(0, 0, 0), NewMaintenanceTime(23, 59, 59))

// NewMaintenanceTimeWindow takes a begin and an end of a time window and returns a pointer to a MaintenanceTimeWindow structure.
func NewMaintenanceTimeWindow(begin, end *MaintenanceTime) *MaintenanceTimeWindow {
	return &MaintenanceTimeWindow{begin, end}
}

// ParseMaintenanceTimeWindow takes a begin and an end of a time window in the maintenance format and returns a pointer
// to a MaintenanceTimeWindow structure.
func ParseMaintenanceTimeWindow(begin, end string) (*MaintenanceTimeWindow, error) {
	maintenanceWindowBegin, err := ParseMaintenanceTime(begin)
	if err != nil {
		return nil, fmt.Errorf("Could not parse begin time: %s", err.Error())
	}
	maintenanceWindowEnd, err := ParseMaintenanceTime(end)
	if err != nil {
		return nil, fmt.Errorf("Could not parse end time: %s", err.Error())
	}
	return NewMaintenanceTimeWindow(maintenanceWindowBegin, maintenanceWindowEnd), nil
}

// String returns the string representation of the time window.
func (m *MaintenanceTimeWindow) String() string {
	return fmt.Sprintf("begin=%s, end=%s", m.begin, m.end)
}

// Begin returns the begin of the time window.
func (m *MaintenanceTimeWindow) Begin() *MaintenanceTime {
	return m.begin
}

// End returns the end of the time window.
func (m *MaintenanceTimeWindow) End() *MaintenanceTime {
	return m.end
}

// WithBegin returns a new maintenance time window with the given <begin> (ending will be kept).
func (m *MaintenanceTimeWindow) WithBegin(begin *MaintenanceTime) *MaintenanceTimeWindow {
	return NewMaintenanceTimeWindow(begin, m.end)
}

// WithEnd returns a new maintenance time window with the given <end> (beginning will be kept).
func (m *MaintenanceTimeWindow) WithEnd(end *MaintenanceTime) *MaintenanceTimeWindow {
	return NewMaintenanceTimeWindow(m.begin, end)
}

// Contains returns true in case the given time is within the time window.
func (m *MaintenanceTimeWindow) Contains(tTime time.Time) bool {
	t := timeToMaintenanceTime(tTime)

	if m.spansDifferentDays() {
		return !(t.Compare(m.end) > 0 && t.Compare(m.begin) < 0)
	}
	return t.Compare(m.begin) >= 0 && t.Compare(m.end) <= 0
}

var (
	// RandomFunc is a function that computes a random number.
	RandomFunc = rand.Int63nRange
)

// RandomDurationUntilNext computes the duration until a random time within the time window for the next maintenance
// execution.
func (m *MaintenanceTimeWindow) RandomDurationUntilNext(from time.Time) time.Duration {
	from = from.UTC()

	var (
		begin = m.adjustedBegin(from)
		end   = m.adjustedEnd(from)
	)

	if begin.Sub(from) < 0 && (m.Contains(from) || from.After(end)) {
		begin = begin.AddDate(0, 0, 1)
		end = end.AddDate(0, 0, 1)
	}

	delta := end.Sub(begin)
	return time.Duration(int64(begin.Sub(from)) + RandomFunc(0, delta.Nanoseconds()))
}

// Duration returns the duration of the maintenance time window.
func (m *MaintenanceTimeWindow) Duration() time.Duration {
	var (
		from  = time.Date(0, time.January, 1, 0, 0, 0, 0, time.UTC)
		begin = m.adjustedBegin(from)
		end   = m.adjustedEnd(from)
	)
	return end.Sub(begin)
}

func (m *MaintenanceTimeWindow) adjustedBegin(t time.Time) time.Time {
	return m.begin.adjust(t)
}

func (m *MaintenanceTimeWindow) adjustedEnd(t time.Time) time.Time {
	end := m.end.adjust(t)
	if m.end.Compare(m.begin) <= 0 {
		return end.AddDate(0, 0, 1)
	}
	return end
}

func (m *MaintenanceTimeWindow) spansDifferentDays() bool {
	return m.end.Compare(m.begin) < 0
}
