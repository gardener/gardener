// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbus

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

// FakeAction is an enum for the fake dbus actions.
type FakeAction int

const (
	// FakeDeamonReload is the fake action for daemon reload.
	FakeDeamonReload FakeAction = iota
	// FakeDisable is the fake action for disable.
	FakeDisable
	// FakeEnable is the fake action for enable.
	FakeEnable
	// FakeRestart is the fake action for restart.
	FakeRestart
	// FakeStart is the fake action for start.
	FakeStart
	// FakeStop is the fake action for stop.
	FakeStop
)

// FakeSystemdAction represents a systemd action
type FakeSystemdAction struct {
	Action    FakeAction
	UnitNames []string
}

// FakeDbus is a fake implementation of Dbus that records all actions.
type FakeDbus struct {
	Actions []FakeSystemdAction
}

// DaemonReload implements Dbus.
func (f *FakeDbus) DaemonReload(ctx context.Context) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action: FakeDeamonReload,
	})
	return nil
}

// Disable implements Dbus.
func (f *FakeDbus) Disable(ctx context.Context, unitNames ...string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeDisable,
		UnitNames: unitNames,
	})
	return nil
}

// Enable implements Dbus.
func (f *FakeDbus) Enable(ctx context.Context, unitNames ...string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeEnable,
		UnitNames: unitNames,
	})

	return nil
}

// Restart implements Dbus.
func (f *FakeDbus) Restart(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeRestart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Start implements Dbus.
func (f *FakeDbus) Start(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeStart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Stop implements Dbus.
func (f *FakeDbus) Stop(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeStop,
		UnitNames: []string{unitName},
	})
	return nil
}
