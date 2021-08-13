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

package shoot

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) seedRegistrationAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	namespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return
	}
	if namespace != v1beta1constants.GardenNamespace {
		return
	}

	c.seedRegistrationQueue.Add(key)
}

func (c *Controller) seedRegistrationUpdate(oldObj, newObj interface{}) {
	oldShoot, ok := oldObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}
	newShoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	// Reconcile only if either of the following is true:
	// * The use-as-seed annotation changed
	// * The generation was updated and the use-as-seed annotation is present
	useAsSeedAnnotationChanged := newShoot.Annotations[v1beta1constants.AnnotationShootUseAsSeed] != oldShoot.Annotations[v1beta1constants.AnnotationShootUseAsSeed]
	genChangedAndUseAsSeedAnnotationPresent := newShoot.Generation != newShoot.Status.ObservedGeneration && newShoot.Annotations[v1beta1constants.AnnotationShootUseAsSeed] != ""
	if !useAsSeedAnnotationChanged && !genChangedAndUseAsSeedAnnotationPresent {
		return
	}

	c.seedRegistrationAdd(newObj)
}

// NewSeedRegistrationReconciler creates a new reconciler that registers ManagedSeeds for Shoots with the use-as-seed
// annotation.
// It should be considered deprecated and will be removed in a future version.
func NewSeedRegistrationReconciler(clientMap clientmap.ClientMap, recorder record.EventRecorder, logger logrus.FieldLogger) reconcile.Reconciler {
	return &seedRegistrationReconciler{
		clientMap: clientMap,
		recorder:  recorder,
		logger:    logger,
	}
}

type seedRegistrationReconciler struct {
	clientMap clientmap.ClientMap
	recorder  record.EventRecorder
	logger    logrus.FieldLogger
}

func (r *seedRegistrationReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := r.logger.WithField("shoot", request.String())

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debugf("[SHOOTED SEED REGISTRATION] Skipping Shoot because it has been deleted")
			return reconcile.Result{}, nil
		}
		log.Errorf("[SHOOTED SEED REGISTRATION] Could not get Shoot from store: %+v", err)
		return reconcile.Result{}, err
	}

	// Read the shooted seed from the "use-as-seed" annotation
	shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the shooted seed
	return r.reconcile(ctx, gardenClient.Client(), shoot, shootedSeed, log)
}

func (r *seedRegistrationReconciler) reconcile(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot, shootedSeed *gardencorev1beta1helper.ShootedSeed, log logrus.FieldLogger) (reconcile.Result, error) {
	exists, isOwnedBy, err := isManagedSeedOwnedBy(ctx, gardenClient, shoot)
	if err != nil {
		message := fmt.Sprintf("Could not get ManagedSeed for shoot: %+v", err)
		log.Errorf(message)
		r.recorder.Event(shoot, corev1.EventTypeWarning, "ManagedSeedGet", message)
		return reconcile.Result{}, err
	}
	if exists && !isOwnedBy {
		log.Infof("[SHOOTED SEED REGISTRATION] Skipping ManagedSeed object update or deletion for shoot because it's not owned by this shoot")
		return reconcile.Result{}, nil
	}

	if shoot.DeletionTimestamp == nil && shootedSeed != nil {
		log.Infof("[SHOOTED SEED REGISTRATION] Creating or updating ManagedSeed object for shoot")
		if err := reconcileManagedSeed(ctx, gardenClient, shoot, shootedSeed); err != nil {
			message := fmt.Sprintf("Could not create or update ManagedSeed object for shoot: %+v", err)
			log.Errorf(message)
			r.recorder.Event(shoot, corev1.EventTypeWarning, "ManagedSeedCreationOrUpdate", message)
			return reconcile.Result{}, err
		}
	} else if exists {
		log.Infof("[SHOOTED SEED REGISTRATION] Deleting ManagedSeed object for shoot")
		if err := deleteManagedSeed(ctx, gardenClient, shoot); err != nil {
			message := fmt.Sprintf("Could not delete ManagedSeed object for shoot: %+v", err)
			log.Errorf(message)
			r.recorder.Event(shoot, corev1.EventTypeWarning, "ManagedSeedDeletion", message)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func isManagedSeedOwnedBy(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) (bool, bool, error) {
	// Get managed seed
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(shoot), managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}

	// Check if managed seed is controlled by shoot
	return true, metav1.IsControlledBy(managedSeed, shoot), nil
}

func reconcileManagedSeed(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, shootedSeed *gardencorev1beta1helper.ShootedSeed) error {
	// Prepare managed seed spec
	managedSeedSpec, err := getManagedSeedSpec(shoot, shootedSeed)
	if err != nil {
		return err
	}

	// Create or update managed seed
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
		},
	}
	_, err = controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c, managedSeed, func() error {
		managedSeed.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		managedSeed.Spec = *managedSeedSpec
		return nil
	})
	return err
}

func deleteManagedSeed(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) error {
	// Delete managed seed
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
		},
	}
	if err := c.Delete(ctx, managedSeed); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

