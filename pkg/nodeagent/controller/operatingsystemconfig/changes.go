// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	componentscontainerd "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

func extractOSCFromSecret(secret *corev1.Secret) (*extensionsv1alpha1.OperatingSystemConfig, []byte, string, error) {
	oscRaw, ok := secret.Data[nodeagentv1alpha1.DataKeyOperatingSystemConfig]
	if !ok {
		return nil, nil, "", fmt.Errorf("no %s key found in OSC secret", nodeagentv1alpha1.DataKeyOperatingSystemConfig)
	}

	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(decoder, oscRaw, osc); err != nil {
		return nil, nil, "", fmt.Errorf("unable to decode OSC from secret data key %s: %w", nodeagentv1alpha1.DataKeyOperatingSystemConfig, err)
	}

	return osc, oscRaw, secret.Annotations[nodeagentv1alpha1.AnnotationKeyChecksumDownloadedOperatingSystemConfig], nil
}

type operatingSystemConfigChanges struct {
	units      units
	files      files
	containerd containerd
}

type units struct {
	changed []changedUnit
	deleted []extensionsv1alpha1.Unit
}

type changedUnit struct {
	extensionsv1alpha1.Unit
	dropIns dropIns
}

type dropIns struct {
	changed []extensionsv1alpha1.DropIn
	deleted []extensionsv1alpha1.DropIn
}

type files struct {
	changed []extensionsv1alpha1.File
	deleted []extensionsv1alpha1.File
}

type containerd struct {
	// configFileChange tracks if the config file of containerd will change, so that GNA can restart the unit.
	configFileChange bool
	// registries tracks the changes of configured registries.
	registries containerdRegistries
}

type containerdRegistries struct {
	desired []extensionsv1alpha1.RegistryConfig
	deleted []extensionsv1alpha1.RegistryConfig
}

func computeOperatingSystemConfigChanges(fs afero.Afero, newOSC *extensionsv1alpha1.OperatingSystemConfig) (*operatingSystemConfigChanges, error) {
	changes := &operatingSystemConfigChanges{}

	// osc.files and osc.unit.files should be changed the same way by OSC controller.
	// The reason for assigning files to units is the detection of changes which require the restart of a unit.
	newOSCFiles := collectAllFiles(newOSC)

	oldOSCRaw, err := fs.ReadFile(lastAppliedOperatingSystemConfigFilePath)
	if err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("error reading last applied OSC from file path %s: %w", lastAppliedOperatingSystemConfigFilePath, err)
		}

		var unitChanges []changedUnit
		for _, unit := range mergeUnits(newOSC.Spec.Units, newOSC.Status.ExtensionUnits) {
			unitChanges = append(unitChanges, changedUnit{
				Unit:    unit,
				dropIns: dropIns{changed: unit.DropIns},
			})
		}

		changes.files.changed = newOSCFiles
		changes.units.changed = unitChanges

		// On new nodes, the deprecated containerd-initializer service can safely be removed.
		// TODO(timuthy): Remove this block after Gardener v1.114 was released.
		removeContainerdInit(changes)

		changes.containerd.configFileChange = true
		if extensionsv1alpha1helper.HasContainerdConfiguration(newOSC.Spec.CRIConfig) {
			changes.containerd.registries.desired = newOSC.Spec.CRIConfig.Containerd.Registries
		}
		return changes, nil
	}

	oldOSC := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(decoder, oldOSCRaw, oldOSC); err != nil {
		return nil, fmt.Errorf("unable to decode the old OSC read from file path %s: %w", lastAppliedOperatingSystemConfigFilePath, err)
	}

	oldOSCFiles := collectAllFiles(oldOSC)
	// File changes have to be computed in one step for all files,
	// because moving a file from osc.unit.files to osc.files or vice versa should not result in a change and a delete event.
	changes.files = computeFileDiffs(oldOSCFiles, newOSCFiles)

	changes.units = computeUnitDiffs(
		mergeUnits(oldOSC.Spec.Units, oldOSC.Status.ExtensionUnits),
		mergeUnits(newOSC.Spec.Units, newOSC.Status.ExtensionUnits),
		changes.files,
	)

	var (
		newRegistries []extensionsv1alpha1.RegistryConfig
		oldRegistries []extensionsv1alpha1.RegistryConfig
	)

	if extensionsv1alpha1helper.HasContainerdConfiguration(newOSC.Spec.CRIConfig) {
		newRegistries = newOSC.Spec.CRIConfig.Containerd.Registries
		if !extensionsv1alpha1helper.HasContainerdConfiguration(oldOSC.Spec.CRIConfig) {
			changes.containerd.configFileChange = true
		} else {
			var (
				newContainerd = newOSC.Spec.CRIConfig.Containerd
				oldContainerd = oldOSC.Spec.CRIConfig.Containerd
			)

			changes.containerd.configFileChange = !apiequality.Semantic.DeepEqual(newContainerd.SandboxImage, oldContainerd.SandboxImage) ||
				!apiequality.Semantic.DeepEqual(newContainerd.Plugins, oldContainerd.Plugins) ||
				!apiequality.Semantic.DeepEqual(newOSC.Spec.CRIConfig.CgroupDriver, oldOSC.Spec.CRIConfig.CgroupDriver)

			oldRegistries = oldOSC.Spec.CRIConfig.Containerd.Registries
		}
	}
	changes.containerd.registries = computeContainerdRegistryDiffs(newRegistries, oldRegistries)

	return changes, nil
}

