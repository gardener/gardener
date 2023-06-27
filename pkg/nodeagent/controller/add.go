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

package controller

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/afero"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/controller/kubeletupgrade"
	"github.com/gardener/gardener/pkg/nodeagent/controller/node"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/nodeagent/controller/selfupgrade"
	"github.com/gardener/gardener/pkg/nodeagent/controller/token"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// AddToManager adds all gardener-node-agent controllers to the given manager.
func AddToManager(mgr manager.Manager) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	fs := afero.NewOsFs()

	// TODO: Why don't we read this during start-up and pass it down here, just like in all other Gardener binaries?
	config, err := common.ReadNodeAgentConfiguration(fs)
	if err != nil {
		return err
	}

	var (
		nodeName              = strings.TrimSpace(strings.ToLower(hostname))
		selfUpgradeChannel    = make(chan event.GenericEvent, 1)
		kubeletUpgradeChannel = make(chan event.GenericEvent, 1)
		db                    = dbus.New()
	)

	if err := (&node.Reconciler{
		NodeName:   nodeName,
		SyncPeriod: 10 * time.Minute,
		Dbus:       db,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding node controller: %w", err)
	}

	if err := (&operatingsystemconfig.Reconciler{
		NodeName:   nodeName,
		Config:     config,
		SyncPeriod: 10 * time.Minute,
		TriggerChannels: []chan event.GenericEvent{
			selfUpgradeChannel,
			kubeletUpgradeChannel,
		},
		Dbus: db,
		Fs:   fs,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding operatingsystemconfig controller: %w", err)
	}

	if err := (&kubeletupgrade.Reconciler{
		Config:           config,
		TargetBinaryPath: "/opt/bin/kubelet",
		SyncPeriod:       10 * time.Minute,
		TriggerChannel:   kubeletUpgradeChannel,
		Dbus:             db,
		Fs:               fs,
		Extractor:        registry.NewExtractor(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding kubelet upgrade controller: %w", err)
	}

	if err := (&selfupgrade.Reconciler{
		NodeName:       nodeName,
		Config:         config,
		SelfBinaryPath: os.Args[0],
		SyncPeriod:     10 * time.Minute,
		TriggerChannel: selfUpgradeChannel,
		Dbus:           db,
		Extractor:      registry.NewExtractor(),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding self upgrade controller: %w", err)
	}

	if err := (&token.Reconciler{
		Config:     config,
		SyncPeriod: 10 * time.Minute,
		Fs:         fs,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding token controller: %w", err)
	}

	return nil
}
