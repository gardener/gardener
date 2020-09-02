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

package controllerinstallation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener-resource-manager/pkg/manager"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const installationTypeHelm = "helm"

func (c *Controller) controllerInstallationAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.controllerInstallationQueue.Add(key)
}

func (c *Controller) controllerInstallationUpdate(oldObj, newObj interface{}) {
	old, ok1 := oldObj.(*gardencorev1beta1.ControllerInstallation)
	new, ok2 := newObj.(*gardencorev1beta1.ControllerInstallation)
	if !ok1 || !ok2 {
		return
	}

	if new.DeletionTimestamp == nil && old.Spec.RegistrationRef.ResourceVersion == new.Spec.RegistrationRef.ResourceVersion && old.Spec.SeedRef.ResourceVersion == new.Spec.SeedRef.ResourceVersion {
		return
	}

	c.controllerInstallationAdd(newObj)
}

func (c *Controller) controllerInstallationDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.controllerInstallationQueue.Add(key)
}

func (c *Controller) reconcileControllerInstallationKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	controllerInstallation, err := c.controllerInstallationLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[CONTROLLERINSTALLATION RECONCILE] %s - skipping because ControllerInstallation has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[CONTROLLERINSTALLATION RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.controllerInstallationControl.Reconcile(controllerInstallation)
}

// ControlInterface implements the control logic for updating ControllerInstallations. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	Reconcile(*gardencorev1beta1.ControllerInstallation) error
}

// NewDefaultControllerInstallationControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for ControllerInstallations. You should use an instance returned from
// NewDefaultControllerInstallationControl() for any scenario other than testing.
func NewDefaultControllerInstallationControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, recorder record.EventRecorder, config *config.GardenletConfiguration, seedLister gardencorelisters.SeedLister, controllerRegistrationLister gardencorelisters.ControllerRegistrationLister, controllerInstallationLister gardencorelisters.ControllerInstallationLister, gardenNamespace *corev1.Namespace) ControlInterface {
	return &defaultControllerInstallationControl{clientMap, k8sGardenCoreInformers, recorder, config, seedLister, controllerRegistrationLister, controllerInstallationLister, gardenNamespace}
}

type defaultControllerInstallationControl struct {
	clientMap                    clientmap.ClientMap
	k8sGardenCoreInformers       gardencoreinformers.SharedInformerFactory
	recorder                     record.EventRecorder
	config                       *config.GardenletConfiguration
	seedLister                   gardencorelisters.SeedLister
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
	gardenNamespace              *corev1.Namespace
}

func (c *defaultControllerInstallationControl) Reconcile(obj *gardencorev1beta1.ControllerInstallation) error {
	var (
		controllerInstallation = obj.DeepCopy()
		logger                 = logger.NewFieldLogger(logger.Logger, "controllerinstallation", controllerInstallation.Name)
	)

	if isResponsible, err := c.isResponsible(controllerInstallation); !isResponsible || err != nil {
		return err
	}

	if controllerInstallation.DeletionTimestamp != nil {
		return c.delete(controllerInstallation, logger)
	}
	return c.reconcile(controllerInstallation, logger)
}

