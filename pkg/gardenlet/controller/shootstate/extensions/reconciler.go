// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	unstructuredutils "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
)

// Reconciler is used to update data about extensions and any resources required by them in the ShootState.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	Config       config.ShootStateSyncControllerConfiguration

	ObjectKind    string
	NewObjectFunc func() client.Object
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	obj := r.NewObjectFunc()
	if err := r.SeedClient.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	extensionObj, err := apiextensions.Accessor(obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	var (
		name         = extensionObj.GetName()
		purpose      = extensionObj.GetExtensionSpec().GetExtensionPurpose()
		newState     = extensionObj.GetExtensionStatus().GetState()
		newResources = extensionObj.GetExtensionStatus().GetResources()
	)

	if obj.GetDeletionTimestamp() != nil {
		newState, newResources = nil, nil
	}

	log = log.WithValues("extensionName", name, "extensionPurpose", purposeToString(purpose))

	shootState, _, err := extensions.GetShootStateForCluster(ctx, r.GardenClient, r.SeedClient, r.getClusterNameFromRequest(request))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithValues("shootState", client.ObjectKeyFromObject(shootState))

	patch := client.MergeFromWithOptions(shootState.DeepCopy(), client.MergeFromWithOptimisticLock{})
	currentState, currentResources := getShootStateExtensionStateAndResources(shootState, r.ObjectKind, &name, purpose)

	// Delete resources which are no longer present in the extension state from the ShootState.Spec.Resources list
	for _, resource := range currentResources {
		if v1beta1helper.GetResourceByName(newResources, resource.Name) == nil {
			updateShootStateResourceData(shootState, &resource.ResourceRef, nil)
		}
	}

	resourcesToUpdate, err := r.getResourcesToUpdate(ctx, shootState, extensionObj.GetNamespace(), newResources)
	if err != nil {
		return reconcile.Result{}, err
	}
	for _, resourceToUpdate := range resourcesToUpdate {
		updateShootStateResourceData(shootState, &resourceToUpdate.CrossVersionObjectReference, &resourceToUpdate.Data)
	}

	shouldUpdateExtensionData := !apiequality.Semantic.DeepEqual(newState, currentState) || !apiequality.Semantic.DeepEqual(newResources, currentResources)
	if shouldUpdateExtensionData {
		updateShootStateExtensionStateAndResources(shootState, r.ObjectKind, &name, purpose, newState, newResources)
	}

	if !shouldUpdateExtensionData && len(resourcesToUpdate) == 0 {
		log.Info("Skipping sync because state is already up-to-date")
		return reconcile.Result{}, nil
	}

	if err := r.GardenClient.Patch(ctx, shootState, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not sync extension state for shoot %q for extension %s/%s/%s: %w", client.ObjectKeyFromObject(shootState).String(), r.ObjectKind, name, purposeToString(purpose), err)
	}

	log.Info("Syncing state was successful")
	return reconcile.Result{}, nil
}

func (r *Reconciler) getResourcesToUpdate(ctx context.Context, shootState *gardencorev1beta1.ShootState, namespace string, newResources []gardencorev1beta1.NamedResourceReference) (v1beta1helper.ResourceDataList, error) {
	var resourcesToAddUpdate v1beta1helper.ResourceDataList

	for _, newResource := range newResources {
		obj, err := unstructuredutils.GetObjectByRef(ctx, r.SeedClient, &newResource.ResourceRef, namespace)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, fmt.Errorf("object not found %v", newResource.ResourceRef)
		}

		raw := &runtime.RawExtension{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, raw); err != nil {
			return nil, err
		}

		resourceList := v1beta1helper.ResourceDataList(shootState.Spec.Resources)
		currentResource := resourceList.Get(&newResource.ResourceRef)

		if currentResource == nil || !apiequality.Semantic.DeepEqual(currentResource.Data, newResource) {
			resourcesToAddUpdate = append(resourcesToAddUpdate, gardencorev1beta1.ResourceData{
				CrossVersionObjectReference: newResource.ResourceRef,
				Data:                        *raw,
			})
		}
	}

	return resourcesToAddUpdate, nil
}

func getShootStateExtensionStateAndResources(shootState *gardencorev1beta1.ShootState, kind string, name, purpose *string) (*runtime.RawExtension, []gardencorev1beta1.NamedResourceReference) {
	list := v1beta1helper.ExtensionResourceStateList(shootState.Spec.Extensions)
	s := list.Get(kind, name, purpose)
	if s != nil {
		return s.State, s.Resources
	}
	return nil, nil
}

func updateShootStateExtensionStateAndResources(shootState *gardencorev1beta1.ShootState, kind string, name, purpose *string, state *runtime.RawExtension, resources []gardencorev1beta1.NamedResourceReference) {
	list := v1beta1helper.ExtensionResourceStateList(shootState.Spec.Extensions)
	if state != nil || len(resources) > 0 {
		list.Upsert(&gardencorev1beta1.ExtensionResourceState{
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

func updateShootStateResourceData(shootState *gardencorev1beta1.ShootState, ref *autoscalingv1.CrossVersionObjectReference, data *runtime.RawExtension) {
	list := v1beta1helper.ResourceDataList(shootState.Spec.Resources)
	if data != nil {
		list.Upsert(&gardencorev1beta1.ResourceData{
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

func (r *Reconciler) getClusterNameFromRequest(req reconcile.Request) string {
	var clusterName string
	if req.Namespace == "" {
		// Handling for cluster-scoped backupentry extension resources.
		clusterName, _ = gardenerutils.ExtractShootDetailsFromBackupEntryName(req.Name)
	} else {
		clusterName = req.Namespace
	}

	return clusterName
}
