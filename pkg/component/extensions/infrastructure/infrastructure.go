// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 10 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 3 * time.Minute
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of an infrastructure resource.
	DefaultTimeout = 10 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface is an interface for managing Infrastructures.
type Interface interface {
	component.DeployMigrateWaiter
	// Get retrieves and returns the Infrastructure resources based on the configured values.
	Get(context.Context) (*extensionsv1alpha1.Infrastructure, error)
	// SetSSHPublicKey sets the SSH public key in the values.
	SetSSHPublicKey([]byte)
	// ProviderStatus returns the generated status of the provider.
	ProviderStatus() *runtime.RawExtension
	// NodesCIDRs returns the generated nodes CIDRs of the provider.
	NodesCIDRs() []string
	// ServicesCIDRs returns the generated services CIDRs of the provider.
	ServicesCIDRs() []string
	// PodsCIDRs returns the generated pods CIDRs of the provider.
	PodsCIDRs() []string
	// EgressCIDRs returns a list of CIDRs used as source IP by any traffic originating from the shoot's worker nodes.
	EgressCIDRs() []string
}

// Values contains the values used to create an Infrastructure resources.
type Values struct {
	// Namespace is the Shoot namespace in the seed.
	Namespace string
	// Name is the name of the Infrastructure resource. Commonly the Shoot's name.
	Name string
	// Type is the type of infrastructure provider.
	Type string
	// ProviderConfig contains the provider config for the Infrastructure provider.
	ProviderConfig *runtime.RawExtension
	// Region is the region of the shoot.
	Region string
	// SSHPublicKey is the to-be-used SSH public key of the shoot.
	SSHPublicKey []byte
	// AnnotateOperation indicates if the Infrastructure resource shall be annotated with the
	// respective "gardener.cloud/operation" (forcing a reconciliation or restoration). If this is false
	// then the Infrastructure object will be created/updated but the extension controller will not
	// act upon it.
	AnnotateOperation bool
}

// New creates a new instance of Interface.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &infrastructure{
		log:                 log,
		client:              client,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		infrastructure: &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

type infrastructure struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	infrastructure *extensionsv1alpha1.Infrastructure
	providerStatus *runtime.RawExtension
	nodesCIDRs     []string
	servicesCIDRs  []string
	podsCIDRs      []string
	egressCIDRs    []string
}

// Deploy uses the seed client to create or update the Infrastructure resource.
func (i *infrastructure) Deploy(ctx context.Context) error {
	_, err := i.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (i *infrastructure) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var providerConfig *runtime.RawExtension
	if cfg := i.values.ProviderConfig; cfg != nil {
		providerConfig = &runtime.RawExtension{
			Raw: cfg.Raw,
		}
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, i.infrastructure, func() error {
		if i.values.AnnotateOperation || i.lastOperationNotSuccessful() || i.isTimestampInvalidOrAfterLastUpdateTime() {
			// Check if gardener timestamp is in an invalid format or is after status.LastOperation.LastUpdateTime.
			// If that is the case health checks for the infrastructure will fail so we request a reconciliation to correct the current state.
			metav1.SetMetaDataAnnotation(&i.infrastructure.ObjectMeta, v1beta1constants.GardenerOperation, operation)
			metav1.SetMetaDataAnnotation(&i.infrastructure.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))
		}

		i.infrastructure.Spec = extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           i.values.Type,
				ProviderConfig: providerConfig,
			},
			Region:       i.values.Region,
			SSHPublicKey: i.values.SSHPublicKey,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: i.infrastructure.Namespace,
			},
		}
		return nil
	})

	return i.infrastructure, err
}

// Restore uses the seed client and the ShootState to create the Infrastructure resources and restore their state.
func (i *infrastructure) Restore(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	return extensions.RestoreExtensionWithDeployFunction(
		ctx,
		i.client,
		shootState,
		extensionsv1alpha1.InfrastructureResource,
		i.deploy,
	)
}

// Migrate migrates the Infrastructure resources.
func (i *infrastructure) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObject(
		ctx,
		i.client,
		i.infrastructure,
	)
}

// Destroy deletes the Infrastructure resource.
func (i *infrastructure) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		i.client,
		i.infrastructure,
	)
}

