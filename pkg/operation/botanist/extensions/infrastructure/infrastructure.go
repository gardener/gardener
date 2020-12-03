// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of an infrastructure resource.
	DefaultTimeout = 10 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

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
	// IsInRestorePhaseOfControlPlaneMigration indicates if the Shoot is in the restoration
	// phase of the ControlPlane migration.
	IsInRestorePhaseOfControlPlaneMigration bool
	// DeploymentRequested indicates if the Infrastructure deployment was explicitly requested,
	// i.e., if the Shoot was annotated with the "infrastructure" task.
	DeploymentRequested bool
}

// New creates a new instance of an ExtensionInfrastructure deployer.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
) shoot.ExtensionInfrastructure {
	return &infrastructure{
		client:              client,
		logger:              logger,
		values:              values,
		waitInterval:        DefaultInterval,
		waitSevereThreshold: DefaultSevereThreshold,
		waitTimeout:         DefaultTimeout,
	}
}

type infrastructure struct {
	values              *Values
	logger              logrus.FieldLogger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	providerStatus *runtime.RawExtension
	nodesCIDR      *string
}

// Deploy uses the seed client to create or update the Infrastructure resource.
func (i *infrastructure) Deploy(ctx context.Context) error {
	var (
		operation        = v1beta1constants.GardenerOperationReconcile
		restorePhase     = i.values.IsInRestorePhaseOfControlPlaneMigration
		requestOperation = i.values.DeploymentRequested || restorePhase
		infrastructure   = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      i.values.Name,
				Namespace: i.values.Namespace,
			},
		}
		providerConfig *runtime.RawExtension
	)

	if cfg := i.values.ProviderConfig; cfg != nil {
		providerConfig = &runtime.RawExtension{
			Raw: cfg.Raw,
		}
	}

	if restorePhase {
		operation = v1beta1constants.GardenerOperationWaitForState
	}

	_, err := controllerutil.CreateOrUpdate(ctx, i.client, infrastructure, func() error {
		if requestOperation {
			metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, v1beta1constants.GardenerOperation, operation)
			metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())
		}

		infrastructure.Spec = extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           i.values.Type,
				ProviderConfig: providerConfig,
			},
			Region:       i.values.Region,
			SSHPublicKey: i.values.SSHPublicKey,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: infrastructure.Namespace,
			},
		}
		return nil
	})
	return err
}

// Destroy deletes the Infrastructure resource.
func (i *infrastructure) Destroy(ctx context.Context) error {
	return common.DeleteExtensionCR(
		ctx,
		i.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Infrastructure{} },
		i.values.Namespace,
		i.values.Name,
	)
}

// Wait waits until the Infrastructure resource is ready.
func (i *infrastructure) Wait(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		i.client,
		i.logger,
		func() runtime.Object { return &extensionsv1alpha1.Infrastructure{} },
		extensionsv1alpha1.InfrastructureResource,
		i.values.Namespace,
		i.values.Name,
		i.waitInterval,
		i.waitSevereThreshold,
		i.waitTimeout,
		func(obj runtime.Object) error {
			infrastructure, ok := obj.(*extensionsv1alpha1.Infrastructure)
			if !ok {
				return fmt.Errorf("expected extensionsv1alpha1.Infrastructure but got %T", infrastructure)
			}

			i.providerStatus = infrastructure.Status.ProviderStatus
			i.nodesCIDR = infrastructure.Status.NodesCIDR
			return nil
		},
	)
}

// WaitCleanup waits until the Infrastructure resource is deleted.
func (i *infrastructure) WaitCleanup(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		i.client,
		i.logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Infrastructure{} },
		extensionsv1alpha1.InfrastructureResource,
		i.values.Namespace,
		i.values.Name,
		i.waitInterval,
		i.waitTimeout,
	)
}

// SetSSHPublicKey sets the SSH public key in the values.
func (i *infrastructure) SetSSHPublicKey(key []byte) {
	i.values.SSHPublicKey = key
}

// ProviderStatus returns the generated status of the provider.
func (i *infrastructure) ProviderStatus() *runtime.RawExtension {
	return i.providerStatus
}

// NodesCIDR returns the generated nodes CIDR of the provider.
func (i *infrastructure) NodesCIDR() *string {
	return i.nodesCIDR
}
