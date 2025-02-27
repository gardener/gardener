// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"errors"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	componentscontainerd "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

func extractOSCFromSecret(secret *corev1.Secret) (*extensionsv1alpha1.OperatingSystemConfig, string, error) {
	oscRaw, ok := secret.Data[nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig]
	if !ok {
		return nil, "", fmt.Errorf("no %s key found in OSC secret", nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig)
	}

	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(decoder, oscRaw, osc); err != nil {
		return nil, "", fmt.Errorf("unable to decode OSC from secret data key %s: %w", nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig, err)
	}

	return osc, secret.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumDownloadedOperatingSystemConfig], nil
}

func computeOperatingSystemConfigChanges(log logr.Logger, fs afero.Afero, newOSC *extensionsv1alpha1.OperatingSystemConfig, newOSCChecksum string) (*operatingSystemConfigChanges, error) {
	changes := &operatingSystemConfigChanges{
		fs:                            fs,
		OperatingSystemConfigChecksum: newOSCChecksum,
	}

	oldChanges, err := loadOSCChanges(fs)
	if err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("failed to load old osc changes file: %w", err)
		}
		// there is no file (yet), set to an empty file, the hashes will mismatch
		oldChanges = &operatingSystemConfigChanges{}
	}

	if oldChanges.OperatingSystemConfigChecksum == changes.OperatingSystemConfigChecksum {
		log.Info("Found previously computed OperatingSystemConfig changes on disk, remaining work",
			"changedFiles", len(oldChanges.Files.Changed),
			"deletedFiles", len(oldChanges.Files.Deleted),
			"changedUnits", len(oldChanges.Units.Changed),
			"deletedUnits", len(oldChanges.Units.Deleted),
			"unitCommands", len(oldChanges.Units.Commands),
		)
		// we already computed the changes, and might have partially applied them
		return oldChanges, nil
	}

	log.Info("OperatingSystemConfig changes checksum does not match, computing new changes")

	// create copy so that we don't accidentally update the `newOSC` when items are removed from the
	// `operatingSystemConfigChanges` when they are marked as completed.
	newOSC = newOSC.DeepCopy()

	// osc.files and osc.unit.files should be changed the same way by OSC controller.
	// The reason for assigning files to units is the detection of changes which require the restart of a unit.
	newOSCFiles := collectAllFiles(newOSC)

	oldOSCRaw, err := fs.ReadFile(lastAppliedOperatingSystemConfigFilePath)
	if err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return nil, fmt.Errorf("error reading last applied OSC from file path %s: %w", lastAppliedOperatingSystemConfigFilePath, err)
		}

		var (
			unitChanges  []changedUnit
			unitCommands []unitCommand
		)

		for _, unit := range mergeUnits(newOSC.Spec.Units, newOSC.Status.ExtensionUnits) {
			unitCommands = append(unitCommands, unitCommand{
				Name:    unit.Name,
				Command: getCommandToExecute(unit),
			})
			unitChanges = append(unitChanges, changedUnit{
				Unit:           unit,
				DropInsChanges: dropIns{Changed: unit.DropIns},
			})
		}

		changes.Files.Changed = newOSCFiles
		changes.Units.Changed = unitChanges
		changes.Units.Commands = unitCommands

		// On new nodes, the deprecated containerd-initializer service can safely be removed.
		// TODO(timuthy): Remove this block after Gardener v1.114 was released.
		removeContainerdInit(changes)

		changes.Containerd.ConfigFileChanged = true
		if extensionsv1alpha1helper.HasContainerdConfiguration(newOSC.Spec.CRIConfig) {
			changes.Containerd.Registries.Desired = newOSC.Spec.CRIConfig.Containerd.Registries
		}
		changes.lock.Lock()
		defer changes.lock.Unlock()
		return changes, changes.persist()
	}

	oldOSC := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(decoder, oldOSCRaw, oldOSC); err != nil {
		return nil, fmt.Errorf("unable to decode the old OSC read from file path %s: %w", lastAppliedOperatingSystemConfigFilePath, err)
	}

	oldOSCFiles := collectAllFiles(oldOSC)
	// File changes have to be computed in one step for all files,
	// because moving a file from osc.unit.files to osc.files or vice versa should not result in a change and a delete event.
	changes.Files = computeFileDiffs(oldOSCFiles, newOSCFiles)

	changes.Units = computeUnitDiffs(
		mergeUnits(oldOSC.Spec.Units, oldOSC.Status.ExtensionUnits),
		mergeUnits(newOSC.Spec.Units, newOSC.Status.ExtensionUnits),
		changes.Files,
	)

	var (
		newRegistries []extensionsv1alpha1.RegistryConfig
		oldRegistries []extensionsv1alpha1.RegistryConfig
	)

	if extensionsv1alpha1helper.HasContainerdConfiguration(newOSC.Spec.CRIConfig) {
		newRegistries = newOSC.Spec.CRIConfig.Containerd.Registries
		if !extensionsv1alpha1helper.HasContainerdConfiguration(oldOSC.Spec.CRIConfig) {
			changes.Containerd.ConfigFileChanged = true
		} else {
			var (
				newContainerd = newOSC.Spec.CRIConfig.Containerd
				oldContainerd = oldOSC.Spec.CRIConfig.Containerd
			)

			changes.Containerd.ConfigFileChanged = !apiequality.Semantic.DeepEqual(newContainerd.SandboxImage, oldContainerd.SandboxImage) ||
				!apiequality.Semantic.DeepEqual(newContainerd.Plugins, oldContainerd.Plugins) ||
				!apiequality.Semantic.DeepEqual(newOSC.Spec.CRIConfig.CgroupDriver, oldOSC.Spec.CRIConfig.CgroupDriver)

			oldRegistries = oldOSC.Spec.CRIConfig.Containerd.Registries
		}
	}
	changes.Containerd.Registries = computeContainerdRegistryDiffs(newRegistries, oldRegistries)

	changes.lock.Lock()
	defer changes.lock.Unlock()
	return changes, changes.persist()
}

