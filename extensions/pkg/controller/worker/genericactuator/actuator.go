// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package genericactuator

import (
	"context"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// GardenPurposeMachineClass is a constant for the 'machineclass' value in a label.
const GardenPurposeMachineClass = "machineclass"

type genericActuator struct {
	logger logr.Logger

	delegateFactory DelegateFactory
	mcmName         string
	mcmSeedChart    util.Chart
	mcmShootChart   util.Chart
	imageVector     imagevector.ImageVector

	client               client.Client
	clientset            kubernetes.Interface
	decoder              runtime.Decoder
	gardenerClientset    gardenerkubernetes.Interface
	chartApplier         gardenerkubernetes.ChartApplier
	chartRendererFactory extensionscontroller.ChartRendererFactory
}

// NewActuator creates a new Actuator that reconciles
// Worker resources of Gardener's `extensions.gardener.cloud` API group.
// It provides a default implementation that allows easier integration of providers.
func NewActuator(logger logr.Logger, delegateFactory DelegateFactory, mcmName string, mcmSeedChart, mcmShootChart util.Chart, imageVector imagevector.ImageVector, chartRendererFactory extensionscontroller.ChartRendererFactory) worker.Actuator {
	return &genericActuator{
		logger: logger.WithName("worker-actuator"),

		delegateFactory:      delegateFactory,
		mcmName:              mcmName,
		mcmSeedChart:         mcmSeedChart,
		mcmShootChart:        mcmShootChart,
		imageVector:          imageVector,
		chartRendererFactory: chartRendererFactory,
	}
}

func (a *genericActuator) InjectFunc(f inject.Func) error {
	return f(a.delegateFactory)
}

func (a *genericActuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

func (a *genericActuator) InjectScheme(scheme *runtime.Scheme) error {
	a.decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
	return nil
}

func (a *genericActuator) InjectConfig(config *rest.Config) error {
	var err error

	a.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "could not create Kubernetes client")
	}

	a.gardenerClientset, err = gardenerkubernetes.NewWithConfig(gardenerkubernetes.WithRESTConfig(config))
	if err != nil {
		return errors.Wrap(err, "could not create Gardener client")
	}

	a.chartApplier, err = gardenerkubernetes.NewChartApplierForConfig(config)
	if err != nil {
		return errors.Wrap(err, "could not create chart applier")
	}

	return nil
}

