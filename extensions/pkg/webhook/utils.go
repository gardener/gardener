// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// LogMutation provides a log message.
func LogMutation(logger logr.Logger, kind, namespace, name string) {
	logger.Info("Mutating resource", "kind", kind, "namespace", namespace, "name", name)
}

// AppendUniqueUnit appends a unit only if it does not exist.
func AppendUniqueUnit(units *[]extensionsv1alpha1.Unit, unit extensionsv1alpha1.Unit) {
	for _, un := range *units {
		if un.Name == unit.Name {
			return
		}
	}
	*units = append(*units, unit)
}

// splitCommandLineRegex is used to split command line arguments by white space or "\".
var splitCommandLineRegex = regexp.MustCompile(`[\\\s]+`)

// DeserializeCommandLine de-serializes the given string to a slice of command line elements by splitting it
// on white space and the "\" character.
func DeserializeCommandLine(s string) []string {
	return splitCommandLineRegex.Split(s, -1)
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

// ContainerWithName returns the first container with the specified name from the slice, or nil if not found.
func ContainerWithName(containers []corev1.Container, name string) *corev1.Container {
	for i, container := range containers {
		if container.Name == name {
			return &containers[i]
		}
	}
	return nil
}

// PVCWithName returns the first PersistentVolumeClaim with the specified name from the slice, or nil if not found.
func PVCWithName(pvcs []corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	for i, pvc := range pvcs {
		if pvc.Name == name {
			return &pvcs[i]
		}
	}
	return nil
}

// UnitWithName returns the first unit with the specified name from the slice, or nil if not found.
func UnitWithName(units []extensionsv1alpha1.Unit, name string) *extensionsv1alpha1.Unit {
	for i, unit := range units {
		if unit.Name == name {
			return &units[i]
		}
	}
	return nil
}

// FileWithPath returns the first file with the specified path from the slice, or nil if not found.
func FileWithPath(files []extensionsv1alpha1.File, path string) *extensionsv1alpha1.File {
	for i, file := range files {
		if file.Path == path {
			return &files[i]
		}
	}
	return nil
}

// UnitOptionWithSectionAndName returns the first unit option with the specified section and name from the slice, or nil if not found.
func UnitOptionWithSectionAndName(opts []*unit.UnitOption, section, name string) *unit.UnitOption {
	for i, opt := range opts {
		if opt.Section == section && opt.Name == name {
			return opts[i]
		}
	}
	return nil
}

// EnsureStringWithPrefix ensures that a string having the given prefix exists in the given slice
// and all matches are with a value equal to prefix + value.
func EnsureStringWithPrefix(items []string, prefix, value string) []string {
	if StringWithPrefixIndex(items, prefix) < 0 {
		return append(items, prefix+value)
	}

	for i, item := range items {
		if strings.HasPrefix(item, prefix) {
			items[i] = prefix + value
		}
	}
	return items
}

// EnsureNoStringWithPrefix ensures that a string having the given prefix does not exist in the given slice.
func EnsureNoStringWithPrefix(items []string, prefix string) []string {
	return slices.DeleteFunc(items, func(s string) bool {
		return strings.HasPrefix(s, prefix)
	})
}

// EnsureStringWithPrefixContains ensures that a string having the given prefix exists in the given slice
// and all matches contain the given value in a list separated by sep.
func EnsureStringWithPrefixContains(items []string, prefix, value, sep string) []string {
	if StringWithPrefixIndex(items, prefix) < 0 {
		return append(items, prefix+value)
	}

	for i, item := range items {
		if !strings.HasPrefix(item, prefix) {
			continue
		}
		values := strings.Split(strings.TrimPrefix(items[i], prefix), sep)
		if slices.Index(values, value) < 0 {
			values = append(values, value)
			items[i] = prefix + strings.Join(values, sep)
		}
	}
	return items
}

// EnsureNoStringWithPrefixContains ensures that either a string having the given prefix does not exist in the given slice,
// or it doesn't contain the given value in a list separated by sep.
func EnsureNoStringWithPrefixContains(items []string, prefix, value, sep string) []string {
	for i, item := range items {
		if !strings.HasPrefix(item, prefix) {
			continue
		}
		values := strings.Split(strings.TrimPrefix(items[i], prefix), sep)
		if j := slices.Index(values, value); j >= 0 {
			values = append(values[:j], values[j+1:]...)
			items[i] = prefix + strings.Join(values, sep)
		}
	}
	return items
}

// EnsureEnvVarWithName ensures that a EnvVar with a name equal to the name of the given EnvVar exists
// in the given slice and the first item in the list would be equal to the given EnvVar.
func EnsureEnvVarWithName(items []corev1.EnvVar, item corev1.EnvVar) []corev1.EnvVar {
	i := slices.IndexFunc(items, func(ev corev1.EnvVar) bool {
		return ev.Name == item.Name
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
	return items
}

// EnsureNoEnvVarWithName ensures that a EnvVar with the given name does not exist in the given slice.
func EnsureNoEnvVarWithName(items []corev1.EnvVar, name string) []corev1.EnvVar {
	return slices.DeleteFunc(items, func(ev corev1.EnvVar) bool {
		return ev.Name == name
	})
}

// EnsureVolumeMountWithName ensures that a VolumeMount with a name equal to the name of the given VolumeMount exists
// in the given slice and the first item in the list would be equal to the given VolumeMount.
func EnsureVolumeMountWithName(items []corev1.VolumeMount, item corev1.VolumeMount) []corev1.VolumeMount {
	i := slices.IndexFunc(items, func(vm corev1.VolumeMount) bool {
		return vm.Name == item.Name
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
	return items
}

// EnsureNoVolumeMountWithName ensures that a VolumeMount with the given name does not exist in the given slice.
func EnsureNoVolumeMountWithName(items []corev1.VolumeMount, name string) []corev1.VolumeMount {
	return slices.DeleteFunc(items, func(vm corev1.VolumeMount) bool {
		return vm.Name == name
	})
}

// EnsureVolumeWithName ensures that a Volume with a name equal to the name of the given Volume exists
// in the given slice and the first item in the list would be equal to the given Volume.
func EnsureVolumeWithName(items []corev1.Volume, item corev1.Volume) []corev1.Volume {
	i := slices.IndexFunc(items, func(v corev1.Volume) bool {
		return v.Name == item.Name
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
	return items
}

// EnsureNoVolumeWithName ensures that a Volume with the given name does not exist in the given slice.
func EnsureNoVolumeWithName(items []corev1.Volume, name string) []corev1.Volume {
	return slices.DeleteFunc(items, func(v corev1.Volume) bool {
		return v.Name == name
	})
}

// EnsureContainerWithName ensures that a Container with a name equal to the name of the given Container exists
// in the given slice and the first item in the list would be equal to the given Container.
func EnsureContainerWithName(items []corev1.Container, item corev1.Container) []corev1.Container {
	i := slices.IndexFunc(items, func(c corev1.Container) bool {
		return c.Name == item.Name
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
	return items
}

// EnsureNoContainerWithName ensures that a Container with the given name does not exist in the given slice.
func EnsureNoContainerWithName(items []corev1.Container, name string) []corev1.Container {
	return slices.DeleteFunc(items, func(c corev1.Container) bool {
		return c.Name == name
	})
}

// EnsureVPAContainerResourcePolicyWithName ensures that a container policy with a name equal to the name of the given
// container policy exists in the given slice and the first item in the list would be equal to the given container policy.
func EnsureVPAContainerResourcePolicyWithName(items []vpaautoscalingv1.ContainerResourcePolicy, item vpaautoscalingv1.ContainerResourcePolicy) []vpaautoscalingv1.ContainerResourcePolicy {
	i := slices.IndexFunc(items, func(crp vpaautoscalingv1.ContainerResourcePolicy) bool {
		return crp.ContainerName == item.ContainerName
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
	return items
}

// EnsurePVCWithName ensures that a PVC with a name equal to the name of the given PVC exists
// in the given slice and the first item in the list would be equal to the given PVC.
func EnsurePVCWithName(items []corev1.PersistentVolumeClaim, item corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	i := slices.IndexFunc(items, func(pvc corev1.PersistentVolumeClaim) bool {
		return pvc.Name == item.Name
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
	return items
}

// EnsureNoPVCWithName ensures that a PVC with the given name does not exist in the given slice.
func EnsureNoPVCWithName(items []corev1.PersistentVolumeClaim, name string) []corev1.PersistentVolumeClaim {
	return slices.DeleteFunc(items, func(pvc corev1.PersistentVolumeClaim) bool {
		return pvc.Name == name
	})
}

// EnsureUnitOption ensures the given unit option exist in the given slice.
func EnsureUnitOption(items []*unit.UnitOption, item *unit.UnitOption) []*unit.UnitOption {
	i := slices.IndexFunc(items, func(uo *unit.UnitOption) bool {
		return reflect.DeepEqual(uo, item)
	})
	if i < 0 {
		return append(items, item)
	}
	return items
}

// EnsureFileWithPath ensures that a file with a path equal to the path of the given file exists in the given slice
// and is equal to the given file.
func EnsureFileWithPath(items []extensionsv1alpha1.File, item extensionsv1alpha1.File) []extensionsv1alpha1.File {
	i := slices.IndexFunc(items, func(f extensionsv1alpha1.File) bool {
		return f.Path == item.Path
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
	return items
}

// EnsureUnitWithName ensures that an unit with a name equal to the name of the given unit exists in the given slice
// and is equal to the given unit.
func EnsureUnitWithName(items []extensionsv1alpha1.Unit, item extensionsv1alpha1.Unit) []extensionsv1alpha1.Unit {
	i := slices.IndexFunc(items, func(u extensionsv1alpha1.Unit) bool {
		return u.Name == item.Name
	})
	if i < 0 {
		return append(items, item)
	}
	items[i] = item
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

// StringWithPrefixIndex returns the index of the first occurrence of a string having the given prefix in the given slice, or -1 if not found.
func StringWithPrefixIndex(items []string, prefix string) int {
	return slices.IndexFunc(items, func(s string) bool {
		return strings.HasPrefix(s, prefix)
	})
}
