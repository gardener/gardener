// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// Action is an int type alias.
type Action int

const (
	// ActionDaemonReload is constant for the 'DaemonReload' action.
	ActionDaemonReload Action = iota
	// ActionDisable is constant for the 'Disable' action.
	ActionDisable
	// ActionEnable is constant for the 'Enable' action.
	ActionEnable
	// ActionRestart is constant for the 'Restart' action.
	ActionRestart
	// ActionStart is constant for the 'Start' action.
	ActionStart
	// ActionStop is constant for the 'Stop' action.
	ActionStop
	// ActionReboot is constant for the 'Reboot' action.
	ActionReboot
)

// SystemdAction is used for the implementation of the fake dbus.
type SystemdAction struct {
	Action    Action
	UnitNames []string
}

// DBus is a fake implementation for the dbus.DBus interface.
type DBus struct {
	Actions  []SystemdAction
	failures map[string]error

	mutex sync.Mutex
}

var _ dbus.DBus = &DBus{}

// New returns a simple implementation of dbus.DBus which can be used to fake the dbus actions in unit tests.
func New() *DBus {
	return &DBus{
		failures: map[string]error{},
	}
}

// InjectRestartFailure returns the given error the first time a restart is triggered on the given units.
func (d *DBus) InjectRestartFailure(err error, unitNames ...string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	key := failureKey(SystemdAction{Action: ActionRestart, UnitNames: unitNames})
	d.failures[key] = err
}

func (d *DBus) maybeError(action SystemdAction) error {
	key := failureKey(action)
	err, ok := d.failures[key]
	if !ok {
		return nil
	}
	delete(d.failures, key)
	return err
}

// DaemonReload implements dbus.DBus.
func (d *DBus) DaemonReload(_ context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action: ActionDaemonReload,
	})

	return nil
}

// Disable implements dbus.DBus.
func (d *DBus) Disable(_ context.Context, unitNames ...string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionDisable,
		UnitNames: unitNames,
	})

	return nil
}

// Enable implements dbus.DBus.
func (d *DBus) Enable(_ context.Context, unitNames ...string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionEnable,
		UnitNames: unitNames,
	})

	return nil
}

// Restart implements dbus.DBus.
func (d *DBus) Restart(_ context.Context, _ record.EventRecorder, _ runtime.Object, unitName string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	action := SystemdAction{
		Action:    ActionRestart,
		UnitNames: []string{unitName},
	}
	d.Actions = append(d.Actions, action)

	return d.maybeError(action)
}

// Reboot implements dbus.DBus.
func (d *DBus) Reboot() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionReboot,
		UnitNames: []string{"reboot"},
	})
	return nil
}

// Start implements dbus.DBus.
func (d *DBus) Start(_ context.Context, _ record.EventRecorder, _ runtime.Object, unitName string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionStart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Stop implements dbus.DBus.
func (d *DBus) Stop(_ context.Context, _ record.EventRecorder, _ runtime.Object, unitName string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionStop,
		UnitNames: []string{unitName},
	})
	return nil
}

func failureKey(action SystemdAction) string {
	return strings.Join(action.UnitNames, "-") + strconv.Itoa(int(action.Action))
}