// Wait waits until the Infrastructure resource is ready.
func (i *infrastructure) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		i.client,
		i.log,
		i.infrastructure,
		extensionsv1alpha1.InfrastructureResource,
		i.waitInterval,
		i.waitSevereThreshold,
		i.waitTimeout,
		func() error {
			i.extractStatus(i.infrastructure.Status)
			return nil
		})
}

// WaitMigrate waits until the Infrastructure resources are migrated successfully.
func (i *infrastructure) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(
		ctx,
		i.client,
		i.infrastructure,
		extensionsv1alpha1.InfrastructureResource,
		i.waitInterval,
		i.waitTimeout,
	)
}

// WaitCleanup waits until the Infrastructure resource is deleted.
func (i *infrastructure) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		i.client,
		i.log,
		i.infrastructure,
		extensionsv1alpha1.InfrastructureResource,
		i.waitInterval,
		i.waitTimeout,
	)
}

// Get retrieves and returns the Infrastructure resources based on the configured values.
func (i *infrastructure) Get(ctx context.Context) (*extensionsv1alpha1.Infrastructure, error) {
	if err := i.client.Get(ctx, client.ObjectKeyFromObject(i.infrastructure), i.infrastructure); err != nil {
		return nil, err
	}

	i.extractStatus(i.infrastructure.Status)
	return i.infrastructure, nil
}

// SetSSHPublicKey sets the SSH public key in the values.
func (i *infrastructure) SetSSHPublicKey(key []byte) {
	i.values.SSHPublicKey = key
}

// ProviderStatus returns the generated status of the provider.
func (i *infrastructure) ProviderStatus() *runtime.RawExtension {
	return i.providerStatus
}

// NodesCIDRs returns the generated nodes CIDRs of the provider.
func (i *infrastructure) NodesCIDRs() []string {
	return i.nodesCIDRs
}

// ServicesCIDRs returns the generated services CIDRs of the provider.
func (i *infrastructure) ServicesCIDRs() []string {
	return i.servicesCIDRs
}

// PodsCIDRs returns the generated pods CIDRs of the provider.
func (i *infrastructure) PodsCIDRs() []string {
	return i.podsCIDRs
}

// EgressCIDRs returns a list of CIDRs used as source IP by any traffic originating from the shoot's worker nodes.
func (i *infrastructure) EgressCIDRs() []string {
	return i.egressCIDRs
}

func (i *infrastructure) extractStatus(status extensionsv1alpha1.InfrastructureStatus) {
	i.providerStatus = status.ProviderStatus
	if status.NodesCIDR != nil {
		nodes := *status.NodesCIDR
		i.nodesCIDRs = []string{nodes}
	}
	if status.Networking != nil {
		existingNodes := sets.New(i.nodesCIDRs...)
		for _, n := range status.Networking.Nodes {
			if !existingNodes.Has(n) {
				i.nodesCIDRs = append(i.nodesCIDRs, n)
			}
		}
		i.podsCIDRs = make([]string, len(status.Networking.Pods))
		copy(i.podsCIDRs, status.Networking.Pods)
		i.servicesCIDRs = make([]string, len(status.Networking.Services))
		copy(i.servicesCIDRs, status.Networking.Services)
	}
	i.egressCIDRs = make([]string, len(status.EgressCIDRs))
	copy(i.egressCIDRs, status.EgressCIDRs)
}

func (i *infrastructure) lastOperationNotSuccessful() bool {
	return i.infrastructure.Status.LastOperation != nil && i.infrastructure.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded
}

// isTimestampInvalidOrAfterLastUpdateTime returns true if v1beta1constants.GardenerTimestamp is after status.LastOperation.LastUpdateTime
// or if v1beta1constants.GardenerTimestamp is in invalid format
func (i *infrastructure) isTimestampInvalidOrAfterLastUpdateTime() bool {
	timestamp, ok := i.infrastructure.Annotations[v1beta1constants.GardenerTimestamp]
	if ok && i.infrastructure.Status.LastOperation != nil {
		parsedTimestamp, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			// this should not happen
			// we cannot do anything meaningful about this error so we mark the timestamp invalid
			return true
		}

		if parsedTimestamp.Truncate(time.Second).UTC().After(i.infrastructure.Status.LastOperation.LastUpdateTime.UTC()) {
			return true
		}
	}

	return false
}
