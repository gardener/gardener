// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerapiserver

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// DeploymentName is the name of the deployment.
	DeploymentName = "gardener-apiserver"

	managedResourceNameRuntime = "gardener-apiserver-runtime"
	managedResourceNameVirtual = "gardener-apiserver-virtual"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Interface contains functions for a gardener-apiserver deployer.
type Interface interface {
	apiserver.Interface
	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
}

// Values contains configuration values for the gardener-apiserver resources.
type Values struct {
	apiserver.Values
	// Image is the container images used for the gardener-apiserver pods.
	Image string
}

// New creates a new instance of DeployWaiter for the gardener-apiserver.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) Interface {
	return &gardenerAPIServer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerAPIServer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerAPIServer) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)

		configMapAuditPolicy              = g.emptyConfigMap(configMapAuditPolicyNamePrefix)
		configMapAdmissionConfigs         = g.emptyConfigMap(configMapAdmissionNamePrefix)
		secretAdmissionKubeconfigs        = g.emptySecret(secretAdmissionKubeconfigsNamePrefix)
		secretETCDEncryptionConfiguration = g.emptySecret(v1beta1constants.SecretNamePrefixGardenerETCDEncryptionConfiguration)
		secretAuditWebhookKubeconfig      = g.emptySecret(secretAuditWebhookKubeconfigNamePrefix)
		virtualGardenAccessSecret         = g.newVirtualGardenAccessSecret()
	)

	secretServer, err := g.reconcileSecretServer(ctx)
	if err != nil {
		return err
	}

	if err := virtualGardenAccessSecret.Reconcile(ctx, g.client); err != nil {
		return err
	}

	if err := g.reconcileSecretETCDEncryptionConfiguration(ctx, secretETCDEncryptionConfiguration); err != nil {
		return err
	}

	if err := apiserver.ReconcileConfigMapAdmission(ctx, g.client, configMapAdmissionConfigs, g.values.Values); err != nil {
		return err
	}
	if err := apiserver.ReconcileSecretAdmissionKubeconfigs(ctx, g.client, secretAdmissionKubeconfigs, g.values.Values); err != nil {
		return err
	}

	if err := apiserver.ReconcileConfigMapAuditPolicy(ctx, g.client, configMapAuditPolicy, g.values.Audit); err != nil {
		return err
	}
	if err := apiserver.ReconcileSecretAuditWebhookKubeconfig(ctx, g.client, secretAuditWebhookKubeconfig, g.values.Audit); err != nil {
		return err
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		g.podDisruptionBudget(),
	)
	if err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := managedresources.CreateForSeed(ctx, g.client, g.namespace, managedResourceNameRuntime, false, runtimeResources); err != nil {
		return err
	}
	if err := managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, managedResourceNameRuntime); err != nil {
		return err
	}

	var (
		virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)
	)

	virtualResources, err := virtualRegistry.AddAllAndSerialize()
	if err != nil {
		return err
	}

	timeoutCtx, cancel = context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := managedresources.CreateForShoot(ctx, g.client, g.namespace, managedResourceNameVirtual, managedresources.LabelValueGardener, false, virtualResources); err != nil {
		return err
	}
	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, managedResourceNameVirtual)
}

func (g *gardenerAPIServer) Destroy(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, managedResourceNameVirtual); err != nil {
		return err
	}
	if err := managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, managedResourceNameVirtual); err != nil {
		return err
	}

	timeoutCtx, cancel = context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, managedResourceNameRuntime); err != nil {
		return err
	}
	if err := managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, managedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

func (g *gardenerAPIServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := g.waitUntilRuntimeManagedResourceHealthyAndNotProgressing(ctx); err != nil {
		return err
	}
	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, managedResourceNameVirtual)
}

func (g *gardenerAPIServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	if err := managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, managedResourceNameVirtual); err != nil {
		return err
	}
	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, managedResourceNameRuntime)
}

func (g *gardenerAPIServer) waitUntilRuntimeManagedResourceHealthyAndNotProgressing(ctx context.Context) error {
	obj := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedResourceNameRuntime,
			Namespace: g.namespace,
		},
	}

	return retry.Until(ctx, managedresources.IntervalWait, func(ctx context.Context) (done bool, err error) {
		if err := g.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckManagedResource(obj); err != nil {
			return retry.MinorError(fmt.Errorf("managed resource %s is unhealthy", client.ObjectKeyFromObject(obj)))
		}

		if err := health.CheckManagedResourceProgressing(obj); err != nil {
			return retry.MinorError(fmt.Errorf("managed resource %s is still progressing", client.ObjectKeyFromObject(obj)))
		}

		return retry.Ok()
	})
}

func (g *gardenerAPIServer) GetValues() Values {
	return g.values
}

func (g *gardenerAPIServer) GetAutoscalingReplicas() *int32 {
	return g.values.Autoscaling.Replicas
}

func (g *gardenerAPIServer) SetAutoscalingAPIServerResources(resources corev1.ResourceRequirements) {
	g.values.Autoscaling.APIServerResources = resources
}

func (g *gardenerAPIServer) SetAutoscalingReplicas(replicas *int32) {
	g.values.Autoscaling.Replicas = replicas
}

func (g *gardenerAPIServer) SetETCDEncryptionConfig(config apiserver.ETCDEncryptionConfig) {
	g.values.ETCDEncryption = config
}

// GetLabels returns the labels for the gardener-apiserver.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}
