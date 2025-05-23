// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	extensionscmdwebhook "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/gardener/gardener/pkg/provider-local/admission/mutator"
	"github.com/gardener/gardener/pkg/provider-local/admission/validator"
)

// GardenWebhookSwitchOptions are the extensionscmdwebhook.SwitchOptions for the admission webhooks.
func GardenWebhookSwitchOptions() *extensionscmdwebhook.SwitchOptions {
	return extensionscmdwebhook.NewSwitchOptions(
		extensionscmdwebhook.Switch(validator.Name, validator.New),
		extensionscmdwebhook.Switch(validator.SecretsValidatorName, validator.NewSecretsWebhook),
		extensionscmdwebhook.Switch(mutator.Name, mutator.New),
	)
}
