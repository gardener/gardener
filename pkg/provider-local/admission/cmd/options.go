// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	extensionscmdwebhook "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/gardener/gardener/pkg/provider-local/admission/mutator"
)

// GardenWebhookSwitchOptions are the extensionscmdwebhook.SwitchOptions for the admission webhooks.
func GardenWebhookSwitchOptions() *extensionscmdwebhook.SwitchOptions {
	return extensionscmdwebhook.NewSwitchOptions(
		extensionscmdwebhook.Switch(mutator.Name, mutator.New),
	)
}
