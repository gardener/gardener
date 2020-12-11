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
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

func (c *Controller) reconcileShootedSeedRegistrationKey(req reconcile.Request) (reconcile.Result, error) {
	// Get shoot from store
	shoot, err := c.shootLister.Shoots(req.Namespace).Get(req.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Logger.Debugf("[SHOOTED SEED REGISTRATION] Skipping Shoot %s because it has been deleted", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		logger.Logger.Errorf("[SHOOTED SEED REGISTRATION] Could not get Shoot %s from store: %v", req.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// Read the shooted seed from the "use-as-seed" annotation
	shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the shooted seed
	return c.seedRegistrationControl.Reconcile(shoot, shootedSeed)
}

// SeedRegistrationControlInterface implements the control logic for reconciling shooted seeds.
// It is implemented as an interface to allow for extensions that provide different semantics. Currently, there is only one
// implementation.
type SeedRegistrationControlInterface interface {
	Reconcile(shoot *gardencorev1beta1.Shoot, shootedSeed *gardencorev1beta1helper.ShootedSeed) (reconcile.Result, error)
}

// NewDefaultSeedRegistrationControl returns a new instance of the default implementation of the SeedRegistrationControlInterface that
// implements the documented semantics for registering shooted seeds. You should use an instance returned from
// NewDefaultSeedRegistrationControl() for any scenario other than testing.
func NewDefaultSeedRegistrationControl(clientMap clientmap.ClientMap, recorder record.EventRecorder) SeedRegistrationControlInterface {
	return &defaultSeedRegistrationControl{clientMap, recorder}
}

type defaultSeedRegistrationControl struct {
	clientMap clientmap.ClientMap
	recorder  record.EventRecorder
}

func (c *defaultSeedRegistrationControl) Reconcile(shoot *gardencorev1beta1.Shoot, shootedSeed *gardencorev1beta1helper.ShootedSeed) (reconcile.Result, error) {
	var (
		ctx         = context.TODO()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	isOwnedBy, err := isManagedSeedOwnedBy(ctx, gardenClient.Client(), shoot)
	if err != nil {
		message := fmt.Sprintf("Could not get ManagedSeed object for shoot %q: %+v", kutil.ObjectName(shoot), err)
		shootLogger.Errorf(message)
		c.recorder.Event(shoot, corev1.EventTypeWarning, "ManagedSeedGet", message)
		return reconcile.Result{}, err
	}
	if !isOwnedBy {
		logger.Logger.Infof("[SHOOTED SEED REGISTRATION] Skipping ManagedSeed object for shoot %s because it's not owned by this shoot", kutil.ObjectName(shoot))
		return reconcile.Result{}, nil
	}

	if shoot.DeletionTimestamp == nil && shootedSeed != nil {
		shootLogger.Infof("[SHOOTED SEED REGISTRATION] Creating or updating ManagedSeed object for shoot %s", kutil.ObjectName(shoot))
		if err := createOrUpdateManagedSeed(ctx, gardenClient.Client(), shoot, shootedSeed); err != nil {
			message := fmt.Sprintf("Could not create or update ManagedSeed object for shoot %q: %+v", kutil.ObjectName(shoot), err)
			shootLogger.Errorf(message)
			c.recorder.Event(shoot, corev1.EventTypeWarning, "ManagedSeedCreationOrUpdate", message)
			return reconcile.Result{}, err
		}
	} else {
		shootLogger.Infof("[SHOOTED SEED REGISTRATION] Deleting ManagedSeed object for shoot %s", kutil.ObjectName(shoot))
		if err := deleteManagedSeed(ctx, gardenClient.Client(), shoot); err != nil {
			message := fmt.Sprintf("Could not delete ManagedSeed object for shoot %q: %+v", kutil.ObjectName(shoot), err)
			shootLogger.Errorf(message)
			c.recorder.Event(shoot, corev1.EventTypeWarning, "ManagedSeedDeletion", message)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func isManagedSeedOwnedBy(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) (bool, error) {
	// Get managed seed
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := c.Get(ctx, kutil.Key(shoot.Namespace, shoot.Name), managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	// Check if managed seed is controlled by shoot
	return metav1.IsControlledBy(managedSeed, shoot), nil
}

func createOrUpdateManagedSeed(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, shootedSeed *gardencorev1beta1helper.ShootedSeed) error {
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
	_, err = controllerutil.CreateOrUpdate(ctx, c, managedSeed, func() error {
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
		seed      *seedmanagementv1alpha1.SeedTemplateSpec
		gardenlet *seedmanagementv1alpha1.Gardenlet
	)

	// Initialize seed spec
	seedSpec, err := getSeedSpec(shoot, shootedSeed)
	if err != nil {
		return nil, err
	}

	if shootedSeed.NoGardenlet {
		// Initialize seed template
		seed = &seedmanagementv1alpha1.SeedTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: shoot.Labels,
			},
			Spec: *seedSpec,
		}
	} else {
		// Initialize gardenlet config resources
		var resources *configv1alpha1.ResourcesConfiguration
		if shootedSeed.Resources != nil {
			resources = &configv1alpha1.ResourcesConfiguration{
				Capacity: shootedSeed.Resources.Capacity,
				Reserved: shootedSeed.Resources.Reserved,
			}
		}

		// Initialize the garden connection bootstrap mechanism
		gardenConnectionBootstrap := seedmanagementv1alpha1.GardenConnectionBootstrapToken
		if shootedSeed.UseServiceAccountBootstrapping {
			gardenConnectionBootstrap = seedmanagementv1alpha1.GardenConnectionBootstrapServiceAccount
		}

		// Initialize the seed connection mechanism
		seedConnection := seedmanagementv1alpha1.SeedConnectionServiceAccount

		// Initialize gardenlet configuraton and parameters
		gardenlet = &seedmanagementv1alpha1.Gardenlet{
			Config: &configv1alpha1.GardenletConfiguration{
				Resources:    resources,
				FeatureGates: shootedSeed.FeatureGates,
				SeedConfig: &configv1alpha1.SeedConfig{
					Seed: gardencorev1beta1.Seed{
						ObjectMeta: metav1.ObjectMeta{
							Labels: shoot.Labels,
						},
						Spec: *seedSpec,
					},
				},
			},
			GardenConnectionBootstrap: &gardenConnectionBootstrap,
			// For backward compatibility with the use-as-seed annotation this should rather be nil,
			// since the seed registration controller implementation always inherits the seed connection settings
			// from the parent gardenlet. However, I believe this really only works with "service account" anyway,
			// and always fails with local dev setups since there is a wrong kubeconfig set in the parent.
			SeedConnection:    &seedConnection,
			MergeParentConfig: true,
		}
	}

	// Initalize shoot APIServer parameters
	var apiServer *seedmanagementv1alpha1.APIServer
	if shootedSeed.APIServer != nil {
		// Initalize shoot APIServer autoscaler parameters
		var autoscaler *seedmanagementv1alpha1.APIServerAutoscaler
		if shootedSeed.APIServer.Autoscaler != nil {
			autoscaler = &seedmanagementv1alpha1.APIServerAutoscaler{
				MinReplicas: shootedSeed.APIServer.Autoscaler.MinReplicas,
				MaxReplicas: pointer.Int32Ptr(shootedSeed.APIServer.Autoscaler.MaxReplicas),
			}
		}

		apiServer = &seedmanagementv1alpha1.APIServer{
			Replicas:   shootedSeed.APIServer.Replicas,
			Autoscaler: autoscaler,
		}
	}

	// Return result
	return &seedmanagementv1alpha1.ManagedSeedSpec{
		Shoot: &seedmanagementv1alpha1.Shoot{
			Name:      shoot.Name,
			APIServer: apiServer,
		},
		Seed:      seed,
		Gardenlet: gardenlet,
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
		},
	}, nil
}