func getManagedSeedSpec(shoot *gardencorev1beta1.Shoot, shootedSeed *gardencorev1beta1helper.ShootedSeed) (*seedmanagementv1alpha1.ManagedSeedSpec, error) {
	var (
		seedTemplate *gardencorev1beta1.SeedTemplate
		gardenlet    *seedmanagementv1alpha1.Gardenlet
	)

	// Initialize seed spec
	seedSpec, err := getSeedSpec(shoot, shootedSeed)
	if err != nil {
		return nil, err
	}

	if shootedSeed.NoGardenlet {
		// Initialize seed template
		seedTemplate = &gardencorev1beta1.SeedTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Labels: shoot.Labels,
			},
			Spec: *seedSpec,
		}
	} else {
		// Initialize gardenlet config
		var resources *configv1alpha1.ResourcesConfiguration
		if shootedSeed.Resources != nil {
			resources = &configv1alpha1.ResourcesConfiguration{
				Capacity: shootedSeed.Resources.Capacity,
				Reserved: shootedSeed.Resources.Reserved,
			}
		}
		config := &configv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			Resources:    resources,
			FeatureGates: shootedSeed.FeatureGates,
			SeedConfig: &configv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: shoot.Labels,
					},
					Spec: *seedSpec,
				},
			},
		}

		// Encode gardenlet config to raw extension
		re, err := encoding.EncodeGardenletConfiguration(config)
		if err != nil {
			return nil, err
		}

		// Initialize the garden connection bootstrap mechanism
		bootstrap := seedmanagementv1alpha1.BootstrapToken
		if shootedSeed.UseServiceAccountBootstrapping {
			bootstrap = seedmanagementv1alpha1.BootstrapServiceAccount
		}

		// Initialize gardenlet configuraton and parameters
		gardenlet = &seedmanagementv1alpha1.Gardenlet{
			Config:          *re,
			Bootstrap:       &bootstrap,
			MergeWithParent: pointer.Bool(true),
		}
	}

	// Return result
	return &seedmanagementv1alpha1.ManagedSeedSpec{
		Shoot: &seedmanagementv1alpha1.Shoot{
			Name: shoot.Name,
		},
		SeedTemplate: seedTemplate,
		Gardenlet:    gardenlet,
	}, nil
}

func getSeedSpec(shoot *gardencorev1beta1.Shoot, shootedSeed *gardencorev1beta1helper.ShootedSeed) (*gardencorev1beta1.SeedSpec, error) {
	// Initialize secret reference
	var secretRef *corev1.SecretReference
	if shootedSeed.NoGardenlet || shootedSeed.WithSecretRef {
		secretRef = &corev1.SecretReference{
			Name:      fmt.Sprintf("seed-%s", shoot.Name),
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	// Initialize taints
	var taints []gardencorev1beta1.SeedTaint
	if shootedSeed.Protected != nil && *shootedSeed.Protected {
		taints = append(taints, gardencorev1beta1.SeedTaint{Key: gardencorev1beta1.SeedTaintProtected})
	}

	// Initialize volume
	var volume *gardencorev1beta1.SeedVolume
	if shootedSeed.MinimumVolumeSize != nil {
		minimumSize, err := resource.ParseQuantity(*shootedSeed.MinimumVolumeSize)
		if err != nil {
			return nil, err
		}
		volume = &gardencorev1beta1.SeedVolume{
			MinimumSize: &minimumSize,
		}
	}

	// Initialize settings
	var loadBalancerServices *gardencorev1beta1.SeedSettingLoadBalancerServices
	if shootedSeed.LoadBalancerServicesAnnotations != nil {
		loadBalancerServices = &gardencorev1beta1.SeedSettingLoadBalancerServices{
			Annotations: shootedSeed.LoadBalancerServicesAnnotations,
		}
	}

	// Initialize ingress
	var ingress *gardencorev1beta1.Ingress
	if shootedSeed.IngressController != nil {
		ingress = &gardencorev1beta1.Ingress{
			Controller: *shootedSeed.IngressController,
		}
	}

	// Return result
	return &gardencorev1beta1.SeedSpec{
		Backup: shootedSeed.Backup,
		Networks: gardencorev1beta1.SeedNetworks{
			BlockCIDRs:    shootedSeed.BlockCIDRs,
			ShootDefaults: shootedSeed.ShootDefaults,
		},
		Provider: gardencorev1beta1.SeedProvider{
			ProviderConfig: shootedSeed.SeedProviderConfig,
		},
		SecretRef: secretRef,
		Taints:    taints,
		Volume:    volume,
		Settings: &gardencorev1beta1.SeedSettings{
			ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
				Enabled: shootedSeed.DisableCapacityReservation == nil || !*shootedSeed.DisableCapacityReservation,
			},
			Scheduling: &gardencorev1beta1.SeedSettingScheduling{
				Visible: shootedSeed.Visible == nil || *shootedSeed.Visible,
			},
			ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
				Enabled: shootedSeed.DisableDNS == nil || !*shootedSeed.DisableDNS,
			},
			LoadBalancerServices: loadBalancerServices,
		},
		Ingress: ingress,
	}, nil
}
