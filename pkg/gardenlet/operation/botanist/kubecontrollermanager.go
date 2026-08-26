// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const kubeRootCAConfigMapUpdateQPS = 100

// DefaultKubeControllerManager returns a deployer for the kube-controller-manager.
func (b *Botanist) DefaultKubeControllerManager() (kubecontrollermanager.Interface, error) {
	return shared.NewKubeControllerManager(
		b.Logger,
		b.SeedClientSet,
		b.Shoot.ControlPlaneNamespace,
		b.Shoot.RuntimeKubernetesVersion,
		b.Shoot.KubernetesVersion,
		b.SecretsManager,
		"",
		b.Shoot.GetInfo().Spec.Kubernetes.KubeControllerManager,
		v1beta1constants.PriorityClassNameShootControlPlane300,
		b.Shoot.IsWorkerless,
		metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.ShootAlphaControlPlaneScaleDownDisabled),
		nil,
		kubecontrollermanager.ControllerWorkers{},
		kubecontrollermanager.ControllerSyncPeriods{},
		nil,
	)
}

// DeployKubeControllerManager deploys the Kubernetes Controller Manager.
func (b *Botanist) DeployKubeControllerManager(ctx context.Context) error {
	replicaCount, err := b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameKubeControllerManager, 1)
	if err != nil {
		return err
	}
	if b.Shoot.RunsControlPlane() {
		replicaCount = 0
	}

	b.Shoot.Components.ControlPlane.KubeControllerManager.SetReplicaCount(replicaCount)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetRuntimeConfig(b.Shoot.Components.ControlPlane.KubeAPIServer.GetValues().RuntimeConfig)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetServiceNetworks(b.Shoot.Networks.Services)
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetPodNetworks(b.Shoot.Networks.Pods)

	return b.Shoot.Components.ControlPlane.KubeControllerManager.Deploy(ctx)
}

// WaitForKubeControllerManagerToBeActive waits for the kube controller manager of a Shoot cluster has acquired leader election, thus is active.
func (b *Botanist) WaitForKubeControllerManagerToBeActive(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.KubeControllerManager.SetShootClient(b.ShootClientSet.Client())

	return b.Shoot.Components.ControlPlane.KubeControllerManager.WaitForControllerToBeActive(ctx)
}

// ScaleKubeControllerManagerToOne scales kube-controller-manager replicas to one.
func (b *Botanist) ScaleKubeControllerManagerToOne(ctx context.Context) error {
	return kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameKubeControllerManager}, 1)
}

// WaitUntilKubeRootCAConfigMapsUpdated verifies that all kube-root-ca.crt ConfigMaps in all namespaces of the shoot
// cluster contain the CA bundle published by kube-controller-manager. It labels each confirmed ConfigMap with
// credentials.gardener.cloud/ca-bundle-name so that retries skip already-verified ConfigMaps. This should only be
// called during the 'Preparing' phase of the CA rotation operation, after kube-controller-manager is ready.
func (b *Botanist) WaitUntilKubeRootCAConfigMapsUpdated(ctx context.Context) error {
	caBundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	configMapList := &corev1.ConfigMapList{}
	if err := b.ShootClientSet.Client().List(ctx, configMapList,
		client.MatchingFields{"metadata.name": "kube-root-ca.crt"},
		client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(secretsrotation.LabelKeyCABundleName, selection.NotEquals, caBundleSecret.Name))},
	); err != nil {
		return fmt.Errorf("failed listing kube-root-ca.crt ConfigMaps with outdated CA bundle: %w", err)
	}

	if len(configMapList.Items) == 0 {
		return nil
	}

	b.Logger.Info("Found kube-root-ca.crt ConfigMaps not yet confirmed to contain the new CA bundle", "number", len(configMapList.Items))

	var (
		limiter  = rate.NewLimiter(rate.Limit(kubeRootCAConfigMapUpdateQPS), kubeRootCAConfigMapUpdateQPS)
		notReady atomic.Int32
		taskFns  []flow.TaskFn
	)

	for _, configMap := range configMapList.Items {
		taskFns = append(taskFns, func(ctx context.Context) error {
			if caBundleInConfigMap, ok := configMap.Data[secretsutils.DataKeyCertificateCA]; !ok || !bytes.Equal([]byte(caBundleInConfigMap), caBundleSecret.Data[secretsutils.DataKeyCertificateBundle]) {
				notReady.Add(1)
				return nil
			}

			if err := limiter.Wait(ctx); err != nil {
				return fmt.Errorf("error while waiting for limiter: %w", err)
			}

			patch := client.MergeFrom(configMap.DeepCopy())
			metav1.SetMetaDataLabel(&configMap.ObjectMeta, secretsrotation.LabelKeyCABundleName, caBundleSecret.Name)
			return b.ShootClientSet.Client().Patch(ctx, &configMap, patch)
		})
	}

	if err := flow.Parallel(taskFns...)(ctx); err != nil {
		return fmt.Errorf("error while ensuring kube-root-ca.crt ConfigMaps are updated: %w", err)
	}

	if n := notReady.Load(); n > 0 {
		return fmt.Errorf("%d kube-root-ca.crt ConfigMap(s) do not yet contain the expected CA bundle", n)
	}
	return nil
}
