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

package managedresources

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/utils/chart"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources/builder"
	"github.com/gardener/gardener/pkg/utils/retry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SecretPrefix is the prefix that can be used for secrets referenced by managed resources.
	SecretPrefix = "managedresource-"
	// LabelKeyOrigin is a key for a label on a managed resource with the value 'origin'.
	LabelKeyOrigin = "origin"
	// LabelValueGardener is a value for a label on a managed resource with the value 'gardener'.
	LabelValueGardener = "gardener"
)

// SecretName returns the name of a corev1.Secret for the given name of a resourcesv1alpha1.ManagedResource. If
// <withPrefix> is set then the name will be prefixed with 'managedresource-'.
func SecretName(name string, withPrefix bool) string {
	if withPrefix {
		return SecretPrefix + name
	}
	return name
}

// New initiates a new ManagedResource object which can be reconciled.
func New(client client.Client, namespace, name, class string, keepObjects *bool, labels, injectedLabels map[string]string, forceOverwriteAnnotations *bool) *builder.ManagedResource {
	mr := builder.NewManagedResource(client).
		WithNamespacedName(namespace, name).
		WithClass(class).
		WithLabels(labels).
		WithInjectedLabels(injectedLabels)

	if keepObjects != nil {
		mr = mr.KeepObjects(*keepObjects)
	}
	if forceOverwriteAnnotations != nil {
		mr = mr.ForceOverwriteAnnotations(*forceOverwriteAnnotations)
	}

	return mr
}

// NewForShoot constructs a new ManagedResource object for the shoot's Gardener-Resource-Manager.
func NewForShoot(c client.Client, namespace, name string, keepObjects bool) *builder.ManagedResource {
	var (
		injectedLabels = map[string]string{v1beta1constants.ShootNoCleanup: "true"}
		labels         = map[string]string{LabelKeyOrigin: LabelValueGardener}
	)

	return New(c, namespace, name, "", &keepObjects, labels, injectedLabels, nil)
}

// NewForSeed constructs a new ManagedResource object for the seed's Gardener-Resource-Manager.
func NewForSeed(c client.Client, namespace, name string, keepObjects bool) *builder.ManagedResource {
	var labels map[string]string
	if !strings.HasPrefix(namespace, v1beta1constants.TechnicalIDPrefix) {
		labels = map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeedSystemComponent}
	}

	return New(c, namespace, name, v1beta1constants.SeedResourceManagerClass, &keepObjects, labels, nil, nil)
}

// NewSecret initiates a new Secret object which can be reconciled.
func NewSecret(client client.Client, namespace, name string, data map[string][]byte, secretNameWithPrefix bool) (string, *builder.Secret) {
	secretName := SecretName(name, secretNameWithPrefix)
	return secretName, builder.NewSecret(client).
		WithNamespacedName(namespace, secretName).
		WithKeyValues(data)
}

// CreateFromUnstructured creates a managed resource and its secret with the given name, class, and objects in the given namespace.
func CreateFromUnstructured(
	ctx context.Context,
	client client.Client,
	namespace, name string,
	secretNameWithPrefix bool,
	class string,
	objs []*unstructured.Unstructured,
	keepObjects bool,
	injectedLabels map[string]string,
) error {
	var data []byte
	for _, obj := range objs {
		bytes, err := obj.MarshalJSON()
		if err != nil {
			return fmt.Errorf("marshal failed for '%s/%s' for secret '%s/%s': %w", obj.GetNamespace(), obj.GetName(), namespace, name, err)
		}
		data = append(data, []byte("\n---\n")...)
		data = append(data, bytes...)
	}
	return Create(ctx, client, namespace, name, nil, secretNameWithPrefix, class, map[string][]byte{name: data}, &keepObjects, injectedLabels, pointer.Bool(false))
}

