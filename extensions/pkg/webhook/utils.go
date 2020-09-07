// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhook

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

// LogMutation provides a log message.
func LogMutation(logger logr.Logger, kind, namespace, name string) {
	logger.Info("Mutating resource", "kind", kind, "namespace", namespace, "name", name)
}

// AppendUniqueUnit appens a unit only if it does not exist.
func AppendUniqueUnit(units *[]extensionsv1alpha1.Unit, unit extensionsv1alpha1.Unit) {
	for _, un := range *units {
		if un.Name == unit.Name {
			return
		}
	}
	*units = append(*units, unit)
}

// DeserializeCommandLine de-serializes the given string to a slice of command line elements by splitting it
// on white space and the "\" character.
func DeserializeCommandLine(s string) []string {
	return regexp.MustCompile(`[\\\s]+`).Split(s, -1)
}

// SerializeCommandLine serializes the given command line elements slice to a string by joining the first
// n+1 elements with a space " ", and all subsequent elements with the given separator.
func SerializeCommandLine(command []string, n int, sep string) string {
	if len(command) <= n {
		return strings.Join(command, " ")
	}
	if n == 0 {
		return strings.Join(command, sep)
	}
	return strings.Join(command[0:n], " ") + " " + strings.Join(command[n:], sep)
}

// ContainerWithName returns the container with the given name if it exists in the given slice, nil otherwise.
func ContainerWithName(containers []corev1.Container, name string) *corev1.Container {
	if i := containerWithNameIndex(containers, name); i >= 0 {
		return &containers[i]
	}
	return nil
}

// PVCWithName returns the PersistentVolumeClaim with the given name if it exists in the given slice, nil otherwise.
func PVCWithName(pvcs []corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	if i := pvcWithNameIndex(pvcs, name); i >= 0 {
		return &pvcs[i]
	}
	return nil
}

// UnitWithName returns the unit with the given name if it exists in the given slice, nil otherwise.
func UnitWithName(units []extensionsv1alpha1.Unit, name string) *extensionsv1alpha1.Unit {
	if i := unitWithNameIndex(units, name); i >= 0 {
		return &units[i]
	}
	return nil
}

// FileWithPath returns the file with the given path if it exists in the given slice, nil otherwise.
func FileWithPath(files []extensionsv1alpha1.File, path string) *extensionsv1alpha1.File {
	if i := fileWithPathIndex(files, path); i >= 0 {
		return &files[i]
	}
	return nil
}

// UnitOptionWithSectionAndName returns the unit option with the given section and name if it exists in the given slice, nil otherwise.
func UnitOptionWithSectionAndName(opts []*unit.UnitOption, section, name string) *unit.UnitOption {
	if i := unitOptionWithSectionAndNameIndex(opts, section, name); i >= 0 {
		return opts[i]
	}
	return nil
}

// EnsureStringWithPrefix ensures that a string having the given prefix exists in the given slice
// with a value equal to prefix + value.
func EnsureStringWithPrefix(items []string, prefix, value string) []string {
	item := prefix + value
	if i := StringWithPrefixIndex(items, prefix); i < 0 {
		items = append(items, item)
	} else if items[i] != item {
		items = append(append(items[:i], item), items[i+1:]...)
	}
	return items
}

// EnsureNoStringWithPrefix ensures that a string having the given prefix does not exist in the given slice.
func EnsureNoStringWithPrefix(items []string, prefix string) []string {
	if i := StringWithPrefixIndex(items, prefix); i >= 0 {
		items = append(items[:i], items[i+1:]...)
	}
	return items
}

// EnsureStringWithPrefixContains ensures that a string having the given prefix exists in the given slice
// and contains the given value in a list separated by sep.
func EnsureStringWithPrefixContains(items []string, prefix, value, sep string) []string {
	if i := StringWithPrefixIndex(items, prefix); i < 0 {
		items = append(items, prefix+value)
	} else {
		valuesList := strings.TrimPrefix(items[i], prefix)
		var values []string
		if valuesList != "" {
			values = strings.Split(valuesList, sep)
		}
		if j := StringIndex(values, value); j < 0 {
			values = append(values, value)
			items = append(append(items[:i], prefix+strings.Join(values, sep)), items[i+1:]...)
		}
	}
	return items
}

