// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// nolint:revive // this is just a temporary implementation
package botanist

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
)

// FakeOSC is a dummy implementation of operatingsystemconfig.Interface.
// It creates and returns a static user data secret. This is just a temporary implementation for testing the Worker
// deployment until we have a real OperatingSystemConfig for `gardenadm bootstrap`.
// TODO(timebertt): replace this with a proper OperatingSystemConfig component implementation
type FakeOSC struct {
	Client client.Client

	ControlPlaneWorkerPool, ControlPlaneNamespace string
}

const fakeUserDataSecretName = "user-data"

func (f *FakeOSC) Deploy(ctx context.Context) error {
	userDataSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeUserDataSecretName,
			Namespace: f.ControlPlaneNamespace,
		},
		Data: map[string][]byte{
			extensionsv1alpha1.OperatingSystemConfigSecretDataKey: []byte("#!/usr/bin/env bash\necho Hello gardenadm!"),
		},
	}

	return f.Client.Patch(ctx, userDataSecret, client.Apply, client.FieldOwner("gardenadm"), client.ForceOwnership)
}

func (f *FakeOSC) WorkerPoolNameToOperatingSystemConfigsMap() map[string]*operatingsystemconfig.OperatingSystemConfigs {
	return map[string]*operatingsystemconfig.OperatingSystemConfigs{
		f.ControlPlaneWorkerPool: {
			Init: operatingsystemconfig.Data{
				SecretName: ptr.To(fakeUserDataSecretName),
			},
		},
	}
}

func (*FakeOSC) Destroy(context.Context) error                                { panic("implement me") }
func (*FakeOSC) Restore(context.Context, *gardencorev1beta1.ShootState) error { panic("implement me") }
func (*FakeOSC) Migrate(context.Context) error                                { panic("implement me") }
func (*FakeOSC) WaitMigrate(context.Context) error                            { panic("implement me") }
func (*FakeOSC) Wait(context.Context) error                                   { panic("implement me") }
func (*FakeOSC) WaitCleanup(context.Context) error                            { panic("implement me") }
func (*FakeOSC) DeleteStaleResources(context.Context) error                   { panic("implement me") }
func (*FakeOSC) WaitCleanupStaleResources(context.Context) error              { panic("implement me") }
func (*FakeOSC) SetAPIServerURL(string)                                       { panic("implement me") }
func (*FakeOSC) SetCABundle(string)                                           { panic("implement me") }
func (*FakeOSC) SetSSHPublicKeys([]string)                                    { panic("implement me") }
func (*FakeOSC) SetClusterDNSAddresses([]string)                              { panic("implement me") }
func (*FakeOSC) SetCredentialsRotationStatus(*gardencorev1beta1.ShootCredentialsRotation) {
	panic("implement me")
}
