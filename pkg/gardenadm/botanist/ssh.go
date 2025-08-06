// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"net"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	sshutils "github.com/gardener/gardener/pkg/utils/ssh"
)

func (b *AutonomousBotanist) ConnectToMachine(ctx context.Context, index int) (*sshutils.Connection, error) {
	machineList := &machinev1alpha1.MachineList{}
	if err := b.SeedClientSet.Client().List(ctx, machineList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
		return nil, fmt.Errorf("failed to list machines: %w", err)
	}
	if len(machineList.Items) <= index {
		return nil, fmt.Errorf("only %q machines founds, but wanted to connect to machine with index %d", len(machineList.Items), index)
	}
	machine := machineList.Items[index]
	machineAddr := b.sshAddressForMachine(&machine)

	sshKeypairSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
	if !found {
		return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
	}

	conn, err := sshutils.Dial(ctx, machineAddr,
		sshutils.WithProxyConnection(b.Bastion.Connection),
		sshutils.WithUser("gardener"),
		sshutils.WithPrivateKeyBytes(sshKeypairSecret.Data[secretsutils.DataKeyRSAPrivateKey]),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to machine %q at %q via bastion: %w", machine.Name, machineAddr, err)
	}

	// add a line prefix to distinguish output from different machines
	conn.OutputPrefix = fmt.Sprintf("[%s] ", machine.Name)

	return conn, nil
}

func (b *AutonomousBotanist) sshAddressForMachine(machine *machinev1alpha1.Machine) string {
	// For now, we expect that the Bastion can connect to the control plane machine via its hostname and that the hostname
	// is equal to the machine's name.
	// More cases will be supported once https://github.com/gardener/machine-controller-manager/pull/1012 gets released.
	// TODO(timebertt): prefer Machine.status.addresses if present once mcm has been updated in gardener
	machineHostname := machine.Name

	// Until machine-controller-manager-provider-local reports the correct hostname/IP in Machine.status.addresses, we
	// prefix the machine name with "machine-" because we know that this is the hostname of local machines.
	// TODO(timebertt): drop this shortcut once https://github.com/gardener/gardener/pull/12489 has been merged.
	if b.Shoot.GetInfo().Spec.Provider.Type == "local" {
		machineHostname = "machine-" + machineHostname
	}

	return net.JoinHostPort(machineHostname, "22")
}
