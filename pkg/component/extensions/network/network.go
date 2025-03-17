// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"
	"net"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of a network resource.
	DefaultTimeout = 3 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface is an interface for managing the network CRD deployment.
type Interface interface {
	component.DeployMigrateWaiter
	SetPodCIDRs([]net.IPNet)
	SetServiceCIDRs([]net.IPNet)
	Get(ctx context.Context) (*extensionsv1alpha1.Network, error)
}

// Values contains the values used to create a Network CRD
type Values struct {
	// Namespace is the namespace of the Shoot network in the Seed
	Namespace string
	// Name is the name of the Network extension. Commonly the Shoot's name.
	Name string
	// Type is the type of Network plugin/extension (e.g calico)
	Type string
	// IPFamilies specifies the IP protocol versions to use for shoot networking.
	IPFamilies []extensionsv1alpha1.IPFamily
	// ProviderConfig contains the provider config for the Network extension.
	ProviderConfig *runtime.RawExtension
	// PodCIDRs are the Shoot's pod CIDRs in the Shoot VPC
	PodCIDRs []net.IPNet
	// ServiceCIDRs are the Shoot's service CIDRs in the Shoot VPC
	ServiceCIDRs []net.IPNet
}

// New creates a new instance of DeployWaiter for a Network.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &network{
		client:              client,
		log:                 log,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		network: &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

type network struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	network *extensionsv1alpha1.Network
}

// Deploy uses the seed client to create or update the Network custom resource in the Shoot namespace in the Seed
func (n *network) Deploy(ctx context.Context) error {
	_, err := n.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

// Restore uses the seed client and the ShootState to create the Network custom resource in the Shoot namespace in the Seed and restore its state
func (n *network) Restore(ctx context.Context, shootState *v1beta1.ShootState) error {
	return extensions.RestoreExtensionWithDeployFunction(
		ctx,
		n.client,
		shootState,
		extensionsv1alpha1.NetworkResource,
		n.deploy,
	)
}

// Migrate migrates the Network custom resource
func (n *network) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObject(
		ctx,
		n.client,
		n.network,
	)
}

// WaitMigrate waits until the Network custom resource has been successfully migrated.
func (n *network) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(
		ctx,
		n.client,
		n.network,
		extensionsv1alpha1.NetworkResource,
		n.waitInterval,
		n.waitTimeout,
	)
}

// Destroy deletes the Network CRD
func (n *network) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		n.client,
		n.network,
	)
}

// Wait waits until the Network CRD is ready (deployed or restored)
func (n *network) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		n.client,
		n.log,
		n.network,
		extensionsv1alpha1.NetworkResource,
		n.waitInterval,
		n.waitSevereThreshold,
		n.waitTimeout,
		nil,
	)
}

// WaitCleanup waits until the Network CRD is deleted
func (n *network) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		n.client,
		n.log,
		n.network,
		extensionsv1alpha1.NetworkResource,
		n.waitInterval,
		n.waitTimeout,
	)
}

func getCIDRforSpec(ipFamilies []extensionsv1alpha1.IPFamily, PodCIDRs []net.IPNet) string {
	if len(ipFamilies) == 2 && ipFamilies[0] == extensionsv1alpha1.IPFamilyIPv6 {
		return PodCIDRs[1].String()
	}
	return PodCIDRs[0].String()
}

func (n *network) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, n.network, func() error {
		metav1.SetMetaDataAnnotation(&n.network.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&n.network.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))

		n.network.Spec = extensionsv1alpha1.NetworkSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           n.values.Type,
				ProviderConfig: n.values.ProviderConfig,
			},
			IPFamilies:  n.values.IPFamilies,
			PodCIDR:     getCIDRforSpec(n.values.IPFamilies, n.values.PodCIDRs),
			ServiceCIDR: getCIDRforSpec(n.values.IPFamilies, n.values.ServiceCIDRs),
		}

		return nil
	})

	return n.network, err
}

func (n *network) SetPodCIDRs(pods []net.IPNet) {
	n.values.PodCIDRs = pods
}

func (n *network) SetServiceCIDRs(services []net.IPNet) {
	n.values.ServiceCIDRs = services
}

// Get retrieves and returns the Network resources based on the configured values.
func (n *network) Get(ctx context.Context) (*extensionsv1alpha1.Network, error) {
	if err := n.client.Get(ctx, client.ObjectKeyFromObject(n.network), n.network); err != nil {
		return nil, err
	}
	return n.network, nil
}
