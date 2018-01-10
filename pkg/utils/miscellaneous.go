// Copyright 2018 The Gardener Authors.
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
)

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

// MergeMaps takes two maps <defaults>, <custom> and merges them. If <custom> defines a value with a key
// already existing in the <defaults> map, the <default> value for that key will be overwritten.
func MergeMaps(defaults, custom map[string]interface{}) map[string]interface{} {
	var values = map[string]interface{}{}
	for i, v := range defaults {
		values[i] = v
	}
	for i, v := range custom {
		values[i] = v
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
