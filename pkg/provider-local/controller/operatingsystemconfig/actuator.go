// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type actuator struct {
	client client.Client
}

// NewActuator creates a new Actuator that updates the status of the handled OperatingSystemConfig resources.
func NewActuator(mgr manager.Manager) operatingsystemconfig.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, osc *extensionsv1alpha1.OperatingSystemConfig) ([]byte, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, *extensionsv1alpha1.InPlaceUpdatesStatus, error) {
	switch purpose := osc.Spec.Purpose; purpose {
	case extensionsv1alpha1.OperatingSystemConfigPurposeProvision:
		userData, err := a.handleProvisionOSC(ctx, osc)
		return []byte(userData), nil, nil, nil, err

	case extensionsv1alpha1.OperatingSystemConfigPurposeReconcile:
		extensionUnits, inPlaceUpdates := a.handleReconcileOSC(osc)
		return nil, extensionUnits, nil, inPlaceUpdates, nil

	default:
		return nil, nil, nil, nil, fmt.Errorf("unknown purpose: %s", purpose)
	}
}

func (a *actuator) Delete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.OperatingSystemConfig) error {
	return nil
}

func (a *actuator) Migrate(ctx context.Context, log logr.Logger, osc *extensionsv1alpha1.OperatingSystemConfig) error {
	return a.Delete(ctx, log, osc)
}

func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, osc *extensionsv1alpha1.OperatingSystemConfig) error {
	return a.Delete(ctx, log, osc)
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, osc *extensionsv1alpha1.OperatingSystemConfig) ([]byte, []extensionsv1alpha1.Unit, []extensionsv1alpha1.File, *extensionsv1alpha1.InPlaceUpdatesStatus, error) {
	return a.Reconcile(ctx, log, osc)
}

func (a *actuator) handleProvisionOSC(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig) (string, error) {
	writeFilesToDiskScript, err := operatingsystemconfig.FilesToDiskScript(ctx, a.client, osc.Namespace, osc.Spec.Files)
	if err != nil {
		return "", err
	}
	writeUnitsToDiskScript := operatingsystemconfig.UnitsToDiskScript(osc.Spec.Units)

	script := `#!/bin/bash
` + writeFilesToDiskScript + `
` + writeUnitsToDiskScript + `
systemctl daemon-reload
`
	for _, unit := range osc.Spec.Units {
		script += fmt.Sprintf(`systemctl enable '%s' && systemctl restart --no-block '%s'
`, unit.Name, unit.Name)
	}

	return operatingsystemconfig.WrapProvisionOSCIntoOneshotScript(script), nil
}

func (a *actuator) handleReconcileOSC(osc *extensionsv1alpha1.OperatingSystemConfig) ([]extensionsv1alpha1.Unit, *extensionsv1alpha1.InPlaceUpdatesStatus) {
	var (
		extensionUnits       []extensionsv1alpha1.Unit
		inPlaceUpdatesStatus *extensionsv1alpha1.InPlaceUpdatesStatus
	)

	extensionUnits = append(extensionUnits,
		// Add explicit dependencies from and to systemd-user-sessions.service. As there is no unit depending on it, it will
		// never be executed by systemd on local machine pods. Without this, non-privileged users are not able to log into
		// the machine (e.g., via SSH) because the /run/nologin file (created by systemd-tmpfiles-setup.service) is not
		// deleted.
		// The drop-in configures the unit to run after the /run/nologin file has been created. And it explicitly marks the
		// unit as required by multi-user.target (dependency of the default boot target) so that it is definitely executed,
		// even if no other unit depends on it.
		extensionsv1alpha1.Unit{
			Name: "systemd-user-sessions.service",
			DropIns: []extensionsv1alpha1.DropIn{{
				Name: "dependencies.conf",
				Content: `[Unit]
Wants=systemd-tmpfiles-setup.service
After=systemd-tmpfiles-setup.service
[Install]
WantedBy=multi-user.target`,
			}},
		},
	)

	if osc.Spec.InPlaceUpdates != nil {
		inPlaceUpdatesStatus = &extensionsv1alpha1.InPlaceUpdatesStatus{
			OSUpdate: &extensionsv1alpha1.OSUpdate{
				Command: "sed",
				Args: []string{
					"-i", "-E",
					fmt.Sprintf(
						`s/^PRETTY_NAME="[^"]*"/PRETTY_NAME="Machine Image Version %s (version overwritten for tests, check VERSION_ID for actual version)"/`,
						osc.Spec.InPlaceUpdates.OperatingSystemVersion,
					),
					"/etc/os-release",
				},
			},
		}
	}

	return extensionUnits, inPlaceUpdatesStatus
}
