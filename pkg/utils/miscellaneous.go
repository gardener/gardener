// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"math/big"
	"net"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

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
	var values = make(map[string]interface{}, len(b))

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

// MergeStringMaps merges the content of the newMaps with the oldMap. If a key already exists then
// it gets overwritten by the last value with the same key.
func MergeStringMaps[T any](oldMap map[string]T, newMaps ...map[string]T) map[string]T {
	var out map[string]T

	if oldMap != nil {
		out = make(map[string]T, len(oldMap))
	}
	for k, v := range oldMap {
		out[k] = v
	}

	for _, newMap := range newMaps {
		if newMap != nil && out == nil {
			out = make(map[string]T)
		}

		for k, v := range newMap {
			out[k] = v
		}
	}

	return out
}

// TimeElapsed takes a <timestamp> and a <duration> checks whether the elapsed time until now is less than the <duration>.
// If yes, it returns true, otherwise it returns false.
func TimeElapsed(timestamp *metav1.Time, duration time.Duration) bool {
	if timestamp == nil {
		return true
	}

	var (
		end = metav1.NewTime(timestamp.Time.UTC().Add(duration))
		now = metav1.NewTime(time.Now().UTC())
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

// emailVefiryRegex is used to verify the validity of an email.
var emailVefiryRegex = regexp.MustCompile(`^[^@]+@(?:[a-zA-Z-0-9]+\.)+[a-zA-Z]{2,}$`)

// TestEmail validates the provided <email> against a regular expression and returns whether it matches.
func TestEmail(email string) bool {
	return emailVefiryRegex.MatchString(email)
}

// IDForKeyWithOptionalValue returns an identifier for the given key + optional value.
func IDForKeyWithOptionalValue(key string, value *string) string {
	v := ""
	if value != nil {
		v = "=" + *value
	}
	return key + v
}

// QuantityPtr returns a Quantity pointer to its argument.
func QuantityPtr(q resource.Quantity) *resource.Quantity {
	return &q
}

// ProtocolPtr returns a corev1.Protocol pointer to its argument.
func ProtocolPtr(protocol corev1.Protocol) *corev1.Protocol {
	return &protocol
}

// TimePtr returns a time.Time pointer to its argument.
func TimePtr(t time.Time) *time.Time {
	return &t
}

// TimePtrDeref dereferences the time.Time ptr and returns it if not nil, or else
// returns def.
func TimePtrDeref(ptr *time.Time, def time.Time) time.Time {
	if ptr != nil {
		return *ptr
	}
	return def
}

// IntStrPtrFromInt returns an intstr.IntOrString pointer to its argument.
func IntStrPtrFromInt(port int) *intstr.IntOrString {
	v := intstr.FromInt(port)
	return &v
}

// IntStrPtrFromString returns an intstr.IntOrString pointer to its argument.
func IntStrPtrFromString(port string) *intstr.IntOrString {
	v := intstr.FromString(port)
	return &v
}

// Indent indents the given string with the given number of spaces.
func Indent(str string, spaces int) string {
	return strings.ReplaceAll(str, "\n", "\n"+strings.Repeat(" ", spaces))
}

// ShallowCopyMapStringInterface creates a shallow copy of the given map.
func ShallowCopyMapStringInterface(values map[string]interface{}) map[string]interface{} {
	copiedValues := make(map[string]interface{}, len(values))
	for k, v := range values {
		copiedValues[k] = v
	}
	return copiedValues
}

// IifString returns onTrue if the condition is true, and onFalse otherwise.
// It is similar to the ternary operator (?:) and the IIF function (see https://en.wikipedia.org/wiki/IIf) in other languages.
func IifString(condition bool, onTrue, onFalse string) string {
	if condition {
		return onTrue
	}
	return onFalse
}

// InterfaceMapToStringMap translates map[string]interface{} to map[string]string.
func InterfaceMapToStringMap(in map[string]interface{}) map[string]string {
	m := make(map[string]string, len(in))
	for k, v := range in {
		m[k] = fmt.Sprint(v)
	}
	return m
}

// FilterEntriesByPrefix returns a list of strings which begin with the given prefix.
func FilterEntriesByPrefix(prefix string, entries []string) []string {
	var result []string
	for _, entry := range entries {
		if strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return result
}

// ComputeOffsetIP parses the provided <subnet> and offsets with the value of <offset>.
// For example, <subnet> = 100.64.0.0/11 and <offset> = 10 the result would be 100.64.0.10
// IPv6 and IPv4 is supported.
func ComputeOffsetIP(subnet *net.IPNet, offset int64) (net.IP, error) {
	if subnet == nil {
		return nil, fmt.Errorf("subnet is nil")
	}

	isIPv6 := false

	bytes := subnet.IP.To4()
	if bytes == nil {
		isIPv6 = true
		bytes = subnet.IP.To16()
	}

	ip := net.IP(big.NewInt(0).Add(big.NewInt(0).SetBytes(bytes), big.NewInt(offset)).Bytes())

	if !subnet.Contains(ip) {
		return nil, fmt.Errorf("cannot compute IP with offset %d - subnet %q too small", offset, subnet)
	}

	// there is no broadcast address on IPv6
	if isIPv6 {
		return ip, nil
	}

	for i := range ip {
		// IP address is not the same, so it's not the broadcast ip.
		if ip[i] != ip[i]|^subnet.Mask[i] {
			return ip.To4(), nil
		}
	}

	return nil, fmt.Errorf("computed IPv4 address %q is broadcast for subnet %q", ip, subnet)
}
