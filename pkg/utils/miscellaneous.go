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
	"net"
	"reflect"
	"regexp"
	"runtime"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

const maintenanceTimeLayout = "150405-0700"

// FuncName takes a function <f> as input and returns its name as a string. If the function is a method
// of a struct, the struct will be also prefixed, e.g. 'Botanist.CreateNamespace'.
func FuncName(f interface{}) string {
	funcName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	re := regexp.MustCompile(`^.*\.(\(.*)\-.*$`)
	match := re.FindStringSubmatch(funcName)
	if len(match) > 1 {
		return match[1]
	}
	return funcName
}

// ValueExists returns true or false, depending on whether the given string <value>
// is part of the given []string list <list>.
func ValueExists(value string, list []string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// MergeMaps takes two maps <a>, <b> and merges them. If <b> defines a value with a key
// already existing in the <a> map, the <a> value for that key will be overwritten.
func MergeMaps(a, b map[string]interface{}) map[string]interface{} {
	var values = map[string]interface{}{}

	for i, v := range b {
		existing, ok := a[i]
		values[i] = v

		switch elem := v.(type) {
		case map[string]interface{}:
			if ok {
				if extMap, ok := existing.(map[string]interface{}); ok {
					values[i] = MergeMaps(extMap, elem)
				}
			}
		default:
			values[i] = v
		}
	}

	for i, v := range a {
		if _, ok := values[i]; !ok {
			values[i] = v
		}
	}

	return values
}

// TimeElapsed takes a <timestamp> and a <duration> checks whether the elapsed time until now is less than the <duration>.
// If yes, it returns true, otherwise it returns false.
func TimeElapsed(timestamp *metav1.Time, duration time.Duration) bool {
	if timestamp == nil {
		return true
	}

	var (
		end = metav1.NewTime(timestamp.Time.Add(duration))
		now = metav1.Now()
	)
	return !now.Before(&end)
}

// FindFreePort finds a free port on the host machine and returns it.
func FindFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// TestEmail validates the provided <email> against a regular expression and returns whether it matches.
func TestEmail(email string) bool {
	match, _ := regexp.MatchString(`^[^@]+@(?:[a-zA-Z-0-9]+\.)+[a-zA-Z]{2,}$`, email)
	return match
}

// ComputeRandomTimeWindow computes a random time window and returns both in the format HHMMSS+ZONE.
func ComputeRandomTimeWindow() (string, string) {
	t := time.Date(1970, 1, 1, rand.IntnRange(0, 23), 0, 0, 0, time.UTC)
	return FormatMaintenanceTime(t), FormatMaintenanceTime(t.Add(time.Hour))
}

// FormatMaintenanceTime formats a time object to the maintenance time format.
func FormatMaintenanceTime(t time.Time) string {
	return t.Format(maintenanceTimeLayout)
}

// ParseMaintenanceTime parses the maintenance time and returns it as Time object. In case the parse fails, an
// error is returned. The time object is converted to UTC zone.
func ParseMaintenanceTime(value string) (time.Time, error) {
	timeInZone, err := time.Parse(maintenanceTimeLayout, value)
	if err != nil {
		return timeInZone, err
	}

	timeInUTC := timeInZone.UTC()
	return time.Date(0, time.January, 1, timeInUTC.Hour(), timeInUTC.Minute(), timeInUTC.Second(), timeInUTC.Nanosecond(), timeInUTC.Location()), nil
}
