// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	extensionscmdcontroller "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	extensionscontrolplanecontroller "github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	extensionsdnsrecordcontroller "github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionshealthcheckcontroller "github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	extensionsheartbeatcontroller "github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	extensionsinfrastructurecontroller "github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsoperatingsystemconfigcontroller "github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionscmdwebhook "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	extensionscontrolplanewebhook "github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	extensionsshootwebhook "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	backupbucketcontroller "github.com/gardener/gardener/pkg/provider-local/controller/backupbucket"
	backupentrycontroller "github.com/gardener/gardener/pkg/provider-local/controller/backupentry"
	controlplanecontroller "github.com/gardener/gardener/pkg/provider-local/controller/controlplane"
	dnsrecordcontroller "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
	localextensionseedcontroller "github.com/gardener/gardener/pkg/provider-local/controller/extension/seed"
	localextensionshootcontroller "github.com/gardener/gardener/pkg/provider-local/controller/extension/shoot"
	localextensionshootafterworkercontroller "github.com/gardener/gardener/pkg/provider-local/controller/extension/shootafterworker"
	healthcheckcontroller "github.com/gardener/gardener/pkg/provider-local/controller/healthcheck"
	infrastructurecontroller "github.com/gardener/gardener/pkg/provider-local/controller/infrastructure"
	ingresscontroller "github.com/gardener/gardener/pkg/provider-local/controller/ingress"
	networkpolicycontroller "github.com/gardener/gardener/pkg/provider-local/controller/networkpolicy"
	operatingsystemconfigcontroller "github.com/gardener/gardener/pkg/provider-local/controller/operatingsystemconfig"
	servicecontroller "github.com/gardener/gardener/pkg/provider-local/controller/service"
	workercontroller "github.com/gardener/gardener/pkg/provider-local/controller/worker"
	controlplanewebhook "github.com/gardener/gardener/pkg/provider-local/webhook/controlplane"
	dnsconfigwebhook "github.com/gardener/gardener/pkg/provider-local/webhook/dnsconfig"
	networkpolicywebhook "github.com/gardener/gardener/pkg/provider-local/webhook/networkpolicy"
	nodewebhook "github.com/gardener/gardener/pkg/provider-local/webhook/node"
	prometheuswebhook "github.com/gardener/gardener/pkg/provider-local/webhook/prometheus"
	rolloutspeedupwebhook "github.com/gardener/gardener/pkg/provider-local/webhook/rolloutspeedup"
	shootwebhook "github.com/gardener/gardener/pkg/provider-local/webhook/shoot"
)

// ControllerSwitchOptions are the extensionscmdcontroller.SwitchOptions for the provider controllers.
func ControllerSwitchOptions() *extensionscmdcontroller.SwitchOptions {
	return extensionscmdcontroller.NewSwitchOptions(
		extensionscmdcontroller.Switch(backupbucketcontroller.ControllerName, backupbucketcontroller.AddToManager),
		extensionscmdcontroller.Switch(backupentrycontroller.ControllerName, backupentrycontroller.AddToManager),
		extensionscmdcontroller.Switch(extensionscontrolplanecontroller.ControllerName, controlplanecontroller.AddToManager),
		extensionscmdcontroller.Switch(extensionsdnsrecordcontroller.ControllerName, dnsrecordcontroller.AddToManager),
		extensionscmdcontroller.Switch(extensionsinfrastructurecontroller.ControllerName, infrastructurecontroller.AddToManager),
		extensionscmdcontroller.Switch(extensionsworkercontroller.ControllerName, workercontroller.AddToManager),
		extensionscmdcontroller.Switch(ingresscontroller.ControllerName, ingresscontroller.AddToManager),
		extensionscmdcontroller.Switch(servicecontroller.ControllerName, servicecontroller.AddToManager),
		extensionscmdcontroller.Switch(extensionshealthcheckcontroller.ControllerName, healthcheckcontroller.AddToManager),
		extensionscmdcontroller.Switch(extensionsoperatingsystemconfigcontroller.ControllerName, operatingsystemconfigcontroller.AddToManager),
		extensionscmdcontroller.Switch(extensionsheartbeatcontroller.ControllerName, extensionsheartbeatcontroller.AddToManager),
		extensionscmdcontroller.Switch(localextensionseedcontroller.ControllerName, localextensionseedcontroller.AddToManager),
		extensionscmdcontroller.Switch(localextensionshootcontroller.ControllerName, localextensionshootcontroller.AddToManager),
		extensionscmdcontroller.Switch(localextensionshootafterworkercontroller.ControllerName, localextensionshootafterworkercontroller.AddToManager),
		extensionscmdcontroller.Switch(networkpolicycontroller.ControllerName, networkpolicycontroller.AddToManager),
	)
}

// WebhookSwitchOptions are the extensionscmdwebhook.SwitchOptions for the provider webhooks.
func WebhookSwitchOptions() *extensionscmdwebhook.SwitchOptions {
	return extensionscmdwebhook.NewSwitchOptions(
		extensionscmdwebhook.Switch(extensionscontrolplanewebhook.WebhookName, controlplanewebhook.AddToManager),
		extensionscmdwebhook.Switch(extensionsshootwebhook.WebhookName, shootwebhook.AddToManager),
		extensionscmdwebhook.Switch(dnsconfigwebhook.WebhookName, dnsconfigwebhook.AddToManager),
		extensionscmdwebhook.Switch(rolloutspeedupwebhook.WebhookName, rolloutspeedupwebhook.AddToManager),
		extensionscmdwebhook.Switch(networkpolicywebhook.WebhookName, networkpolicywebhook.AddToManager),
		extensionscmdwebhook.Switch(nodewebhook.WebhookName, nodewebhook.AddToManager),
		extensionscmdwebhook.Switch(nodewebhook.WebhookNameShoot, nodewebhook.AddShootWebhookToManager),
		extensionscmdwebhook.Switch(prometheuswebhook.WebhookName, prometheuswebhook.AddToManager),
	)
}
