// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// SwitchOptions are the cmd.SwitchOptions for the provider controllers.
func SwitchOptions(ctrlName string, osTypes []string, generator generator.Generator) *cmd.SwitchOptions {
	return cmd.NewSwitchOptions(
		cmd.Switch(operatingsystemconfig.ControllerName, func(mgr manager.Manager) error {
			return oscommon.AddToManager(mgr, ctrlName, osTypes, generator)
		}),
	)
}
