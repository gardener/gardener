// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/cmd"
	extensionsheartbeatcontroller "github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
)

// SwitchOptions are the cmd.SwitchOptions for the provider controllers.
func SwitchOptions(ctrlName string, osTypes []string, generator generator.Generator) *cmd.SwitchOptions {
	return cmd.NewSwitchOptions(
		cmd.Switch(operatingsystemconfig.ControllerName, func(ctx context.Context, mgr manager.Manager) error {
			return oscommon.AddToManager(ctx, mgr, ctrlName, osTypes, generator)
		}),
		cmd.Switch(extensionsheartbeatcontroller.ControllerName, extensionsheartbeatcontroller.AddToManager),
	)
}
