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

package extensions

import (
	"context"
	"fmt"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type shootStateControl struct {
	k8sGardenClient kubernetes.Interface
	seedClient      kubernetes.Interface
	log             *logrus.Entry
	recorder        record.EventRecorder
	shootRetriever  *ShootRetriever
}

func (s *shootStateControl) createShootStateSyncReconcileFunc(ctx context.Context, kind string, objectCreator func() runtime.Object) reconcile.Func {
	return func(req reconcile.Request) (reconcile.Result, error) {
		obj := objectCreator()
		err := s.seedClient.Client().Get(ctx, req.NamespacedName, obj)
		if apierrors.IsNotFound(err) {
			s.log.Debugf("Skipping ShootState sync because resource with kind %s is missing in namespace %s", kind, req.NamespacedName)
			return reconcile.Result{}, nil
		}
		if err != nil {
			return reconcile.Result{}, err
		}

		extensionObject, err := apiextensions.Accessor(obj)
		if err != nil {
			return reconcile.Result{}, err
		}

		if shouldSkipExtensionObjectSync(extensionObject) {
			return reconcile.Result{}, nil
		}

		clusterName := fromRequest(req)
		cluster := &extensionsv1alpha1.Cluster{}
		if err := s.seedClient.Client().Get(ctx, kutil.Key(clusterName), cluster); err != nil {
			if apierrors.IsNotFound(err) {
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, fmt.Errorf("could not get cluster with name %s : %v", clusterName, err)
		}

		shoot, err := s.shootRetriever.FromCluster(cluster)
		if err != nil {
			return reconcile.Result{}, err
		}

		name := extensionObject.GetName()
		purpose := extensionObject.GetExtensionSpec().GetExtensionPurpose()
		state := extensionObject.GetExtensionStatus().GetState()
		resources := extensionObject.GetExtensionStatus().GetResources()

		shootState := &gardencorev1alpha1.ShootState{ObjectMeta: metav1.ObjectMeta{Name: shoot.Name, Namespace: shoot.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, s.k8sGardenClient.Client(), shootState, func() error {
			// Delete resource data for resources that are no longer present in the extension state
			_, currentResources := getShootStateExtensionState(shootState, kind, &name, purpose)
			for _, currentResource := range currentResources {
				if gardencorev1beta1helper.GetResourceByName(resources, currentResource.Name) == nil {
					updateShootStateResourceData(shootState, &currentResource.ResourceRef, nil)
				}
			}

			// Update the extension state
			updateShootStateExtensionState(shootState, kind, &name, purpose, state, resources)

			// Update resource data for resources that are still present in the extension state
			for _, resource := range resources {
				obj, err := utils.GetObjectByRef(ctx, s.seedClient.Client(), &resource.ResourceRef, extensionObject.GetNamespace())
				if err != nil {
					return err
				}
				if obj == nil {
					return fmt.Errorf("object not found %v", resource.ResourceRef)
				}
				raw := &runtime.RawExtension{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, raw); err != nil {
					return err
				}
				updateShootStateResourceData(shootState, &resource.ResourceRef, raw)
			}
			return nil
		}); err != nil {
			message := fmt.Sprintf("Shoot's %s %s extension state with name %s and purpose %s was NOT successfully synced: %v", shoot.Name, kind, name, purposeToString(purpose), err)
			s.log.Error(message)
			return reconcile.Result{}, err
		}

		message := fmt.Sprintf("Shoot's %s %s extension state with name %s and purpose %s was successfully synced", shoot.Name, kind, name, purposeToString(purpose))
		s.log.Info(message)
		return reconcile.Result{}, nil
	}
}

func shouldSkipExtensionObjectSync(extensionObject extensionsv1alpha1.Object) bool {
	if extensionObject.GetDeletionTimestamp() != nil {
		return true
	}

	annotations := extensionObject.GetAnnotations()
	if annotations != nil {
		operationAnnotation := annotations[v1beta1constants.GardenerOperation]
		return operationAnnotation == v1beta1constants.GardenerOperationWaitForState ||
			operationAnnotation == v1beta1constants.GardenerOperationRestore ||
			operationAnnotation == v1beta1constants.GardenerOperationMigrate
	}
	return false
}

func getShootStateExtensionState(shootState *gardencorev1alpha1.ShootState, kind string, name, purpose *string) (*runtime.RawExtension, []gardencorev1beta1.NamedResourceReference) {
	list := gardencorev1alpha1helper.ExtensionResourceStateList(shootState.Spec.Extensions)
	s := list.Get(kind, name, purpose)
	if s != nil {
		return s.State, s.Resources
	}
	return nil, nil
}

func updateShootStateExtensionState(shootState *gardencorev1alpha1.ShootState, kind string, name, purpose *string, state *runtime.RawExtension, resources []gardencorev1beta1.NamedResourceReference) {
	list := gardencorev1alpha1helper.ExtensionResourceStateList(shootState.Spec.Extensions)
	if state != nil || resources != nil {
		list.Upsert(&gardencorev1alpha1.ExtensionResourceState{
			Kind:      kind,
			Name:      name,
			Purpose:   purpose,
			State:     state,
			Resources: resources,
		})
	} else {
		list.Delete(kind, name, purpose)
	}
	shootState.Spec.Extensions = list
}

func updateShootStateResourceData(shootState *gardencorev1alpha1.ShootState, ref *autoscalingv1.CrossVersionObjectReference, data *runtime.RawExtension) {
	list := gardencorev1alpha1helper.ResourceDataList(shootState.Spec.Resources)
	if data != nil {
		list.Upsert(&gardencorev1alpha1.ResourceData{
			CrossVersionObjectReference: *ref,
			Data:                        *data,
		})
	} else {
		list.Delete(ref)
	}
	shootState.Spec.Resources = list
}

func purposeToString(purpose *string) string {
	if purpose == nil {
		return "<nil>"
	}
	return *purpose
}

func fromRequest(req reconcile.Request) (clusterName string) {
	if req.Namespace == "" {
		// Handling for cluster-scoped backupentry extension resources.
		clusterName, _ = common.ExtractShootDetailsFromBackupEntryName(req.Name)
	} else {
		clusterName = req.Namespace
	}
	return
}