func (a *genericActuator) cleanupMachineDeployments(ctx context.Context, existingMachineDeployments *machinev1alpha1.MachineDeploymentList, wantedMachineDeployments worker.MachineDeployments) error {
	for _, existingMachineDeployment := range existingMachineDeployments.Items {
		if !wantedMachineDeployments.HasDeployment(existingMachineDeployment.Name) {
			if err := a.client.Delete(ctx, &existingMachineDeployment); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *genericActuator) listMachineClassNames(ctx context.Context, namespace string, machineClassList runtime.Object) (sets.String, error) {
	if err := a.client.List(ctx, machineClassList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	classNames := sets.NewString()

	if err := meta.EachListItem(machineClassList, func(machineClass runtime.Object) error {
		accessor, err := meta.Accessor(machineClass)
		if err != nil {
			return err
		}

		classNames.Insert(accessor.GetName())
		return nil
	}); err != nil {
		return nil, err
	}

	return classNames, nil
}

func (a *genericActuator) cleanupMachineClasses(ctx context.Context, namespace string, machineClassList runtime.Object, wantedMachineDeployments worker.MachineDeployments) error {
	if err := a.client.List(ctx, machineClassList, client.InNamespace(namespace)); err != nil {
		return err
	}

	return meta.EachListItem(machineClassList, func(machineClass runtime.Object) error {
		accessor, err := meta.Accessor(machineClass)
		if err != nil {
			return err
		}

		if !wantedMachineDeployments.HasClass(accessor.GetName()) {
			if err := a.client.Delete(ctx, machineClass); err != nil {
				return err
			}
		}

		return nil
	})
}

func (a *genericActuator) listMachineClassSecrets(ctx context.Context, namespace string) (*corev1.SecretList, error) {
	var (
		secretList           = &corev1.SecretList{}
		deprecatedSecretList = &corev1.SecretList{}
		labels               = map[string]string{
			v1beta1constants.GardenerPurpose: GardenPurposeMachineClass,
		}
		deprecatedLabels = map[string]string{
			"garden.sapcloud.io/purpose": GardenPurposeMachineClass,
		}
	)

	if err := a.client.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	if err := a.client.List(ctx, deprecatedSecretList, client.InNamespace(namespace), client.MatchingLabels(deprecatedLabels)); err != nil {
		return nil, err
	}

	for _, depSecret := range deprecatedSecretList.Items {
		exists := false
		for _, secret := range secretList.Items {
			if depSecret.Name == secret.Name {
				exists = true
				break
			}
		}
		if !exists {
			secretList.Items = append(secretList.Items, depSecret)
		}
	}

	return secretList, nil
}

// cleanupMachineClassSecrets deletes all unused machine class secrets (i.e., those which are not part
// of the provided list <usedSecrets>.
func (a *genericActuator) cleanupMachineClassSecrets(ctx context.Context, namespace string, wantedMachineDeployments worker.MachineDeployments) error {
	secretList, err := a.listMachineClassSecrets(ctx, namespace)
	if err != nil {
		return err
	}

	// Cleanup all secrets which were used for machine classes that do not exist anymore.
	for _, secret := range secretList.Items {
		if !wantedMachineDeployments.HasSecret(secret.Name) {
			if err := a.client.Delete(ctx, &secret); err != nil {
				return err
			}
		}
	}

	return nil
}

// shallowDeleteMachineClassSecrets deletes all unused machine class secrets (i.e., those which are not part
// of the provided list <usedSecrets>) without waiting for MCM to do this.
func (a *genericActuator) shallowDeleteMachineClassSecrets(ctx context.Context, namespace string, wantedMachineDeployments worker.MachineDeployments) error {
	secretList, err := a.listMachineClassSecrets(ctx, namespace)
	if err != nil {
		return err
	}
	// Delete the finalizers to all secrets which were used for machine classes that do not exist anymore.
	for _, secret := range secretList.Items {
		if !wantedMachineDeployments.HasSecret(secret.Name) {
			if err := extensionscontroller.DeleteAllFinalizers(ctx, a.client, &secret); err != nil {
				return errors.Wrapf(err, "Error removing finalizer from MachineClassSecret: %s/%s", secret.Namespace, secret.Name)
			}
			if err := a.client.Delete(ctx, &secret); err != nil {
				return err
			}
		}
	}

	return nil
}

// cleanupMachineClassSecrets deletes MachineSets having number of desired and actual replicas equaling 0
func (a *genericActuator) cleanupMachineSets(ctx context.Context, namespace string) error {
	machineSetList := &machinev1alpha1.MachineSetList{}
	if err := a.client.List(ctx, machineSetList, client.InNamespace(namespace)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for _, machineSet := range machineSetList.Items {
		if machineSet.Spec.Replicas == 0 && machineSet.Status.Replicas == 0 {
			a.logger.Info("Deleting MachineSet as the number of desired and actual replicas is 0.", "name", machineSet.Name)
			if err := a.client.Delete(ctx, machineSet.DeepCopy()); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}
	return nil
}

func (a *genericActuator) shallowDeleteAllObjects(ctx context.Context, namespace string, objectList runtime.Object) error {
	if err := a.client.List(ctx, objectList, client.InNamespace(namespace)); err != nil {
		return err
	}

	return meta.EachListItem(objectList, func(obj runtime.Object) error {
		object := obj.DeepCopyObject()
		if err := extensionscontroller.DeleteAllFinalizers(ctx, a.client, object); err != nil {
			return err
		}
		if err := a.client.Delete(ctx, object); client.IgnoreNotFound(err) != nil {
			return err
		}
		return nil
	})
}

// CleanupLeakedClusterRoles cleans up leaked ClusterRoles from the system that were created earlier without
// owner references. See https://github.com/gardener-attic/gardener-extensions/pull/378/files and
// https://github.com/gardener/gardener/issues/2144.
// TODO: This code can be removed in a future version again.
func CleanupLeakedClusterRoles(ctx context.Context, c client.Client, providerName string) error {
	clusterRoleList := &rbacv1.ClusterRoleList{}
	if err := c.List(ctx, clusterRoleList); err != nil {
		return err
	}

	var (
		namespaces    = sets.NewString()
		namespaceList = &corev1.NamespaceList{}
		fns           []flow.TaskFn
	)

	if err := c.List(ctx, namespaceList); err != nil {
		return err
	}
	for _, namespace := range namespaceList.Items {
		namespaces.Insert(namespace.Name)
	}

	for _, clusterRole := range clusterRoleList.Items {
		clusterRoleName := clusterRole.Name
		if !strings.HasPrefix(clusterRoleName, "extensions.gardener.cloud:"+providerName) || !strings.HasSuffix(clusterRoleName, ":machine-controller-manager") {
			continue
		}

		split := strings.Split(clusterRoleName, ":")
		if len(split) != 4 {
			continue
		}
		if namespace := split[2]; namespaces.Has(namespace) {
			continue
		}

		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName}}))
		})
	}

	return flow.Parallel(fns...)(ctx)
}
