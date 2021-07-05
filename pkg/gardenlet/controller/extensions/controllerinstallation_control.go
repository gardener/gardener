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

package extensions

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type controllerInstallationControl struct {
	k8sGardenClient kubernetes.Interface
	seedClient      kubernetes.Interface
	seedName        string
	log             *logrus.Logger

	controllerInstallationQueue workqueue.RateLimitingInterface
	lock                        *sync.RWMutex
	kindToRequiredTypes         map[string]sets.String
}

// createExtensionRequiredReconcileFunc returns a function for the given extension kind that lists all existing
// extension resources of the given kind and stores the respective types in the `kindToRequiredTypes` map. Afterwards,
// it enqueue all ControllerInstallations for the seed that are referring to ControllerRegistrations responsible for
// the given kind.
// The returned reconciler doesn't care about which object was created/updated/deleted, it just cares about being
// triggered when some object of the kind, it is responsible for, is created/updated/deleted.
func (c *controllerInstallationControl) createExtensionRequiredReconcileFunc(kind string, newListObjFunc func() client.ObjectList) reconcile.Func {
	return func(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
		listObj := newListObjFunc()
		if err := c.seedClient.Client().List(ctx, listObj); err != nil {
			return reconcile.Result{}, err
		}

		c.lock.RLock()
		oldRequiredTypes, kindCalculated := c.kindToRequiredTypes[kind]
		c.lock.RUnlock()
		newRequiredTypes := sets.NewString()

		if err := meta.EachListItem(listObj, func(o runtime.Object) error {
			dnsProvider, ok := o.(*dnsv1alpha1.DNSProvider)
			if ok {
				newRequiredTypes.Insert(dnsProvider.Spec.Type)
				return nil
			}

			obj, err := extensions.Accessor(o)
			if err != nil {
				return err
			}

			newRequiredTypes.Insert(obj.GetExtensionSpec().GetExtensionType())
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}

		// if there is no difference compared to before then exit early
		if kindCalculated && oldRequiredTypes.Equal(newRequiredTypes) {
			return reconcile.Result{}, nil
		}

		c.lock.Lock()
		c.kindToRequiredTypes[kind] = newRequiredTypes
		c.lock.Unlock()

		// Step 2: List all existing controller registrations and filter for those that are supporting resources for the
		// extension kind this particular reconciler is responsible for.

		controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
		if err := c.k8sGardenClient.Client().List(ctx, controllerRegistrationList); err != nil {
			return reconcile.Result{}, err
		}

		controllerRegistrationNamesForKind := sets.NewString()
		for _, controllerRegistration := range controllerRegistrationList.Items {
			for _, resource := range controllerRegistration.Spec.Resources {
				if resource.Kind == kind {
					controllerRegistrationNamesForKind.Insert(controllerRegistration.Name)
					break
				}
			}
		}

		// Step 3: List all existing controller installation objects for the seed cluster this controller is responsible
		// for and filter for those that reference registrations collected above. Then requeue those installations for
		// the other reconciler to decide whether it is required or not.

		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		if err := c.k8sGardenClient.Client().List(ctx, controllerInstallationList); err != nil {
			return reconcile.Result{}, err
		}

		for _, obj := range controllerInstallationList.Items {
			if obj.Spec.SeedRef.Name != c.seedName {
				continue
			}

			if !controllerRegistrationNamesForKind.Has(obj.Spec.RegistrationRef.Name) {
				continue
			}

			c.controllerInstallationQueue.Add(obj.Name)
		}

		return reconcile.Result{}, nil
	}
}

// reconcileControllerInstallationRequired reconciles ControllerInstallations. It checks whether it is still
// required by using the <kindToRequiredTypes> map that was computed by a separate reconciler.
func (c *controllerInstallationControl) reconcileControllerInstallationRequired(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	if err := c.k8sGardenClient.Client().Get(ctx, req.NamespacedName, controllerInstallation); err != nil {
		return reconcile.Result{}, err
	}

	controllerRegistration := &gardencorev1beta1.ControllerRegistration{}
	if err := c.k8sGardenClient.Client().Get(ctx, kutil.Key(controllerInstallation.Spec.RegistrationRef.Name), controllerRegistration); err != nil {
		return reconcile.Result{}, err
	}

	var (
		allKindsCalculated = true
		required           *bool
		requiredKindTypes  = sets.NewString()
		message            string
	)

	c.lock.RLock()
	for _, resource := range controllerRegistration.Spec.Resources {
		requiredTypes, ok := c.kindToRequiredTypes[resource.Kind]
		if !ok {
			allKindsCalculated = false
			continue
		}

		if requiredTypes.Has(resource.Type) {
			required = pointer.Bool(true)
			requiredKindTypes.Insert(fmt.Sprintf("%s/%s", resource.Kind, resource.Type))
		}
	}
	c.lock.RUnlock()

	if required == nil {
		if !allKindsCalculated {
			// if required wasn't set yet then but not all kinds were calculated then the it's not possible to
			// decide yet whether it's required or not
			return reconcile.Result{}, nil
		}

		// if required wasn't set yet then but all kinds were calculated then the installation is no longer required
		required = pointer.Bool(false)
		message = "no extension objects exist in the seed having the kind/type combinations the controller is responsible for"
	} else if *required {
		message = fmt.Sprintf("extension objects still exist in the seed: %+v", requiredKindTypes.UnsortedList())
	}

	if err := updateControllerInstallationRequiredCondition(ctx, c.k8sGardenClient.Client(), controllerInstallation, *required, message); err != nil {
		c.log.Error(err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func updateControllerInstallationRequiredCondition(ctx context.Context, c client.StatusClient, controllerInstallation *gardencorev1beta1.ControllerInstallation, required bool, message string) error {
	var (
		conditionRequired = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationRequired)

		status = gardencorev1beta1.ConditionTrue
		reason = "ExtensionObjectsExist"
	)

	if !required {
		status = gardencorev1beta1.ConditionFalse
		reason = "NoExtensionObjects"
	}

	patch := client.StrategicMergeFrom(controllerInstallation.DeepCopy())
	controllerInstallation.Status.Conditions = gardencorev1beta1helper.MergeConditions(
		controllerInstallation.Status.Conditions,
		gardencorev1beta1helper.UpdatedCondition(conditionRequired, status, reason, message),
	)

	return c.Status().Patch(ctx, controllerInstallation, patch)
}
