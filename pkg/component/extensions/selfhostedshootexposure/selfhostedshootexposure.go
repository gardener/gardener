// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
)

// Interface manages a SelfHostedShootExposure extension resource.
type Interface interface {
	component.DeployWaiter
	// SetEndpoints replaces the endpoints in the values; the next Deploy call will use the new list.
	SetEndpoints([]extensionsv1alpha1.ControlPlaneEndpoint)
	// GetIngress returns the LoadBalancer ingress from the in-memory exposure object's status.
	// It is populated by Wait once the extension controller reports the resource as Ready.
	GetIngress() []corev1.LoadBalancerIngress
}

// Values contains the values used to create a SelfHostedShootExposure resource.
type Values struct {
	// Namespace is the shoot's control-plane namespace.
	Namespace string
	// Name is the name of the SelfHostedShootExposure resource.
	Name string
	// Type is the extension type (e.g. "local", "aws").
	Type string
	// Class holds the extension class (defaults to ExtensionClassShoot when nil).
	Class *extensionsv1alpha1.ExtensionClass
	// CredentialsRef is a reference to the cloud provider credentials.
	// It is only set for shoots with managed infrastructure.
	CredentialsRef *corev1.ObjectReference
	// Endpoints contains the control-plane nodes that should be exposed.
	Endpoints []extensionsv1alpha1.ControlPlaneEndpoint
}

// SelfHostedShootExposure manages a SelfHostedShootExposure extension resource.
type SelfHostedShootExposure struct {
	log    logr.Logger
	client client.Client
	values *Values

	// exposed for testing
	Clock               clock.PassiveClock
	WaitInterval        time.Duration
	WaitSevereThreshold time.Duration
	WaitTimeout         time.Duration

	exposure *extensionsv1alpha1.SelfHostedShootExposure
}

// New creates a new SelfHostedShootExposure component with the default clock and wait settings.
func New(
	log logr.Logger,
	c client.Client,
	values *Values,
) *SelfHostedShootExposure {
	return &SelfHostedShootExposure{
		log:    log,
		client: c,
		values: values,

		Clock:               &clock.RealClock{},
		WaitInterval:        5 * time.Second,
		WaitSevereThreshold: 30 * time.Second,
		WaitTimeout:         5 * time.Minute,

		exposure: &extensionsv1alpha1.SelfHostedShootExposure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

// Deploy creates or updates the SelfHostedShootExposure resource and triggers a reconciliation
// by setting the gardener.cloud/operation=reconcile annotation.
func (s *SelfHostedShootExposure) Deploy(ctx context.Context) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, s.client, s.exposure, func() error {
		metav1.SetMetaDataAnnotation(&s.exposure.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&s.exposure.ObjectMeta, v1beta1constants.GardenerTimestamp, s.Clock.Now().UTC().Format(time.RFC3339Nano))

		s.exposure.Spec = extensionsv1alpha1.SelfHostedShootExposureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:  s.values.Type,
				Class: s.values.Class,
			},
			CredentialsRef: s.values.CredentialsRef,
			Port:           kubeapiserverconstants.Port,
			Endpoints:      s.values.Endpoints,
		}
		return nil
	})

	return err
}

// Destroy deletes the SelfHostedShootExposure resource.
func (s *SelfHostedShootExposure) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		s.client,
		s.exposure,
	)
}

// Wait waits until the SelfHostedShootExposure resource is ready.
func (s *SelfHostedShootExposure) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		s.client,
		s.log,
		s.exposure,
		extensionsv1alpha1.SelfHostedShootExposureResource,
		s.WaitInterval,
		s.WaitSevereThreshold,
		s.WaitTimeout,
		nil,
	)
}

// WaitCleanup waits until the SelfHostedShootExposure resource is deleted.
func (s *SelfHostedShootExposure) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		s.client,
		s.log,
		s.exposure,
		extensionsv1alpha1.SelfHostedShootExposureResource,
		s.WaitInterval,
		s.WaitTimeout,
	)
}

// SetEndpoints replaces the endpoints in the values; the next Deploy call will use the new list.
func (s *SelfHostedShootExposure) SetEndpoints(endpoints []extensionsv1alpha1.ControlPlaneEndpoint) {
	s.values.Endpoints = endpoints
}

// GetIngress returns the LoadBalancer ingress from the in-memory exposure object's status.
// It is populated by Wait once the extension controller reports the resource as Ready.
func (s *SelfHostedShootExposure) GetIngress() []corev1.LoadBalancerIngress {
	return s.exposure.Status.Ingress
}