// EnsureNoStringWithPrefixContains ensures that either a string having the given prefix does not exist in the given slice,
// or it doesn't contain the given value in a list separated by sep.
func EnsureNoStringWithPrefixContains(items []string, prefix, value, sep string) []string {
	if i := StringWithPrefixIndex(items, prefix); i >= 0 {
		values := strings.Split(strings.TrimPrefix(items[i], prefix), sep)
		if j := StringIndex(values, value); j >= 0 {
			values = append(values[:j], values[j+1:]...)
			items = append(append(items[:i], prefix+strings.Join(values, sep)), items[i+1:]...)
		}
	}
	return items
}

// EnsureEnvVarWithName ensures that a EnvVar with a name equal to the name of the given EnvVar exists
// in the given slice and is equal to the given EnvVar.
func EnsureEnvVarWithName(items []corev1.EnvVar, item corev1.EnvVar) []corev1.EnvVar {
	if i := envVarWithNameIndex(items, item.Name); i < 0 {
		items = append(items, item)
	} else if !reflect.DeepEqual(items[i], item) {
		items = append(append(items[:i], item), items[i+1:]...)
	}
	return items
}

// EnsureNoEnvVarWithName ensures that a EnvVar with the given name does not exist in the given slice.
func EnsureNoEnvVarWithName(items []corev1.EnvVar, name string) []corev1.EnvVar {
	if i := envVarWithNameIndex(items, name); i >= 0 {
		items = append(items[:i], items[i+1:]...)
	}
	return items
}

// EnsureVolumeMountWithName ensures that a VolumeMount with a name equal to the name of the given VolumeMount exists
// in the given slice and is equal to the given VolumeMount.
func EnsureVolumeMountWithName(items []corev1.VolumeMount, item corev1.VolumeMount) []corev1.VolumeMount {
	if i := volumeMountWithNameIndex(items, item.Name); i < 0 {
		items = append(items, item)
	} else if !reflect.DeepEqual(items[i], item) {
		items = append(append(items[:i], item), items[i+1:]...)
	}
	return items
}

// EnsureNoVolumeMountWithName ensures that a VolumeMount with the given name does not exist in the given slice.
func EnsureNoVolumeMountWithName(items []corev1.VolumeMount, name string) []corev1.VolumeMount {
	if i := volumeMountWithNameIndex(items, name); i >= 0 {
		items = append(items[:i], items[i+1:]...)
	}
	return items
}

// EnsureVolumeWithName ensures that a Volume with a name equal to the name of the given Volume exists
// in the given slice and is equal to the given Volume.
func EnsureVolumeWithName(items []corev1.Volume, item corev1.Volume) []corev1.Volume {
	if i := volumeWithNameIndex(items, item.Name); i < 0 {
		items = append(items, item)
	} else if !reflect.DeepEqual(items[i], item) {
		items = append(append(items[:i], item), items[i+1:]...)
	}
	return items
}

// EnsureNoVolumeWithName ensures that a Volume with the given name does not exist in the given slice.
func EnsureNoVolumeWithName(items []corev1.Volume, name string) []corev1.Volume {
	if i := volumeWithNameIndex(items, name); i >= 0 {
		items = append(items[:i], items[i+1:]...)
	}
	return items
}

// EnsureContainerWithName ensures that a Container with a name equal to the name of the given Container exists
// in the given slice and is equal to the given Container.
func EnsureContainerWithName(items []corev1.Container, item corev1.Container) []corev1.Container {
	if i := containerWithNameIndex(items, item.Name); i < 0 {
		items = append(items, item)
	} else if !reflect.DeepEqual(items[i], item) {
		items = append(append(items[:i], item), items[i+1:]...)
	}
	return items
}

// EnsureNoContainerWithName ensures that a Container with the given name does not exist in the given slice.
func EnsureNoContainerWithName(items []corev1.Container, name string) []corev1.Container {
	if i := containerWithNameIndex(items, name); i >= 0 {
		items = append(items[:i], items[i+1:]...)
	}
	return items
}

