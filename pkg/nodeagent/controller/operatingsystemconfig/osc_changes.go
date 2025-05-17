// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"fmt"
	"slices"
	"sync"

	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type operatingSystemConfigChanges struct {
	lock sync.Mutex
	fs   afero.Afero

	OperatingSystemConfigChecksum string     `json:"operatingSystemConfigChecksum"`
	Units                         units      `json:"units"`
	Files                         files      `json:"files"`
	Containerd                    containerd `json:"containerd"`
	MustRestartNodeAgent          bool       `json:"mustRestartNodeAgent"`

	InPlaceUpdates inPlaceUpdates `json:"inPlaceUpdates"`
}

type units struct {
	Changed  []changedUnit             `json:"changed,omitempty"`
	Commands []unitCommand             `json:"commands,omitempty"`
	Deleted  []extensionsv1alpha1.Unit `json:"deleted,omitempty"`
}

type changedUnit struct {
	extensionsv1alpha1.Unit `json:",inline"`

	DropInsChanges dropIns `json:"dropInsChanges"`
}

type unitCommand struct {
	Name    string                         `json:"name"`
	Command extensionsv1alpha1.UnitCommand `json:"command"`
}

type dropIns struct {
	Changed []extensionsv1alpha1.DropIn `json:"changed,omitempty"`
	Deleted []extensionsv1alpha1.DropIn `json:"deleted,omitempty"`
}

type files struct {
	Changed []extensionsv1alpha1.File `json:"changed,omitempty"`
	Deleted []extensionsv1alpha1.File `json:"deleted,omitempty"`
}

type containerd struct {
	// ConfigFileChanged tracks if the config file of containerd will change, so that GNA can restart the unit.
	ConfigFileChanged bool `json:"configFileChanged"`
	// Registries tracks the changes of configured Registries.
	Registries containerdRegistries `json:"registries"`
}

type containerdRegistries struct {
	UpstreamsToProbe []string                            `json:"upstreamsToProbe,omitempty"`
	Desired          []extensionsv1alpha1.RegistryConfig `json:"desired,omitempty"`
	Deleted          []extensionsv1alpha1.RegistryConfig `json:"deleted,omitempty"`
}

type inPlaceUpdates struct {
	OperatingSystem                bool                           `json:"operatingSystem"`
	Kubelet                        kubelet                        `json:"kubelet"`
	CertificateAuthoritiesRotation certificateAuthoritiesRotation `json:"certificateAuthoritiesRotation"`
	ServiceAccountKeyRotation      bool                           `json:"serviceAccountKeyRotation"`
}

type kubelet struct {
	MinorVersion     bool `json:"minorVersion"`
	Config           bool `json:"config"`
	CPUManagerPolicy bool `json:"cpuManagerPolicy"`
}

type certificateAuthoritiesRotation struct {
	Kubelet   bool `json:"kubelet"`
	NodeAgent bool `json:"nodeAgent"`
}

// persist the operatingSystemConfigChanges to disk. persist() requires the caller to ensure no concurrent actions are
// taking place by holding the lock.
func (o *operatingSystemConfigChanges) persist() error {
	out, err := yaml.Marshal(o)
	if err != nil {
		return fmt.Errorf("failed marshalling the changes into YAML: %w", err)
	}
	return o.fs.WriteFile(lastComputedOperatingSystemConfigChangesFilePath, out, 0600)
}

func (o *operatingSystemConfigChanges) setMustRestartNodeAgent(restart bool) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.MustRestartNodeAgent = restart
	return o.persist()
}

func (o *operatingSystemConfigChanges) completeOSUpdate() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.InPlaceUpdates.OperatingSystem = false

	return o.persist()
}

func (o *operatingSystemConfigChanges) completeKubeletMinorVersionUpdate() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.InPlaceUpdates.Kubelet.MinorVersion = false

	return o.persist()
}

func (o *operatingSystemConfigChanges) completeKubeletConfigUpdate() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.InPlaceUpdates.Kubelet.Config = false

	return o.persist()
}

func (o *operatingSystemConfigChanges) completeKubeletCPUManagerPolicyUpdate() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.InPlaceUpdates.Kubelet.CPUManagerPolicy = false

	return o.persist()
}

