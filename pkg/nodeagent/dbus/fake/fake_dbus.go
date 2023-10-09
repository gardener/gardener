// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fake

import (
	"context"

	corev1 "k8s.io/api/core/v1"
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
)

// SystemdAction is used for the implementation of the fake dbus.
type SystemdAction struct {
	Action    Action
	UnitNames []string
}

// DBus is a fake implementation for the dbus.DBus interface.
type DBus struct {
	Actions []SystemdAction
}

var _ dbus.DBus = &DBus{}

// New returns a simple implementation of dbus.DBus which can be used to fake the dbus actions in unit tests.
func New() *DBus {
	return &DBus{}
}

// DaemonReload implements dbus.DBus.
func (d *DBus) DaemonReload(_ context.Context) error {
	d.Actions = append(d.Actions, SystemdAction{
		Action: ActionDaemonReload,
	})
	return nil
}

// Disable implements dbus.DBus.
func (d *DBus) Disable(_ context.Context, unitNames ...string) error {
	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionDisable,
		UnitNames: unitNames,
	})
	return nil
}

// Enable implements dbus.DBus.
func (d *DBus) Enable(_ context.Context, unitNames ...string) error {
	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionEnable,
		UnitNames: unitNames,
	})

	return nil
}

// Restart implements dbus.DBus.
func (d *DBus) Restart(_ context.Context, _ record.EventRecorder, _ *corev1.Node, unitName string) error {
	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionRestart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Start implements dbus.DBus.
func (d *DBus) Start(_ context.Context, _ record.EventRecorder, _ *corev1.Node, unitName string) error {
	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionStart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Stop implements dbus.DBus.
func (d *DBus) Stop(_ context.Context, _ record.EventRecorder, _ *corev1.Node, unitName string) error {
	d.Actions = append(d.Actions, SystemdAction{
		Action:    ActionStop,
		UnitNames: []string{unitName},
	})
	return nil
}
