// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	sshutils "github.com/gardener/gardener/pkg/utils/ssh"
)

// Bastion is a component for managing a Bastion (extensions.gardener.cloud) object. It is used for accessing the
// control plane machines in `gardenadm bootstrap`.
type Bastion struct {
	log            logr.Logger
	client         client.Client
	secretsManager secretsmanager.Interface
	Values         *Values

	// exposed for testing
	Clock               clock.PassiveClock
	WaitInterval        time.Duration
	WaitSevereThreshold time.Duration
	WaitTimeout         time.Duration
	SSHDial             func(ctx context.Context, addr string, opts ...sshutils.Option) (*sshutils.Connection, error)

	bastion          *extensionsv1alpha1.Bastion
	sshKeypairSecret *corev1.Secret

	// Connection is the SSH connection to the Bastion opened by Wait.
	Connection *sshutils.Connection
}

// Values contains the values used to create a Bastion.
type Values struct {
	// Name is the Bastion name.
	Name string
	// Namespace is the Bastion namespace (control plane namespace).
	Namespace string
	// Provider is the Bastion provider type.
	Provider string
	// IngressCIDRs restricts ingress to the Bastion.
	// Defaults to ["0.0.0.0/0", "::/0"].
	IngressCIDRs []string
}

// New creates a new Bastion component with the default clock and wait settings.
func New(
	log logr.Logger,
	client client.Client,
	secretsManager secretsmanager.Interface,
	values *Values,
) *Bastion {
	return &Bastion{
		log:            log,
		client:         client,
		secretsManager: secretsManager,
		Values:         values,

		Clock:               &clock.RealClock{},
		WaitInterval:        5 * time.Second,
		WaitSevereThreshold: 30 * time.Second,
		WaitTimeout:         15 * time.Minute,
		SSHDial:             sshutils.Dial,

		bastion: &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

// Deploy generates an SSH key pair and deploys the Bastion object.
func (b *Bastion) Deploy(ctx context.Context) error {
	var err error
	b.sshKeypairSecret, err = b.secretsManager.Generate(ctx, &secretsutils.RSASecretConfig{
		Name:       b.sshKeypairSecretName(),
		Bits:       4096,
		UsedForSSH: true,
	})
	if err != nil {
		return fmt.Errorf("could not generate ssh keypair: %w", err)
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, b.client, b.bastion, func() error {
		metav1.SetMetaDataAnnotation(&b.bastion.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&b.bastion.ObjectMeta, v1beta1constants.GardenerTimestamp, b.Clock.Now().UTC().Format(time.RFC3339Nano))

		b.bastion.Spec = extensionsv1alpha1.BastionSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: b.Values.Provider,
			},
			UserData: userDataForBastion(b.sshKeypairSecret.Data[secretsutils.DataKeySSHAuthorizedKeys]),
			Ingress:  b.ingressPolicies(),
		}
		return nil
	})
	return err
}

// Wait waits for the Bastion object to get ready and opens an SSH connection to the bastion host.
// After a successful Wait call, Connection is ready to be used.
func (b *Bastion) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		b.client,
		b.log,
		b.bastion,
		extensionsv1alpha1.BastionResource,
		b.WaitInterval,
		b.WaitSevereThreshold,
		b.WaitTimeout,
		func() error {
			return b.connect(ctx)
		},
	)
}

// Destroy deletes all resources related to the Bastion object.
func (b *Bastion) Destroy(ctx context.Context) error {
	// close SSH connection on a best-effort basis
	if b.Connection != nil {
		if err := b.Connection.Close(); err != nil {
			b.log.Error(err, "Failed closing SSH connection")
		}
	}

	if err := extensions.DeleteExtensionObject(ctx, b.client, b.bastion); err != nil {
		return fmt.Errorf("failed deleting bastion object %q: %w", client.ObjectKeyFromObject(b.bastion), err)
	}

	// Delete SSH key pair secret after the Bastion has been marked for deletion successfully.
	if err := b.client.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(b.Values.Namespace), client.MatchingLabels{
		secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
		secretsmanager.LabelKeyName:      b.sshKeypairSecretName(),
	}); err != nil {
		return fmt.Errorf("failed deleting ssh keypair secret %q: %w", b.sshKeypairSecretName(), err)
	}

	return nil
}

// WaitCleanup waits until the Bastion object has been deleted successfully.
func (b *Bastion) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		b.client,
		b.log,
		b.bastion,
		extensionsv1alpha1.BastionResource,
		b.WaitInterval,
		b.WaitTimeout,
	)
}

func (b *Bastion) sshKeypairSecretName() string {
	return fmt.Sprintf("bastion-%s-ssh-keypair", b.Values.Name)
}

func (b *Bastion) ingressPolicies() []extensionsv1alpha1.BastionIngressPolicy {
	if len(b.Values.IngressCIDRs) == 0 {
		return []extensionsv1alpha1.BastionIngressPolicy{
			{IPBlock: networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
			{IPBlock: networkingv1.IPBlock{CIDR: "::/0"}},
		}
	}

	policies := make([]extensionsv1alpha1.BastionIngressPolicy, len(b.Values.IngressCIDRs))
	for i, cidr := range b.Values.IngressCIDRs {
		policies[i] = extensionsv1alpha1.BastionIngressPolicy{
			IPBlock: networkingv1.IPBlock{
				CIDR: cidr,
			},
		}
	}
	return policies
}

const bastionUser = "gardener"

func userDataForBastion(sshPublicKey []byte) []byte {
	return []byte(fmt.Sprintf(`#!/bin/bash -eu

id %[1]s || useradd %[1]s -mU
mkdir -p /home/%[1]s/.ssh
echo "%[2]s" > /home/%[1]s/.ssh/authorized_keys
chown %[1]s:%[1]s /home/%[1]s/.ssh/authorized_keys
systemctl start ssh
`, bastionUser, sshPublicKey))
}

// connect is called after the Bastion object has been reconciled successfully. It opens an SSH connection to the
// bastion host and stores it in Connection.
func (b *Bastion) connect(ctx context.Context) error {
	if b.bastion.Status.Ingress == nil {
		return fmt.Errorf("bastion is missing ingress status")
	}

	bastionHost := b.bastion.Status.Ingress.IP
	if bastionHost == "" {
		bastionHost = b.bastion.Status.Ingress.Hostname
	}
	bastionAddr := net.JoinHostPort(bastionHost, "22")

	var err error
	b.Connection, err = b.SSHDial(ctx, bastionAddr,
		sshutils.WithUser(bastionUser),
		sshutils.WithPrivateKeyBytes(b.sshKeypairSecret.Data[secretsutils.DataKeyRSAPrivateKey]),
	)
	if err != nil {
		return fmt.Errorf("error connecting to bastion %q: %w", bastionAddr, err)
	}
	return nil
}
