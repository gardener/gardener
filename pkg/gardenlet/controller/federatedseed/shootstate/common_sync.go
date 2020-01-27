// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootstate

import (
	"context"
	"fmt"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	shootoperations "github.com/gardener/gardener/pkg/operation/shoot"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createEnqueueOnAddFunc(queue workqueue.RateLimitingInterface) func(extensionObject interface{}) {
	return func(newObj interface{}) {
		extensionObject := newObj.(extensionsv1alpha1.Object)
		if extensionObject.GetExtensionStatus().GetState() == nil {
			return
		}
		enqueue(queue, newObj)
	}
}

func createEnqueueOnUpdateFunc(queue workqueue.RateLimitingInterface) func(newExtensionObject, oldExtensionObject interface{}) {
	return func(newObj, oldObj interface{}) {
		var (
			newExtensionObj = newObj.(extensionsv1alpha1.Object)
			oldExtensionObj = oldObj.(extensionsv1alpha1.Object)
		)

		if apiequality.Semantic.DeepEqual(newExtensionObj.GetExtensionStatus().GetState(), oldExtensionObj.GetExtensionStatus().GetState()) {
			return
		}

		enqueue(queue, newObj)
	}
}

func enqueue(queue workqueue.RateLimitingInterface, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	queue.Add(key)
}

func (s *SyncController) createShootStateSyncReconcileFunc(ctx context.Context, kind string, objectCreator func() runtime.Object) reconcile.Func {
	return func(req reconcile.Request) (reconcile.Result, error) {
		obj := objectCreator()
		err := s.seedClient.Client().Get(ctx, req.NamespacedName, obj)
		if apierrors.IsNotFound(err) {
			s.log.Debugf("Skipping ShootState sync because resoruce with kind %s is missing in namespace %s", kind, req.NamespacedName)
			return reconcile.Result{}, nil
		}
		if err != nil {
			return reconcile.Result{}, err
		}

		extensionObject, err := apiextensions.Accessor(obj)
		if err != nil {
			return reconcile.Result{}, err
		}

		if extensionObject.GetDeletionTimestamp() != nil {
			return reconcile.Result{}, nil
		}

		name := extensionObject.GetName()
		purpose := extensionObject.GetExtensionSpec().GetExtensionPurpose()
		state := extensionObject.GetExtensionStatus().GetState()
		projectNamespace, shootName := fromNamespace(req.Namespace)

		shoot, err := s.k8sGardenClient.GardenCore().CoreV1beta1().Shoots(projectNamespace).Get(shootName, metav1.GetOptions{})
		if err != nil {
			s.log.Errorf("Couldn't retrieve Shoot %s from namespace %s. Extension %s state with name %s and purpose %v was NOT synced: %v", shootName, projectNamespace, kind, name, purpose, err)
			return reconcile.Result{}, err
		}

		ownerReference := metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))

		blockOwnerDeletion := false
		ownerReference.BlockOwnerDeletion = &blockOwnerDeletion
		shootState := &gardencorev1alpha1.ShootState{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: projectNamespace}}

		if _, err := controllerutil.CreateOrUpdate(ctx, s.k8sGardenClient.Client(), shootState, func() error {
			shootState.OwnerReferences = []metav1.OwnerReference{*ownerReference}
			return updateShootStateExtensionState(state, shootState, kind, name, purpose)
		}); err != nil {
			message := fmt.Sprintf("Shoot's %s %s extension state with name %s and purpose %v was NOT successfully synced: %v", shootName, kind, name, purpose, err)
			s.log.Error(message)
			s.recorder.Event(shootState, corev1.EventTypeNormal, "ScheduledNextSync", message)
			return reconcile.Result{}, err
		}

		message := fmt.Sprintf("Shoot's %s %s extension state with name %s and purpose %v was successfully synced", shootName, kind, name, purpose)
		s.log.Info(message)
		s.recorder.Event(shootState, corev1.EventTypeNormal, "ScheduledNextSync", message)
		return reconcile.Result{}, nil
	}
}

func updateShootStateExtensionState(extensionState *runtime.RawExtension, shootState *gardencorev1alpha1.ShootState, kind string, name string, purpose *string) error {
	i, currentExtensionState := gardencorev1alpha1helper.GetExtensionResourceState(shootState.Spec.Extensions, kind, &name, purpose)
	if extensionState == nil && i > -1 {
		lastElementIndex := len(shootState.Spec.Extensions) - 1
		shootState.Spec.Extensions[i] = shootState.Spec.Extensions[lastElementIndex]
		shootState.Spec.Extensions = shootState.Spec.Extensions[0:lastElementIndex]
		return nil
	}

	if currentExtensionState == nil {
		shootState.Spec.Extensions = append(shootState.Spec.Extensions, gardencorev1alpha1.ExtensionResourceState{
			Kind:    kind,
			Name:    &name,
			Purpose: purpose,
			State:   gardencorev1alpha1.ProviderConfig{RawExtension: *extensionState},
		})
	} else {
		currentExtensionState.State = gardencorev1alpha1.ProviderConfig{RawExtension: *extensionState}
	}
	return nil
}

func fromNamespace(namespace string) (projectNamespace, shootName string) {
	projectName, shootName := shootoperations.UnfoldTechnicalID(namespace)
	projectNamespace = "garden-" + projectName
	return
}