// TODO(timuthy): Remove this block after Gardener v1.114 was released.
func removeContainerdInit(changes *operatingSystemConfigChanges) {
	for i, file := range changes.files.changed {
		if file.Path == componentscontainerd.InitializerScriptPath {
			changes.files.changed = slices.Delete(changes.files.changed, i, i+1)
		}
	}
	for i, unit := range changes.units.changed {
		if unit.Name == componentscontainerd.InitializerUnitName {
			changes.units.changed = slices.Delete(changes.units.changed, i, i+1)
		}
	}
}

func computeUnitDiffs(oldUnits, newUnits []extensionsv1alpha1.Unit, fileDiffs files) units {
	var u units

	var changedFiles = sets.New[string]()
	// Only changed files are relevant here. Deleted files must be removed from `Unit.FilePaths` too which leads to a semantic difference.
	for _, file := range fileDiffs.changed {
		changedFiles.Insert(file.Path)
	}

	for _, oldUnit := range oldUnits {
		if !slices.ContainsFunc(newUnits, func(newUnit extensionsv1alpha1.Unit) bool {
			return oldUnit.Name == newUnit.Name
		}) {
			u.deleted = append(u.deleted, oldUnit)
		}
	}

	for _, newUnit := range newUnits {
		oldUnitIndex := slices.IndexFunc(oldUnits, func(oldUnit extensionsv1alpha1.Unit) bool {
			return oldUnit.Name == newUnit.Name
		})

		var fileContentChanged bool
		for _, filePath := range newUnit.FilePaths {
			if changedFiles.Has(filePath) {
				fileContentChanged = true
			}
		}

		if oldUnitIndex == -1 {
			u.changed = append(u.changed, changedUnit{
				Unit:    newUnit,
				dropIns: dropIns{changed: newUnit.DropIns},
			})
		} else if !apiequality.Semantic.DeepEqual(oldUnits[oldUnitIndex], newUnit) || fileContentChanged {
			var d dropIns

			for _, oldDropIn := range oldUnits[oldUnitIndex].DropIns {
				if !slices.ContainsFunc(newUnit.DropIns, func(newDropIn extensionsv1alpha1.DropIn) bool {
					return oldDropIn.Name == newDropIn.Name
				}) {
					d.deleted = append(d.deleted, oldDropIn)
				}
			}

			for _, newDropIn := range newUnit.DropIns {
				oldDropInIndex := slices.IndexFunc(oldUnits[oldUnitIndex].DropIns, func(oldDropIn extensionsv1alpha1.DropIn) bool {
					return oldDropIn.Name == newDropIn.Name
				})

				if oldDropInIndex == -1 || !apiequality.Semantic.DeepEqual(oldUnits[oldUnitIndex].DropIns[oldDropInIndex], newDropIn) {
					d.changed = append(d.changed, newDropIn)
					continue
				}
			}

			u.changed = append(u.changed, changedUnit{
				Unit:    newUnit,
				dropIns: d,
			})
		}
	}

	return u
}

func computeFileDiffs(oldFiles, newFiles []extensionsv1alpha1.File) files {
	var f files

	for _, oldFile := range oldFiles {
		if !slices.ContainsFunc(newFiles, func(newFile extensionsv1alpha1.File) bool {
			return oldFile.Path == newFile.Path
		}) {
			f.deleted = append(f.deleted, oldFile)
		}
	}

	for _, newFile := range newFiles {
		oldFileIndex := slices.IndexFunc(oldFiles, func(oldFile extensionsv1alpha1.File) bool {
			return oldFile.Path == newFile.Path
		})

		if oldFileIndex == -1 || !apiequality.Semantic.DeepEqual(oldFiles[oldFileIndex], newFile) {
			f.changed = append(f.changed, newFile)
			continue
		}
	}

	return f
}

func mergeUnits(specUnits, statusUnits []extensionsv1alpha1.Unit) []extensionsv1alpha1.Unit {
	var out []extensionsv1alpha1.Unit

	for _, unit := range append(specUnits, statusUnits...) {
		unitIndex := slices.IndexFunc(out, func(existingUnit extensionsv1alpha1.Unit) bool {
			return existingUnit.Name == unit.Name
		})

		if unitIndex == -1 {
			out = append(out, unit)
			continue
		}

		if unit.Enable != nil {
			out[unitIndex].Enable = unit.Enable
		}
		if unit.Command != nil {
			out[unitIndex].Command = unit.Command
		}
		if unit.Content != nil {
			out[unitIndex].Content = unit.Content
		}
		out[unitIndex].DropIns = append(out[unitIndex].DropIns, unit.DropIns...)
		out[unitIndex].FilePaths = append(out[unitIndex].FilePaths, unit.FilePaths...)
	}

	return out
}

func collectAllFiles(osc *extensionsv1alpha1.OperatingSystemConfig) []extensionsv1alpha1.File {
	return append(osc.Spec.Files, osc.Status.ExtensionFiles...)
}

func computeContainerdRegistryDiffs(newRegistries, oldRegistries []extensionsv1alpha1.RegistryConfig) containerdRegistries {
	r := containerdRegistries{
		desired: newRegistries,
	}

	upstreamsInUse := sets.New[string]()
	for _, registryConfig := range r.desired {
		upstreamsInUse.Insert(registryConfig.Upstream)
	}

	r.deleted = slices.DeleteFunc(oldRegistries, func(config extensionsv1alpha1.RegistryConfig) bool {
		return upstreamsInUse.Has(config.Upstream)
	})

	return r
}
