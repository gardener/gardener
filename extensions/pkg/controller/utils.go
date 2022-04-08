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

package controller

import (
	"context"
	"fmt"
	"reflect"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	localSchemeBuilder = runtime.NewSchemeBuilder(
		scheme.AddToScheme,
		extensionsv1alpha1.AddToScheme,
		resourcesv1alpha1.AddToScheme,
	)

	// AddToScheme adds the Kubernetes and extension scheme to the given scheme.
	AddToScheme = localSchemeBuilder.AddToScheme

	// ExtensionsScheme is the default scheme for extensions, consisting of all Kubernetes built-in
	// schemes (client-go/kubernetes/scheme) and the extensions/v1alpha1 scheme.
	ExtensionsScheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(AddToScheme(ExtensionsScheme))
}

// AddToManagerBuilder aggregates various AddToManager functions.
type AddToManagerBuilder []func(manager.Manager) error

// NewAddToManagerBuilder creates a new AddToManagerBuilder and registers the given functions.
func NewAddToManagerBuilder(funcs ...func(manager.Manager) error) AddToManagerBuilder {
	var builder AddToManagerBuilder
	builder.Register(funcs...)
	return builder
}

// Register registers the given functions in this builder.
func (a *AddToManagerBuilder) Register(funcs ...func(manager.Manager) error) {
	*a = append(*a, funcs...)
}

// AddToManager traverses over all AddToManager-functions of this builder, sequentially applying
// them. It exits on the first error and returns it.
func (a *AddToManagerBuilder) AddToManager(m manager.Manager) error {
	for _, f := range *a {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}

// GetSecretByReference returns the Secret object matching the given SecretReference.
var GetSecretByReference = kutil.GetSecretByReference

// WatchBuilder holds various functions which add watch controls to the passed Controller.
type WatchBuilder []func(controller.Controller) error

// NewWatchBuilder creates a new WatchBuilder and registers the given functions.
func NewWatchBuilder(funcs ...func(controller.Controller) error) WatchBuilder {
	var builder WatchBuilder
	builder.Register(funcs...)
	return builder
}

// Register adds a function which add watch controls to the passed Controller to the WatchBuilder.
func (w *WatchBuilder) Register(funcs ...func(controller.Controller) error) {
	*w = append(*w, funcs...)
}

// AddToController adds the registered watches to the passed controller.
func (w *WatchBuilder) AddToController(ctrl controller.Controller) error {
	for _, f := range *w {
		if err := f(ctrl); err != nil {
			return err
		}
	}
	return nil
}

// UnsafeGuessKind makes an unsafe guess what is the kind of the given object.
//
// The argument to this method _has_ to be a pointer, otherwise it panics.
func UnsafeGuessKind(obj runtime.Object) string {
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("kind of obj %T is not pointer", obj))
	}

	return t.Elem().Name()
}

// GetVerticalPodAutoscalerObject returns unstructured.Unstructured representing vpaautoscalingv1.VerticalPodAutoscaler
func GetVerticalPodAutoscalerObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(vpaautoscalingv1.SchemeGroupVersion.String())
	obj.SetKind("VerticalPodAutoscaler")
	return obj
}

// RemoveAnnotation removes an annotation key passed as annotation
func RemoveAnnotation(ctx context.Context, c client.Client, obj client.Object, annotation string) error {
	withAnnotation := obj.DeepCopyObject().(client.Object)

	annotations := obj.GetAnnotations()
	delete(annotations, annotation)
	obj.SetAnnotations(annotations)

	return c.Patch(ctx, obj, client.MergeFrom(withAnnotation))
}

// IsMigrated checks if an extension object has been migrated
func IsMigrated(obj extensionsv1alpha1.Object) bool {
	lastOp := obj.GetExtensionStatus().GetLastOperation()
	return lastOp != nil &&
		lastOp.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		lastOp.State == gardencorev1beta1.LastOperationStateSucceeded
}

// ShouldSkipOperation checks if the current operation should be skipped depending on the lastOperation of the extension object.
func ShouldSkipOperation(operationType gardencorev1beta1.LastOperationType, obj extensionsv1alpha1.Object) bool {
	return operationType != gardencorev1beta1.LastOperationTypeMigrate && operationType != gardencorev1beta1.LastOperationTypeRestore && IsMigrated(obj)
}

// GetObjectByReference gets an object by the given reference, in the given namespace.
// If the object kind doesn't match the given reference kind this will result in an error.
func GetObjectByReference(ctx context.Context, c client.Client, ref *autoscalingv1.CrossVersionObjectReference, namespace string, obj client.Object) error {
	return c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.ReferencedResourcesPrefix + ref.Name}, obj)
}

// UseTokenRequestor returns true when the provided Gardener version is large enough for supporting acquiring tokens
// for shoot cluster control plane components running in the seed based on the TokenRequestor controller of
// gardener-resource-manager (https://github.com/gardener/gardener/blob/master/docs/concepts/resource-manager.md#tokenrequestor).
// Deprecated: new extension versions need to require at least Gardener version v1.36 and use the token requestor by
// default which makes this function obsolete.
func UseTokenRequestor(gardenerVersion string) (bool, error) {
	return true, nil
}

// UseServiceAccountTokenVolumeProjection returns true when the provided Gardener version is large enough for supporting
// automatic token volume projection for components running in the seed and shoot clusters based on the respective
// webhook part of gardener-resource-manager (https://github.com/gardener/gardener/blob/master/docs/concepts/resource-manager.md#auto-mounting-projected-serviceaccount-tokens).
// Deprecated: new extension versions need to require at least Gardener version v1.37 and use projected service account
// token volumes by default which makes this function obsolete.
func UseServiceAccountTokenVolumeProjection(gardenerVersion string) (bool, error) {
	return true, nil
}