// TODO(timuthy): Remove this block after Gardener v1.114 was released.
func removeContainerdInit(changes *operatingSystemConfigChanges) {
	for i, file := range changes.Files.Changed {
		if file.Path == componentscontainerd.InitializerScriptPath {
			changes.Files.Changed = slices.Delete(changes.Files.Changed, i, i+1)
		}
	}
	for i, unit := range changes.Units.Changed {
		if unit.Name == componentscontainerd.InitializerUnitName {
			changes.Units.Changed = slices.Delete(changes.Units.Changed, i, i+1)
		}
	}
	for i, unit := range changes.Units.Commands {
		if unit.Name == componentscontainerd.InitializerUnitName {
			changes.Units.Commands = slices.Delete(changes.Units.Commands, i, i+1)
		}
	}
}

func computeUnitDiffs(oldUnits, newUnits []extensionsv1alpha1.Unit, fileDiffs files) units {
	var u units

	var changedFiles = sets.New[string]()
	// Only changed files are relevant here. Deleted files must be removed from `Unit.FilePaths` too which leads to a semantic difference.
	for _, file := range fileDiffs.Changed {
		changedFiles.Insert(file.Path)
	}

	for _, oldUnit := range oldUnits {
		if !slices.ContainsFunc(newUnits, func(newUnit extensionsv1alpha1.Unit) bool {
			return oldUnit.Name == newUnit.Name
		}) {
			u.Deleted = append(u.Deleted, oldUnit)
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
		commandToExecute := getCommandToExecute(newUnit)

		if oldUnitIndex == -1 {
			u.Changed = append(u.Changed, changedUnit{
				Unit:           newUnit,
				DropInsChanges: dropIns{Changed: newUnit.DropIns},
			})
			u.Commands = append(u.Commands, unitCommand{
				Name:    newUnit.Name,
				Command: commandToExecute,
			})
		} else if !apiequality.Semantic.DeepEqual(oldUnits[oldUnitIndex], newUnit) || fileContentChanged {
			var d dropIns

			for _, oldDropIn := range oldUnits[oldUnitIndex].DropIns {
				if !slices.ContainsFunc(newUnit.DropIns, func(newDropIn extensionsv1alpha1.DropIn) bool {
					return oldDropIn.Name == newDropIn.Name
				}) {
					d.Deleted = append(d.Deleted, oldDropIn)
				}
			}

			for _, newDropIn := range newUnit.DropIns {
				oldDropInIndex := slices.IndexFunc(oldUnits[oldUnitIndex].DropIns, func(oldDropIn extensionsv1alpha1.DropIn) bool {
					return oldDropIn.Name == newDropIn.Name
				})

				if oldDropInIndex == -1 || !apiequality.Semantic.DeepEqual(oldUnits[oldUnitIndex].DropIns[oldDropInIndex], newDropIn) {
					d.Changed = append(d.Changed, newDropIn)
					continue
				}
			}

			u.Changed = append(u.Changed, changedUnit{
				Unit:           newUnit,
				DropInsChanges: d,
			})
			u.Commands = append(u.Commands, unitCommand{
				Name:    newUnit.Name,
				Command: commandToExecute,
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
			f.Deleted = append(f.Deleted, oldFile)
		}
	}

	for _, newFile := range newFiles {
		oldFileIndex := slices.IndexFunc(oldFiles, func(oldFile extensionsv1alpha1.File) bool {
			return oldFile.Path == newFile.Path
		})

		if oldFileIndex == -1 || !apiequality.Semantic.DeepEqual(oldFiles[oldFileIndex], newFile) {
			f.Changed = append(f.Changed, newFile)
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
	var r containerdRegistries

	for _, oldRegistry := range oldRegistries {
		if !slices.ContainsFunc(newRegistries, func(newRegistry extensionsv1alpha1.RegistryConfig) bool {
			return oldRegistry.Upstream == newRegistry.Upstream
		}) {
			r.Deleted = append(r.Deleted, oldRegistry)
		}
	}

	for _, newRegistry := range newRegistries {
		if ptr.Deref(newRegistry.ReadinessProbe, false) {
			oldRegistryIndex := slices.IndexFunc(oldRegistries, func(oldRegistry extensionsv1alpha1.RegistryConfig) bool {
				return oldRegistry.Upstream == newRegistry.Upstream
			})
			if oldRegistryIndex != -1 && apiequality.Semantic.DeepEqual(oldRegistries[oldRegistryIndex], newRegistry) {
				// suppress host probing if no changes are detected
				newRegistry.ReadinessProbe = ptr.To(false)
			}
		}

		r.Desired = append(r.Desired, newRegistry)
	}
	return r
}

func getCommandToExecute(newUnit extensionsv1alpha1.Unit) extensionsv1alpha1.UnitCommand {
	commandToExecute := extensionsv1alpha1.CommandRestart
	if !ptr.Deref(newUnit.Enable, true) || ptr.Deref(newUnit.Command, "") == extensionsv1alpha1.CommandStop {
		commandToExecute = extensionsv1alpha1.CommandStop
	}
	return commandToExecute
}
