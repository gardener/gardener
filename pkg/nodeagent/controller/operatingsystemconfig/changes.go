// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	kubeletcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
	"github.com/gardener/gardener/pkg/features"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeletconfigv1beta1.AddToScheme(scheme))
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

func computeOperatingSystemConfigChanges(log logr.Logger, fs afero.Afero, newOSC *extensionsv1alpha1.OperatingSystemConfig, newOSCChecksum string, currentOSVersion *string) (*operatingSystemConfigChanges, error) {
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

		changes.Containerd.ConfigFileChanged = true
		if extensionsv1alpha1helper.HasContainerdConfiguration(newOSC.Spec.CRIConfig) {
			newRegistries := newOSC.Spec.CRIConfig.Containerd.Registries
			upstreamsToProbe := make([]string, 0, len(newRegistries))
			for _, registryConfig := range newRegistries {
				if ptr.Deref(registryConfig.ReadinessProbe, false) {
					upstreamsToProbe = append(upstreamsToProbe, registryConfig.Upstream)
				}
			}

			changes.Containerd.Registries = containerdRegistries{
				Desired:          newRegistries,
				UpstreamsToProbe: upstreamsToProbe,
			}
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

	if oldOSC.Spec.InPlaceUpdates != nil && newOSC.Spec.InPlaceUpdates != nil {
		if currentOSVersion != nil && *currentOSVersion != newOSC.Spec.InPlaceUpdates.OperatingSystemVersion {
			changes.InPlaceUpdates.OperatingSystem = true
		}

		if oldOSC.Spec.InPlaceUpdates.KubeletVersion != newOSC.Spec.InPlaceUpdates.KubeletVersion {
			changes.InPlaceUpdates.Kubelet.MinorVersion, err = CheckIfMinorVersionUpdate(oldOSC.Spec.InPlaceUpdates.KubeletVersion, newOSC.Spec.InPlaceUpdates.KubeletVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to check if kubelet minor version was updated: %w", err)
			}
		}

		oldKubeletConfig, err := getKubeletConfig(oldOSC)
		if err != nil {
			return nil, fmt.Errorf("failed to get old kubelet config from the OSC: %w", err)
		}
		newKubeletConfig, err := getKubeletConfig(newOSC)
		if err != nil {
			return nil, fmt.Errorf("failed to get new kubelet config from the OSC: %w", err)
		}

		changes.InPlaceUpdates.Kubelet.Config, changes.InPlaceUpdates.Kubelet.CPUManagerPolicy, err = ComputeKubeletConfigChange(oldKubeletConfig, newKubeletConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to check if kubelet config has changed: %w", err)
		}

		if newOSC.Spec.InPlaceUpdates.CredentialsRotation != nil {
			// Rotation is triggered for the first time
			if oldOSC.Spec.InPlaceUpdates.CredentialsRotation == nil {
				caRotation := newOSC.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities != nil && newOSC.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities.LastInitiationTime != nil
				changes.InPlaceUpdates.CertificateAuthoritiesRotation.Kubelet = caRotation
				changes.InPlaceUpdates.CertificateAuthoritiesRotation.NodeAgent = caRotation && features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer)

				changes.InPlaceUpdates.ServiceAccountKeyRotation = newOSC.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey != nil && newOSC.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey.LastInitiationTime != nil
			} else {
				caRotation := oldOSC.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities != nil &&
					newOSC.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities != nil &&
					!oldOSC.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities.LastInitiationTime.Equal(newOSC.Spec.InPlaceUpdates.CredentialsRotation.CertificateAuthorities.LastInitiationTime)
				changes.InPlaceUpdates.CertificateAuthoritiesRotation.Kubelet = caRotation
				changes.InPlaceUpdates.CertificateAuthoritiesRotation.NodeAgent = caRotation && features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer)

				changes.InPlaceUpdates.ServiceAccountKeyRotation = oldOSC.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey != nil &&
					newOSC.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey != nil &&
					!oldOSC.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey.LastInitiationTime.Equal(newOSC.Spec.InPlaceUpdates.CredentialsRotation.ServiceAccountKey.LastInitiationTime)
			}
		}
	}

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

func getKubeletConfig(osc *extensionsv1alpha1.OperatingSystemConfig) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	var (
		fciCodec           = oscutils.NewFileContentInlineCodec()
		kubeletConfigCodec = kubeletcomponent.NewConfigCodec(fciCodec)
	)

	for _, file := range osc.Spec.Files {
		if file.Path == kubeletcomponent.PathKubeletConfig {
			return kubeletConfigCodec.Decode(file.Content.Inline)
		}
	}

	return nil, fmt.Errorf("kubelet config file with path: %q not found in OSC", kubeletcomponent.PathKubeletConfig)
}

