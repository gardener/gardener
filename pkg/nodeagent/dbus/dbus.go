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

package dbus

import (
	"context"
	"fmt"

	"github.com/coreos/go-systemd/v22/dbus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

const done = "done"

// Dbus is an interface for interacting with systemd via dbus
type Dbus interface {
	// Enable the given units, same as executing systemctl enable unit
	Enable(ctx context.Context, unitNames ...string) error
	// Disable the given units, same as executing systemctl disable unit
	Disable(ctx context.Context, unitNames ...string) error
	// Start the given unit and record an event to the node object, same as executing systemctl start unit
	Start(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error
	// Stop the given unit and record an event to the node object, same as executing systemctl stop unit
	Stop(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error
	// Restart the given unit and record an event to the node object, same as executing systemctl restart unit
	Restart(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error
	// DaemonReload reload the systemd configuration, same as executing systemctl daemon-reload
	DaemonReload(ctx context.Context) error
}

type db struct{}

// New returns a new working Dbus
func New() Dbus {
	return &db{}
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

func (*db) Stop(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	jobCh := make(chan string)

	if _, err := dbc.StopUnitContext(ctx, unitName, "replace", jobCh); err != nil {
		return fmt.Errorf("unable to stop unit %s: %w", unitName, err)
	}

	if completion := <-jobCh; completion != done {
		err = fmt.Errorf("stop failed for %s, due %s", unitName, completion)
	}

	recordEvent(recorder, node, err, unitName, "SystemDUnitStop", "stop")
	return err
}

func (*db) Start(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	jobCh := make(chan string)

	if _, err := dbc.StartUnitContext(ctx, unitName, "replace", jobCh); err != nil {
		return fmt.Errorf("unable to start unit %s: %w", unitName, err)
	}

	completion := <-jobCh
	if completion != done {
		err = fmt.Errorf("start failed for %s, due %s", unitName, completion)
	}

	recordEvent(recorder, node, err, unitName, "SystemDUnitStart", "start")
	return err
}

func (*db) Restart(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	jobCh := make(chan string)

	if _, err := dbc.RestartUnitContext(ctx, unitName, "replace", jobCh); err != nil {
		return fmt.Errorf("unable to restart unit %s: %w", unitName, err)
	}

	completion := <-jobCh
	if completion != done {
		err = fmt.Errorf("restart failed for %s, due %s", unitName, completion)
	}

	recordEvent(recorder, node, err, unitName, "SystemDUnitRestart", "restart")
	return nil
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

func recordEvent(recorder record.EventRecorder, node *corev1.Node, err error, unitName, reason, msg string) {
	if recorder != nil && node != nil {
		eventType := corev1.EventTypeNormal
		if err != nil {
			eventType = corev1.EventTypeWarning
		}
		recorder.Eventf(node, eventType, reason, "processed %s of unit %q with error %v", msg, unitName, err)
	}
}