// Create creates a managed resource and its secret with the given name, class, key, and data in the given namespace.
func Create(
	ctx context.Context,
	client client.Client,
	namespace, name string,
	labels map[string]string,
	secretNameWithPrefix bool,
	class string,
	data map[string][]byte,
	keepObjects *bool,
	injectedLabels map[string]string,
	forceOverwriteAnnotations *bool,
) error {
	var (
		secretName, secret = NewSecret(client, namespace, name, data, secretNameWithPrefix)
		managedResource    = New(client, namespace, name, class, keepObjects, labels, injectedLabels, forceOverwriteAnnotations).WithSecretRef(secretName)
	)

	return deployManagedResource(ctx, secret, managedResource)
}

// CreateForSeed deploys a ManagedResource CR for the seed's gardener-resource-manager.
func CreateForSeed(ctx context.Context, client client.Client, namespace, name string, keepObjects bool, data map[string][]byte) error {
	var (
		secretName, secret = NewSecret(client, namespace, name, data, true)
		managedResource    = NewForSeed(client, namespace, name, keepObjects).WithSecretRef(secretName)
	)

	return deployManagedResource(ctx, secret, managedResource)
}

// CreateForShoot deploys a ManagedResource CR for the shoot's gardener-resource-manager.
func CreateForShoot(ctx context.Context, client client.Client, namespace, name string, keepObjects bool, data map[string][]byte) error {
	var (
		secretName, secret = NewSecret(client, namespace, name, data, true)
		managedResource    = NewForShoot(client, namespace, name, keepObjects).WithSecretRef(secretName)
	)

	return deployManagedResource(ctx, secret, managedResource)
}

func deployManagedResource(ctx context.Context, secret *builder.Secret, managedResource *builder.ManagedResource) error {
	if err := secret.Reconcile(ctx); err != nil {
		return fmt.Errorf("could not create or update secret of managed resources: %w", err)
	}

	if err := managedResource.Reconcile(ctx); err != nil {
		return fmt.Errorf("could not create or update managed resource: %w", err)
	}

	return nil
}

// Delete deletes the managed resource and its secret with the given name in the given namespace.
func Delete(ctx context.Context, client client.Client, namespace string, name string, secretNameWithPrefix bool) error {
	secretName := SecretName(name, secretNameWithPrefix)

	if err := builder.NewManagedResource(client).
		WithNamespacedName(namespace, name).
		Delete(ctx); err != nil {
		return fmt.Errorf("could not delete managed resource '%s/%s': %w", namespace, name, err)
	}

	if err := builder.NewSecret(client).
		WithNamespacedName(namespace, secretName).
		Delete(ctx); err != nil {
		return fmt.Errorf("could not delete secret '%s/%s' of managed resource: %w", namespace, secretName, err)
	}

	return nil
}

var (
	// DeleteForSeed is a function alias for deleteWithSecretNamePrefix.
	DeleteForSeed = deleteWithSecretNamePrefix
	// DeleteForShoot is a function alias for deleteWithSecretNamePrefix.
	DeleteForShoot = deleteWithSecretNamePrefix
)

func deleteWithSecretNamePrefix(ctx context.Context, client client.Client, namespace string, name string) error {
	return Delete(ctx, client, namespace, name, true)
}

// IntervalWait is the interval when waiting for managed resources.
var IntervalWait = 2 * time.Second

// WaitUntilHealthy waits until the given managed resource is healthy.
func WaitUntilHealthy(ctx context.Context, client client.Client, namespace, name string) error {
	obj := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return retry.Until(ctx, IntervalWait, func(ctx context.Context) (done bool, err error) {
		if err := client.Get(ctx, kutil.Key(namespace, name), obj); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckManagedResource(obj); err != nil {
			return retry.MinorError(fmt.Errorf("managed resource %s/%s is not healthy", namespace, name))
		}

		return retry.Ok()
	})
}

