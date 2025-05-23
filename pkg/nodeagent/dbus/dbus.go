// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dbus

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/coreos/go-systemd/v22/login1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// DBus is an interface for interacting with systemd via dbus.
type DBus interface {
	// DaemonReload reload the systemd configuration, same as executing "systemctl daemon-reload".
	DaemonReload(ctx context.Context) error
	// Enable the given units, same as executing "systemctl enable unit".
	Enable(ctx context.Context, unitNames ...string) error
	// Disable the given units, same as executing "systemctl disable unit".
	Disable(ctx context.Context, unitNames ...string) error
	// Start the given unit and record an event to the node object, same as executing "systemctl start unit".
	Start(ctx context.Context, recorder record.EventRecorder, node runtime.Object, unitName string) error
	// Stop the given unit and record an event to the node object, same as executing "systemctl stop unit".
	Stop(ctx context.Context, recorder record.EventRecorder, node runtime.Object, unitName string) error
	// Restart the given unit and record an event to the node object, same as executing "systemctl restart unit".
	Restart(ctx context.Context, recorder record.EventRecorder, node runtime.Object, unitName string) error
	// List lists all units and returns the output.
	List(ctx context.Context) ([]dbus.UnitStatus, error)
	// Reboot this machines, is the same as executing "systemctl reboot".
	Reboot() error
}

type db struct {
	log logr.Logger
}

// New returns a new working DBus
func New(log logr.Logger) DBus {
	return &db{log: log.WithName("dbus")}
}
func (*db) Reboot() error {
	login, err := login1.New()
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer login.Close()

	login.Reboot(false)
	return nil
}

func (*db) Enable(ctx context.Context, unitNames ...string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	_, _, err = dbc.EnableUnitFilesContext(ctx, unitNames, false, true)
	return err
}

func (*db) Disable(ctx context.Context, unitNames ...string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	_, err = dbc.DisableUnitFilesContext(ctx, unitNames, false)
	return err
}

func (d *db) Stop(ctx context.Context, recorder record.EventRecorder, node runtime.Object, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	return d.runCommand(ctx, recorder, node, unitName, dbc.StopUnitContext, "SystemDUnitStop", "stop")
}

func (d *db) Start(ctx context.Context, recorder record.EventRecorder, node runtime.Object, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	return d.runCommand(ctx, recorder, node, unitName, dbc.StartUnitContext, "SystemDUnitStart", "start")
}

func (d *db) Restart(ctx context.Context, recorder record.EventRecorder, node runtime.Object, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	return d.runCommand(ctx, recorder, node, unitName, dbc.RestartUnitContext, "SystemDUnitRestart", "restart")
}

func (d *db) List(ctx context.Context) ([]dbus.UnitStatus, error) {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	return dbc.ListUnitsContext(ctx)
}

func (*db) DaemonReload(ctx context.Context) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	if err := dbc.ReloadContext(ctx); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w", err)
	}

	return nil
}

func (d *db) runCommand(
	ctx context.Context,
	recorder record.EventRecorder,
	node runtime.Object,
	unitName string,
	f func(context.Context, string, string, chan<- string) (int, error),
	eventReason string,
	operation string,
) error {
	var (
		resultCh = make(chan string)
		err      error
	)

	if _, err := f(ctx, unitName, "replace", resultCh); err != nil {
		return fmt.Errorf("unable to %s unit %s: %w", operation, unitName, err)
	}

	select {
	case <-ctx.Done(): // context is cancelled
		err = ctx.Err()

	case result := <-resultCh: // job channel reported back
		if result != "done" {
			err = fmt.Errorf("%s failed for %s, due %s", operation, unitName, result)
		}

	case <-time.After(10 * time.Second): // after 10s, we continue even if the result channel did not report back
		d.log.Info("Systemd operation is blocking for more than 10s, aborting and continuing anyways", "unitName", unitName, "operation", operation)
	}

	recordEvent(recorder, node, err, unitName, eventReason, operation)
	return err
}

func recordEvent(recorder record.EventRecorder, node runtime.Object, err error, unitName, reason, operation string) {
	if recorder != nil && node != nil && !reflect.ValueOf(node).IsNil() { // nil is not nil :(
		var (
			eventType = corev1.EventTypeNormal
			message   = fmt.Sprintf("processed %s of unit %s", operation, unitName)
		)

		if err != nil {
			eventType = corev1.EventTypeWarning
			message += fmt.Sprintf(" failed with error %+v", err)
		}

		recorder.Eventf(node, eventType, reason, message)
	}
}