func (c *defaultControllerInstallationControl) reconcile(controllerInstallation *gardencorev1beta1.ControllerInstallation, logger logrus.FieldLogger) error {
	ctx := context.TODO()
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	if err := controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), controllerInstallation, FinalizerName); err != nil {
		return err
	}

	var (
		conditionValid     = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationValid)
		conditionInstalled = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationInstalled)
	)

	defer func() {
		if _, err := updateConditions(ctx, gardenClient, controllerInstallation, conditionValid, conditionInstalled); err != nil {
			logger.Errorf("Failed to update the conditions : %+v", err)
		}
	}()

	controllerRegistration, err := c.controllerRegistrationLister.Get(controllerInstallation.Spec.RegistrationRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "RegistrationNotFound", fmt.Sprintf("Referenced ControllerRegistration does not exist: %+v", err))
		} else {
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionUnknown, "RegistrationReadError", fmt.Sprintf("Referenced ControllerRegistration cannot be read: %+v", err))
		}
		return err
	}

	seed, err := c.seedLister.Get(controllerInstallation.Spec.SeedRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "SeedNotFound", fmt.Sprintf("Referenced Seed does not exist: %+v", err))
		} else {
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionUnknown, "SeedReadError", fmt.Sprintf("Referenced Seed cannot be read: %+v", err))
		}
		return err
	}

	seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeedWithName(seed.Name))
	if err != nil {
		conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionUnknown, "SeedReadError", fmt.Sprintf("Failed to get Seed client for referenced Seed: %+v", err))
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	var helmDeployment HelmDeployment
	if err := json.Unmarshal(controllerRegistration.Spec.Deployment.ProviderConfig.Raw, &helmDeployment); err != nil {
		conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "ChartInformationInvalid", fmt.Sprintf("Chart Information cannot be unmarshalled: %+v", err))
		return err
	}

	namespace := getNamespaceForControllerInstallation(controllerInstallation)
	if _, err := controllerutil.CreateOrUpdate(ctx, seedClient.Client(), namespace, func() error {
		kutil.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleExtension)
		kutil.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelControllerRegistrationName, controllerRegistration.Name)
		return nil
	}); err != nil {
		return err
	}

	var (
		volumeProvider  string
		volumeProviders []gardencorev1beta1.SeedVolumeProvider
	)

	if seed.Spec.Volume != nil {
		volumeProviders = seed.Spec.Volume.Providers
		if len(seed.Spec.Volume.Providers) > 0 {
			volumeProvider = seed.Spec.Volume.Providers[0].Name
		}
	}

	// Mix-in some standard values for garden and seed.
	gardenerValues := map[string]interface{}{
		"gardener": map[string]interface{}{
			"garden": map[string]interface{}{
				"identity": c.gardenNamespace.UID,
			},
			"seed": map[string]interface{}{
				"identity":        seed.Name,
				"annotations":     seed.Annotations,
				"labels":          seed.Labels,
				"provider":        seed.Spec.Provider.Type,
				"region":          seed.Spec.Provider.Region,
				"volumeProvider":  volumeProvider,
				"volumeProviders": volumeProviders,
				"ingressDomain":   seed.Spec.DNS.IngressDomain,
				"protected":       gardencorev1beta1helper.TaintsHave(seed.Spec.Taints, gardencorev1beta1.SeedTaintProtected),
				"visible":         seed.Spec.Settings.Scheduling.Visible,
				"taints":          seed.Spec.Taints,
				"networks":        seed.Spec.Networks,
				"blockCIDRs":      seed.Spec.Networks.BlockCIDRs,
				"spec":            seed.Spec,
			},
		},
	}

	release, err := seedClient.ChartRenderer().RenderArchive(helmDeployment.Chart, controllerRegistration.Name, namespace.Name, utils.MergeMaps(helmDeployment.Values, gardenerValues))
	if err != nil {
		conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "ChartCannotBeRendered", fmt.Sprintf("Chart rendering process failed: %+v", err))
		return err
	}
	conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionTrue, "RegistrationValid", "Chart could be rendered successfully.")

	// Create secret
	data := release.AsSecretData()

	var secretName = controllerInstallation.Name
	if err := manager.
		NewSecret(seedClient.Client()).
		WithNamespacedName(v1beta1constants.GardenNamespace, secretName).
		WithKeyValues(data).
		Reconcile(ctx); err != nil {
		conditionInstalled = gardencorev1beta1helper.UpdatedCondition(conditionInstalled, gardencorev1beta1.ConditionFalse, "InstallationFailed", fmt.Sprintf("Creation of ManagedResource secret %q failed: %+v", secretName, err))
		return err
	}

	if err := manager.
		NewManagedResource(seedClient.Client()).
		WithNamespacedName(v1beta1constants.GardenNamespace, controllerInstallation.Name).
		WithSecretRef(secretName).
		WithClass(v1beta1constants.SeedResourceManagerClass).
		Reconcile(ctx); err != nil {
		conditionInstalled = gardencorev1beta1helper.UpdatedCondition(conditionInstalled, gardencorev1beta1.ConditionFalse, "InstallationFailed", fmt.Sprintf("Creation of ManagedResource %q failed: %+v", controllerInstallation.Name, err))
		return err
	}

	if conditionInstalled.Status == gardencorev1beta1.ConditionUnknown {
		// initially set condition to Pending
		// care controller will update condition based on 'ResourcesApplied' condition of ManagedResource
		conditionInstalled = gardencorev1beta1helper.UpdatedCondition(conditionInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending", fmt.Sprintf("Installation of ManagedResource %q is still pending.", controllerInstallation.Name))
	}

	return nil
}

