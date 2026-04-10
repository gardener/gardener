// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

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
	// ActionList is constant for the 'List' action.
	ActionList
	// ActionListByNames is constant for the 'ListByNames' action.
	ActionListByNames
	// ActionGetUnitStateChangeTimestamp is constant for the 'GetUnitStateChangeTimestamp' action.
	ActionGetUnitStateChangeTimestamp
	// ActionGetTriggeredBy is constant for the 'GetTriggeredBy' action.
	ActionGetTriggeredBy
	// ActionGetServiceType is constant for the 'GetServiceType' action.
	ActionGetServiceType
)

// SystemdAction is used for the implementation of the fake dbus.
type SystemdAction struct {
	Action    Action
	UnitNames []string
}

// DBus is a fake implementation for the dbus.DBus interface.
type DBus struct {
	Actions               []SystemdAction
	failures              map[string]error
	units                 []systemddbus.UnitStatus
	stateChangeTimestamps map[string]time.Time
	triggeredBy           map[string][]string
	serviceTypes          map[string]string

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
func (d *DBus) Restart(_ context.Context, _ events.EventRecorder, _ runtime.Object, unitName string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	action := SystemdAction{
		Action:    ActionRestart,
		UnitNames: []string{unitName},
	}
	d.Actions = append(d.Actions, action)

	return d.maybeError(action)
}

// List implements dbus.DBus.
func (d *DBus) List(_ context.Context) ([]systemddbus.UnitStatus, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action: ActionList,
	})
	return d.units, nil
}

// ListByNames implements dbus.DBus.
func (d *DBus) ListByNames(_ context.Context, unitNames []string) ([]systemddbus.UnitStatus, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionListByNames,
		UnitNames: unitNames,
	})

	var result []systemddbus.UnitStatus
	for _, unit := range d.units {
		if slices.Contains(unitNames, unit.Name) {
			result = append(result, unit)
		}
	}
	return result, nil
}

// GetUnitStateChangeTimestamp implements dbus.DBus.
func (d *DBus) GetUnitStateChangeTimestamp(_ context.Context, unitName string) (time.Time, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionGetUnitStateChangeTimestamp,
		UnitNames: []string{unitName},
	})

	if d.stateChangeTimestamps != nil {
		if timestamp, ok := d.stateChangeTimestamps[unitName]; ok {
			return timestamp, nil
		}
	}
	return time.Time{}, nil
}

// SetUnitStateChangeTimestamp sets the state change timestamp that will be returned for the given unit.
func (d *DBus) SetUnitStateChangeTimestamp(unitName string, timestamp time.Time) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.stateChangeTimestamps == nil {
		d.stateChangeTimestamps = make(map[string]time.Time)
	}
	d.stateChangeTimestamps[unitName] = timestamp
}

// GetTriggeredBy implements dbus.DBus.
func (d *DBus) GetTriggeredBy(_ context.Context, unitName string) ([]string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionGetTriggeredBy,
		UnitNames: []string{unitName},
	})

	if d.triggeredBy != nil {
		if triggers, ok := d.triggeredBy[unitName]; ok {
			return triggers, nil
		}
	}
	return nil, nil
}

// SetTriggeredBy sets the TriggeredBy list that will be returned for the given unit.
func (d *DBus) SetTriggeredBy(unitName string, triggers []string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.triggeredBy == nil {
		d.triggeredBy = make(map[string][]string)
	}
	d.triggeredBy[unitName] = triggers
}

// GetServiceType implements dbus.DBus.
func (d *DBus) GetServiceType(_ context.Context, unitName string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionGetServiceType,
		UnitNames: []string{unitName},
	})

	if d.serviceTypes != nil {
		if serviceType, ok := d.serviceTypes[unitName]; ok {
			return serviceType, nil
		}
	}
	return "simple", nil
}

// SetServiceType sets the service type that will be returned for the given unit.
func (d *DBus) SetServiceType(unitName, serviceType string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.serviceTypes == nil {
		d.serviceTypes = make(map[string]string)
	}
	d.serviceTypes[unitName] = serviceType
}

// AddUnitsToList adds the given units to the list of units that will be returned by List.
func (d *DBus) AddUnitsToList(units ...systemddbus.UnitStatus) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.units = append(d.units, units...)
}

// SetUnits replaces the list of units that will be returned by List and ListByNames.
func (d *DBus) SetUnits(units ...systemddbus.UnitStatus) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.units = units
}

// Reboot implements dbus.DBus.
func (d *DBus) Reboot() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action: ActionReboot,
	})
	return nil
}

// Start implements dbus.DBus.
func (d *DBus) Start(_ context.Context, _ events.EventRecorder, _ runtime.Object, unitName string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionStart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Stop implements dbus.DBus.
func (d *DBus) Stop(_ context.Context, _ events.EventRecorder, _ runtime.Object, unitName string) error {
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
