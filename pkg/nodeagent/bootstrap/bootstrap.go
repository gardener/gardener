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

package bootstrap

import (
	"context"
	_ "embed"
	"fmt"
	"path"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"

	nodeagentcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// Bootstrap bootstraps the gardener-node-agent by adding and starting its systemd unit and afterward disabling the
// gardener-node-init. If `kubeletDataVolumeSize` is non-zero, it formats the data device.
func Bootstrap(
	ctx context.Context,
	log logr.Logger,
	fs afero.Afero,
	dbus dbus.DBus,
	bootstrapConfig *config.BootstrapConfiguration,
) error {
	log.Info("Starting bootstrap procedure")

	if bootstrapConfig != nil && bootstrapConfig.KubeletDataVolumeSize != nil {
		log.Info("Start kubelet data volume formatter", "kubeletDataVolumeSize", *bootstrapConfig.KubeletDataVolumeSize)
		if err := formatKubeletDataDevice(log.WithName("kubelet-data-volume-device-formatter"), fs, *bootstrapConfig.KubeletDataVolumeSize); err != nil {
			return fmt.Errorf("failed formatting kubelet data volume: %w", err)
		}
	}

	unitFilePath := path.Join("/", "etc", "systemd", "system", nodeagentv1alpha1.UnitName)
	log.Info("Writing unit file for gardener-node-agent", "path", unitFilePath)
	if err := fs.WriteFile(unitFilePath, []byte(nodeagentcomponent.UnitContent()), 0644); err != nil {
		return fmt.Errorf("unable to write unit file %q to path %q: %w", nodeagentv1alpha1.UnitName, unitFilePath, err)
	}

	log.Info("Reloading systemd daemon")
	if err := dbus.DaemonReload(ctx); err != nil {
		return fmt.Errorf("failed reloading systemd daemon: %w", err)
	}

	log.Info("Enabling gardener-node-agent unit")
	if err := dbus.Enable(ctx, nodeagentv1alpha1.UnitName); err != nil {
		return fmt.Errorf("unable to enable unit %q: %w", nodeagentv1alpha1.UnitName, err)
	}

	log.Info("Starting gardener-node-agent unit")
	if err := dbus.Start(ctx, nil, nil, nodeagentv1alpha1.UnitName); err != nil {
		return fmt.Errorf("unable to start unit %q: %w", nodeagentv1alpha1.UnitName, err)
	}

	log.Info("Disabling gardener-node-init unit")
	if err := dbus.Disable(ctx, nodeagentv1alpha1.InitUnitName); err != nil {
		return fmt.Errorf("unable to disable system unit %q: %w", nodeagentv1alpha1.InitUnitName, err)
	}

	// Stop itself must be the last action. With this command, the execution of the gardener-node-agent bootstrap
	// command terminates. It is not possible to perform any logic after this line.
	log.Info("Bootstrap procedure finished, stopping gardener-node-init unit (triggers self-termination)")
	return dbus.Stop(ctx, nil, nil, nodeagentv1alpha1.InitUnitName)
}