// WaitUntilListDeleted waits until the given managed resources are deleted.
func WaitUntilListDeleted(ctx context.Context, client client.Client, mrList *resourcesv1alpha1.ManagedResourceList, listOps ...client.ListOption) error {
	allErrs := gardencorev1beta1helper.NewMultiErrorWithCodes(
		utilerrors.NewErrorFormatFuncWithPrefix("error while waiting for all resources to be deleted: "),
	)

	if err := kutil.WaitUntilResourcesDeleted(ctx, client, mrList, IntervalWait, listOps...); err != nil {
		for _, mr := range mrList.Items {
			resourcesAppliedCondition := gardencorev1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
			if resourcesAppliedCondition != nil && resourcesAppliedCondition.Status != gardencorev1beta1.ConditionTrue &&
				(resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionFailed || resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionPending) {
				deleteError := fmt.Errorf("%w:\n%s", err, resourcesAppliedCondition.Message)

				allErrs.Append(gardencorev1beta1helper.NewErrorWithCodes(deleteError, checkConfigurationError(err)...))
			}
		}
	}

	return allErrs.ErrorOrNil()
}

// WaitUntilDeleted waits until the given managed resource is deleted.
func WaitUntilDeleted(ctx context.Context, client client.Client, namespace, name string) error {
	mr := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := kutil.WaitUntilResourceDeleted(ctx, client, mr, IntervalWait); err != nil {
		resourcesAppliedCondition := gardencorev1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
		if resourcesAppliedCondition != nil && resourcesAppliedCondition.Status != gardencorev1beta1.ConditionTrue &&
			(resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionFailed || resourcesAppliedCondition.Reason == resourcesv1alpha1.ConditionDeletionPending) {
			deleteError := fmt.Errorf("error while waiting for all resources to be deleted: %w:\n%s", err, resourcesAppliedCondition.Message)
			return gardencorev1beta1helper.NewErrorWithCodes(deleteError, checkConfigurationError(err)...)
		}
		return err
	}
	return nil
}

// SetKeepObjects updates the keepObjects field of the managed resource with the given name in the given namespace.
func SetKeepObjects(ctx context.Context, c client.Writer, namespace, name string, keepObjects bool) error {
	resource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	patch := client.MergeFrom(resource.DeepCopy())
	resource.Spec.KeepObjects = &keepObjects
	if err := c.Patch(ctx, resource, patch); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not update managed resource '%s/%s': %w", namespace, name, err)
	}

	return nil
}

// RenderChartAndCreate renders a chart and creates a ManagedResource for the gardener-resource-manager
// out of the results.
func RenderChartAndCreate(ctx context.Context, namespace string, name string, secretNameWithPrefix bool, client client.Client, chartRenderer chartrenderer.Interface, chart chart.Interface, values map[string]interface{}, imageVector imagevector.ImageVector, chartNamespace string, version string, withNoCleanupLabel bool, forceOverwriteAnnotations bool) error {
	chartName, data, err := chart.Render(chartRenderer, chartNamespace, imageVector, version, version, values)
	if err != nil {
		return fmt.Errorf("could not render chart: %w", err)
	}

	// Create or update managed resource referencing the previously created secret
	var injectedLabels map[string]string
	if withNoCleanupLabel {
		injectedLabels = map[string]string{v1beta1constants.ShootNoCleanup: "true"}
	}

	return Create(ctx, client, namespace, name, nil, secretNameWithPrefix, "", map[string][]byte{chartName: data}, pointer.Bool(false), injectedLabels, &forceOverwriteAnnotations)
}

func checkConfigurationError(err error) []gardencorev1beta1.ErrorCode {
	var (
		errorCodes                 []gardencorev1beta1.ErrorCode
		configurationProblemRegexp = regexp.MustCompile(`(?i)(error during apply of object .* is invalid:)`)
	)

	if configurationProblemRegexp.MatchString(err.Error()) {
		errorCodes = append(errorCodes, gardencorev1beta1.ErrorConfigurationProblem)
	}

	return errorCodes
}