func (o *operatingSystemConfigChanges) completeCARotationKubelet() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.InPlaceUpdates.CertificateAuthoritiesRotation.Kubelet = false

	return o.persist()
}

func (o *operatingSystemConfigChanges) completeCARotationNodeAgent() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.MustRestartNodeAgent = true
	o.InPlaceUpdates.CertificateAuthoritiesRotation.NodeAgent = false

	return o.persist()
}

func (o *operatingSystemConfigChanges) completeSAKeyRotation() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.InPlaceUpdates.ServiceAccountKeyRotation = false

	return o.persist()
}

func (o *operatingSystemConfigChanges) completedUnitCommand(name string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Units.Commands = slices.DeleteFunc(o.Units.Commands, func(c unitCommand) bool {
		return c.Name == name
	})

	return o.persist()
}

func (o *operatingSystemConfigChanges) completedUnitChanged(name string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Units.Changed = slices.DeleteFunc(o.Units.Changed, func(c changedUnit) bool {
		return c.Name == name
	})
	return o.persist()
}

func (o *operatingSystemConfigChanges) completedUnitDeleted(name string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Units.Deleted = slices.DeleteFunc(o.Units.Deleted, func(c extensionsv1alpha1.Unit) bool {
		return c.Name == name
	})

	return o.persist()
}

func (o *operatingSystemConfigChanges) completedUnitDropInChanged(unitName, dropInName string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	i := slices.IndexFunc(o.Units.Changed, func(u changedUnit) bool {
		return u.Name == unitName
	})
	if i < 0 {
		return fmt.Errorf("expected to find unit with name %q", unitName)
	}
	o.Units.Changed[i].DropIns = slices.DeleteFunc(o.Units.Changed[i].DropIns, func(d extensionsv1alpha1.DropIn) bool {
		return d.Name == dropInName
	})

	return o.persist()
}

func (o *operatingSystemConfigChanges) completedUnitDropInDeleted(unitName, dropInName string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	i := slices.IndexFunc(o.Units.Changed, func(u changedUnit) bool {
		return u.Name == unitName
	})
	if i < 0 {
		return fmt.Errorf("expected to find unit with name %q", unitName)
	}
	o.Units.Changed[i].DropIns = slices.DeleteFunc(o.Units.Changed[i].DropIns, func(d extensionsv1alpha1.DropIn) bool {
		return d.Name == dropInName
	})

	return o.persist()
}

func (o *operatingSystemConfigChanges) completedFileDeleted(path string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Files.Deleted = slices.DeleteFunc(o.Files.Deleted, func(f extensionsv1alpha1.File) bool {
		return f.Path == path
	})

	return o.persist()
}

func (o *operatingSystemConfigChanges) completedFileChanged(path string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Files.Changed = slices.DeleteFunc(o.Files.Changed, func(f extensionsv1alpha1.File) bool {
		return f.Path == path
	})

	return o.persist()
}
func (o *operatingSystemConfigChanges) completedContainerdConfigFileChange() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Containerd.ConfigFileChanged = false
	return o.persist()
}

func (o *operatingSystemConfigChanges) completedContainerdRegistriesDesired(upstream string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Containerd.Registries.Desired = slices.DeleteFunc(o.Containerd.Registries.Desired, func(c extensionsv1alpha1.RegistryConfig) bool {
		return c.Upstream == upstream
	})

	return o.persist()
}

func (o *operatingSystemConfigChanges) completedContainerdRegistriesDeleted(upstream string) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Containerd.Registries.Deleted = slices.DeleteFunc(o.Containerd.Registries.Deleted, func(c extensionsv1alpha1.RegistryConfig) bool {
		return c.Upstream == upstream
	})

	return o.persist()
}

func loadOSCChanges(fs afero.Afero) (*operatingSystemConfigChanges, error) {
	raw, err := fs.ReadFile(lastComputedOperatingSystemConfigChangesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed reading changes file %s: %w", lastComputedOperatingSystemConfigChangesFilePath, err)
	}

	changes := operatingSystemConfigChanges{}
	if err := yaml.Unmarshal(raw, &changes); err != nil {
		return nil, fmt.Errorf("failed unmarshalling the changes: %w", err)
	}

	changes.fs = fs
	return &changes, nil
}