// ComputeKubeletConfigChange computes changes in the kubelet configuration relevant for in-place updates.
// This function needs to be updated when the kubelet configuration triggers in https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_updates.md#rolling-update-triggers are changed.
func ComputeKubeletConfigChange(oldConfig, newConfig *kubeletconfigv1beta1.KubeletConfiguration) (bool, bool, error) {
	var (
		cpuManagerPolicyChanged = oldConfig.CPUManagerPolicy != newConfig.CPUManagerPolicy
		oldRelevantEvictionHard = make(map[string]string)
		newRelevantEvictionHard = make(map[string]string)
	)

	// Copy only relevant config values for comparison.
	if oldConfig.EvictionHard != nil {
		oldRelevantEvictionHard[components.MemoryAvailable] = oldConfig.EvictionHard[components.MemoryAvailable]
		oldRelevantEvictionHard[components.ImageFSAvailable] = oldConfig.EvictionHard[components.ImageFSAvailable]
		oldRelevantEvictionHard[components.ImageFSInodesFree] = oldConfig.EvictionHard[components.ImageFSInodesFree]
		oldRelevantEvictionHard[components.NodeFSAvailable] = oldConfig.EvictionHard[components.NodeFSAvailable]
		oldRelevantEvictionHard[components.NodeFSInodesFree] = oldConfig.EvictionHard[components.NodeFSInodesFree]
	}

	if newConfig.EvictionHard != nil {
		newRelevantEvictionHard[components.MemoryAvailable] = newConfig.EvictionHard[components.MemoryAvailable]
		newRelevantEvictionHard[components.ImageFSAvailable] = newConfig.EvictionHard[components.ImageFSAvailable]
		newRelevantEvictionHard[components.ImageFSInodesFree] = newConfig.EvictionHard[components.ImageFSInodesFree]
		newRelevantEvictionHard[components.NodeFSAvailable] = newConfig.EvictionHard[components.NodeFSAvailable]
		newRelevantEvictionHard[components.NodeFSInodesFree] = newConfig.EvictionHard[components.NodeFSInodesFree]
	}

	if !maps.Equal(oldRelevantEvictionHard, newRelevantEvictionHard) {
		return true, cpuManagerPolicyChanged, nil
	}

	oldReserved, err := sumResourceReservations(oldConfig.KubeReserved, oldConfig.SystemReserved)
	if err != nil {
		return false, cpuManagerPolicyChanged, fmt.Errorf("failed to sum resource reservations for old kubelet config: %w", err)
	}
	newReserved, err := sumResourceReservations(newConfig.KubeReserved, newConfig.SystemReserved)
	if err != nil {
		return false, cpuManagerPolicyChanged, fmt.Errorf("failed to sum resource reservations for new kubelet config: %w", err)
	}

	return !maps.Equal(oldReserved, newReserved), cpuManagerPolicyChanged, nil
}

func sumResourceReservations(left, right map[string]string) (map[string]string, error) {
	if left == nil {
		return right, nil
	} else if right == nil {
		return left, nil
	}

	out := make(map[string]string)

	for _, resource := range []string{"cpu", "memory", "ephemeral-storage", "pid"} {
		if quantity, err := sumQuantities(left[resource], right[resource]); err != nil {
			return nil, fmt.Errorf("failed to sum %s reservations: %w", resource, err)
		} else {
			out[resource] = quantity.String()
		}
	}

	return out, nil
}

func sumQuantities(l, r string) (*resource.Quantity, error) {
	var (
		left, right resource.Quantity
		err         error
	)

	if r == "" && l == "" {
		return resource.NewQuantity(0, resource.DecimalSI), nil
	}

	if l != "" {
		left, err = resource.ParseQuantity(l)
		if err != nil {
			return nil, fmt.Errorf("failed to parse left quantity: %w", err)
		}

		if r == "" {
			return &left, nil
		}
	}

	if r != "" {
		right, err = resource.ParseQuantity(r)
		if err != nil {
			return nil, fmt.Errorf("failed to parse right quantity: %w", err)
		}

		if l == "" {
			return &right, nil
		}
	}

	copy := left.DeepCopy()
	copy.Add(right)
	return &copy, nil
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
	r := containerdRegistries{
		Desired: newRegistries,
	}

	// Compute deleted upstreams
	upstreamsInUse := sets.New[string]()
	for _, registryConfig := range r.Desired {
		upstreamsInUse.Insert(registryConfig.Upstream)
	}

	r.Deleted = slices.DeleteFunc(oldRegistries, func(config extensionsv1alpha1.RegistryConfig) bool {
		return upstreamsInUse.Has(config.Upstream)
	})

	// Compute upstreams to probe
	for _, newRegistry := range newRegistries {
		// Exit early if the registry endpoints should not be probed.
		if !ptr.Deref(newRegistry.ReadinessProbe, false) {
			continue
		}

		oldRegistryIndex := slices.IndexFunc(oldRegistries, func(oldRegistry extensionsv1alpha1.RegistryConfig) bool {
			return oldRegistry.Upstream == newRegistry.Upstream
		})

		if oldRegistryIndex == -1 {
			// A new registry config with enabled readiness probe is added. It has to be probed.
			r.UpstreamsToProbe = append(r.UpstreamsToProbe, newRegistry.Upstream)
		} else {
			oldRegistry := oldRegistries[oldRegistryIndex]
			if !apiequality.Semantic.DeepEqual(oldRegistry, newRegistry) {
				// There is a change in the registry config. It has to be probed again.
				r.UpstreamsToProbe = append(r.UpstreamsToProbe, newRegistry.Upstream)
			}
		}
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

// CheckIfMinorVersionUpdate checks if the new kubelet version is a minor version update to the old kubelet version.
func CheckIfMinorVersionUpdate(old, new string) (bool, error) {
	oldVersion, err := semver.NewVersion(versionutils.Normalize(old))
	if err != nil {
		return false, fmt.Errorf("failed to parse old kubelet version %s: %w", old, err)
	}
	newVersion, err := semver.NewVersion(versionutils.Normalize(new))
	if err != nil {
		return false, fmt.Errorf("failed to parse new kubelet version %s: %w", new, err)
	}

	return oldVersion.Minor() != newVersion.Minor(), nil
}
