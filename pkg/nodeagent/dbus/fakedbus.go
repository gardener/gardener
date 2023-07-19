package dbus

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

type FakeAction int

const (
	FakeDeamonReload FakeAction = iota
	FakeDisable
	FakeEnable
	FakeRestart
	FakeStart
	FakeStop
)

type FakeSystemdAction struct {
	Action    FakeAction
	UnitNames []string
}

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
func (f *FakeDbus) Restart(ctx context.Context, recorder record.EventRecorder, node *v1.Node, unitName string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeRestart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Start implements Dbus.
func (f *FakeDbus) Start(ctx context.Context, recorder record.EventRecorder, node *v1.Node, unitName string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeStart,
		UnitNames: []string{unitName},
	})
	return nil
}

// Stop implements Dbus.
func (f *FakeDbus) Stop(ctx context.Context, recorder record.EventRecorder, node *v1.Node, unitName string) error {
	f.Actions = append(f.Actions, FakeSystemdAction{
		Action:    FakeStop,
		UnitNames: []string{unitName},
	})
	return nil
}