// EnsurePVCWithName ensures that a PVC with a name equal to the name of the given PVC exists
// in the given slice and is equal to the given PVC.
func EnsurePVCWithName(items []corev1.PersistentVolumeClaim, item corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	if i := pvcWithNameIndex(items, item.Name); i < 0 {
		items = append(items, item)
	} else if !reflect.DeepEqual(items[i], item) {
		items = append(append(items[:i], item), items[i+1:]...)
	}
	return items
}

// EnsureNoPVCWithName ensures that a PVC with the given name does not exist in the given slice.
func EnsureNoPVCWithName(items []corev1.PersistentVolumeClaim, name string) []corev1.PersistentVolumeClaim {
	if i := pvcWithNameIndex(items, name); i >= 0 {
		items = append(items[:i], items[i+1:]...)
	}
	return items
}

// EnsureUnitOption ensures the given unit option exist in the given slice.
func EnsureUnitOption(items []*unit.UnitOption, item *unit.UnitOption) []*unit.UnitOption {
	if i := unitOptionIndex(items, item); i < 0 {
		items = append(items, item)
	}
	return items
}

// EnsureFileWithPath ensures that a file with a path equal to the path of the given file exists in the given slice
// and is equal to the given file.
func EnsureFileWithPath(items []extensionsv1alpha1.File, item extensionsv1alpha1.File) []extensionsv1alpha1.File {
	if i := fileWithPathIndex(items, item.Path); i < 0 {
		items = append(items, item)
	} else if !reflect.DeepEqual(items[i], item) {
		items = append(append(items[:i], item), items[i+1:]...)
	}
	return items
}

// EnsureAnnotationOrLabel ensures the given key/value exists in the annotationOrLabelMap map.
func EnsureAnnotationOrLabel(annotationOrLabelMap map[string]string, key, value string) map[string]string {
	if annotationOrLabelMap == nil {
		annotationOrLabelMap = make(map[string]string, 1)
	}
	annotationOrLabelMap[key] = value
	return annotationOrLabelMap
}

// StringIndex returns the index of the first occurrence of the given string in the given slice, or -1 if not found.
func StringIndex(items []string, value string) int {
	for i, item := range items {
		if item == value {
			return i
		}
	}
	return -1
}

// StringWithPrefixIndex returns the index of the first occurrence of a string having the given prefix in the given slice, or -1 if not found.
func StringWithPrefixIndex(items []string, prefix string) int {
	for i, item := range items {
		if strings.HasPrefix(item, prefix) {
			return i
		}
	}
	return -1
}

func containerWithNameIndex(items []corev1.Container, name string) int {
	for i, item := range items {
		if item.Name == name {
			return i
		}
	}
	return -1
}

func unitWithNameIndex(items []extensionsv1alpha1.Unit, name string) int {
	for i, item := range items {
		if item.Name == name {
			return i
		}
	}
	return -1
}

func fileWithPathIndex(items []extensionsv1alpha1.File, path string) int {
	for i, item := range items {
		if item.Path == path {
			return i
		}
	}
	return -1
}

func unitOptionWithSectionAndNameIndex(items []*unit.UnitOption, section, name string) int {
	for i, item := range items {
		if item.Section == section && item.Name == name {
			return i
		}
	}
	return -1
}

func unitOptionIndex(items []*unit.UnitOption, item *unit.UnitOption) int {
	for i := range items {
		if reflect.DeepEqual(items[i], item) {
			return i
		}
	}
	return -1
}

func envVarWithNameIndex(items []corev1.EnvVar, name string) int {
	for i, item := range items {
		if item.Name == name {
			return i
		}
	}
	return -1
}

func volumeMountWithNameIndex(items []corev1.VolumeMount, name string) int {
	for i, item := range items {
		if item.Name == name {
			return i
		}
	}
	return -1
}

func volumeWithNameIndex(items []corev1.Volume, name string) int {
	for i, item := range items {
		if item.Name == name {
			return i
		}
	}
	return -1
}

func pvcWithNameIndex(items []corev1.PersistentVolumeClaim, name string) int {
	for i, item := range items {
		if item.Name == name {
			return i
		}
	}
	return -1
}
