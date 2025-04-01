// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"strings"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	fieldOwner        = client.FieldOwner("machine-controller-manager-provider-local")
	labelKeyApp       = "app"
	labelKeyProvider  = "machine-provider"
	labelValueMachine = "machine"
	machinePrefix     = "machine-"
)

// NewDriver returns an empty AWSDriver object
func NewDriver(client client.Client) driver.Driver {
	return &localDriver{client}
}

type localDriver struct {
	client client.Client
}

// GenerateMachineClassForMigration is not implemented.
func (d *localDriver) GenerateMachineClassForMigration(_ context.Context, _ *driver.GenerateMachineClassForMigrationRequest) (*driver.GenerateMachineClassForMigrationResponse, error) {
	return &driver.GenerateMachineClassForMigrationResponse{}, nil
}

// InitializeMachine is not implemented.
func (*localDriver) InitializeMachine(context.Context, *driver.InitializeMachineRequest) (*driver.InitializeMachineResponse, error) {
	return nil, status.Error(codes.Unimplemented, "InitializeMachine is not yet implemented")
}

func podForMachine(machine *machinev1alpha1.Machine, machineClass *machinev1alpha1.MachineClass) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName(machine.Name),
			Namespace: getNamespaceForMachine(machine, machineClass),
		},
	}
}

func userDataSecretForMachine(machine *machinev1alpha1.Machine, machineClass *machinev1alpha1.MachineClass) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName(machine.Name) + "-userdata",
			Namespace: getNamespaceForMachine(machine, machineClass),
		},
	}
}

func podName(machineName string) string {
	return machinePrefix + machineName
}

func machineName(podName string) string {
	return strings.TrimPrefix(podName, machinePrefix)
}

// TODO(scheererj): Remove the empty namespace mitigation after https://github.com/gardener/machine-controller-manager/pull/932 has been adopted
func getNamespaceForMachine(machine *machinev1alpha1.Machine, machineClass *machinev1alpha1.MachineClass) string {
	// machine.Namespace may be empty due to machine controller manager omitting namespace
	if machine.Namespace != "" {
		return machine.Namespace
	}
	return machineClass.Namespace
}