func (c *defaultControllerInstallationControl) delete(controllerInstallation *gardencorev1beta1.ControllerInstallation, logger logrus.FieldLogger) error {
	var (
		ctx                = context.TODO()
		newConditions      = gardencorev1beta1helper.MergeConditions(controllerInstallation.Status.Conditions, gardencorev1beta1helper.InitCondition(gardencorev1beta1.ControllerInstallationValid), gardencorev1beta1helper.InitCondition(gardencorev1beta1.ControllerInstallationInstalled))
		conditionValid     = newConditions[0]
		conditionInstalled = newConditions[1]
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	defer func() {
		if _, err := updateConditions(ctx, gardenClient, controllerInstallation, conditionValid, conditionInstalled); client.IgnoreNotFound(err) != nil {
			logger.Errorf("Failed to update the conditions when trying to delete: %+v", err)
		}
	}()

	seed, err := c.seedLister.Get(controllerInstallation.Spec.SeedRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "SeedNotFound", fmt.Sprintf("Referenced Seed does not exist: %+v", err))
		} else {
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionUnknown, "SeedReadError", fmt.Sprintf("Referenced Seed cannot be read: %+v", err))
		}
		return err
	}

	seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeedWithName(seed.Name))
	if err != nil {
		conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionUnknown, "SeedReadError", fmt.Sprintf("Failed to get Seed client for referenced Seed: %+v", err))
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	mr := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallation.Name,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	err = seedClient.Client().Delete(ctx, mr)
	if err == nil {
		message := fmt.Sprintf("Deletion of ManagedResource %q is still pending.", controllerInstallation.Name)
		conditionInstalled = gardencorev1beta1helper.UpdatedCondition(conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionPending", message)
		return errors.New(message)
	} else if !apierrors.IsNotFound(err) {
		conditionInstalled = gardencorev1beta1helper.UpdatedCondition(conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of ManagedResource %q failed: %+v", controllerInstallation.Name, err))
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallation.Name,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	if err := seedClient.Client().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		conditionInstalled = gardencorev1beta1helper.UpdatedCondition(conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of ManagedResource secret %q failed: %+v", controllerInstallation.Name, err))
	}

	if err := seedClient.Client().Delete(ctx, getNamespaceForControllerInstallation(controllerInstallation)); client.IgnoreNotFound(err) != nil {
		return err
	}
	conditionInstalled = gardencorev1beta1helper.UpdatedCondition(conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionSuccessful", "Deletion of old resources succeeded.")

	return controllerutils.RemoveFinalizer(ctx, gardenClient.DirectClient(), controllerInstallation.DeepCopy(), FinalizerName)
}

func updateConditions(ctx context.Context, gardenClient kubernetes.Interface, controllerInstallation *gardencorev1beta1.ControllerInstallation, conditions ...gardencorev1beta1.Condition) (*gardencorev1beta1.ControllerInstallation, error) {
	return kutil.TryUpdateControllerInstallationStatusWithEqualFunc(ctx, gardenClient.GardenCore(), retry.DefaultBackoff, controllerInstallation.ObjectMeta,
		func(controllerInstallation *gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error) {
			controllerInstallation.Status.Conditions = gardencorev1beta1helper.MergeConditions(controllerInstallation.Status.Conditions, conditions...)
			return controllerInstallation, nil
		}, func(cur, updated *gardencorev1beta1.ControllerInstallation) bool {
			return equality.Semantic.DeepEqual(cur.Status.Conditions, updated.Status.Conditions)
		},
	)
}

func (c *defaultControllerInstallationControl) isResponsible(controllerInstallation *gardencorev1beta1.ControllerInstallation) (bool, error) {
	controllerRegistration, err := c.controllerRegistrationLister.Get(controllerInstallation.Spec.RegistrationRef.Name)
	if err != nil {
		return false, err
	}

	if deployment := controllerRegistration.Spec.Deployment; deployment != nil {
		return deployment.Type == installationTypeHelm, nil
	}
	return false, nil
}

func getNamespaceForControllerInstallation(controllerInstallation *gardencorev1beta1.ControllerInstallation) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("extension-%s", controllerInstallation.Name),
		},
	}
}
