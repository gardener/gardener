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

	OSCChecksum     string     `json:"oscChecksum"`
	Units           units      `json:"units"`
	Files           files      `json:"files"`
	Containerd      containerd `json:"containerd"`
	RestartRequired bool       `json:"restartRequired"`
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
	// ConfigFileChange tracks if the config file of containerd will change, so that GNA can restart the unit.
	ConfigFileChange bool `json:"configFileChange"`
	// Registries tracks the changes of configured Registries.
	Registries containerdRegistries `json:"registries"`
}

type containerdRegistries struct {
	Desired []extensionsv1alpha1.RegistryConfig `json:"desired,omitempty"`
	Deleted []extensionsv1alpha1.RegistryConfig `json:"deleted,omitempty"`
}

// persist the operatingSystemConfigChanges to disk. persist() requires the caller to ensure no concurrent actions are
// taking place by holding the lock.
func (o *operatingSystemConfigChanges) persist() error {
	out, err := yaml.Marshal(o)
	if err != nil {
		return err
	}
	return o.fs.WriteFile(operatingSystemConfigChangesFilePath, out, 0600)
}

func (o *operatingSystemConfigChanges) setRestartRequired(restart bool) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.RestartRequired = restart
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

	o.Containerd.ConfigFileChange = false
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
	raw, err := fs.ReadFile(operatingSystemConfigChangesFilePath)
	if err != nil {
		return nil, err
	}
	changes := operatingSystemConfigChanges{}
	if err := yaml.Unmarshal(raw, &changes); err != nil {
		return nil, err
	}
	changes.fs = fs
	return &changes, nil
}
