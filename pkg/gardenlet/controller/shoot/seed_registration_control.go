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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
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

	if newShoot.Generation == newShoot.Status.ObservedGeneration && apiequality.Semantic.DeepEqual(newShoot.Annotations, oldShoot.Annotations) {
		return
	}

	c.seedRegistrationAdd(newObj)
}

func (c *Controller) reconcileShootedSeedRegistrationKey(req reconcile.Request) (reconcile.Result, error) {
	shoot, err := c.shootLister.Shoots(req.Namespace).Get(req.Name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOTED SEED REGISTRATION] %s/%s - skipping because Shoot has been deleted", req.Namespace, req.Name)
		return reconcile.Result{}, nil
	}
	if err != nil {
		logger.Logger.Errorf("[SHOOTED SEED REGISTRATION] %s/%s - unable to retrieve object from store: %v", req.Namespace, req.Name, err)
		return reconcile.Result{}, err
	}

	shootedSeedConfig, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	return c.seedRegistrationControl.Reconcile(shoot, shootedSeedConfig)
}

// SeedRegistrationControlInterface implements the control logic for requeuing shooted Seeds after extensions have been updated.
// It is implemented as an interface to allow for extensions that provide different semantics. Currently, there is only one
// implementation.
type SeedRegistrationControlInterface interface {
	Reconcile(shootObj *gardencorev1beta1.Shoot, shootedSeedConfig *gardencorev1beta1helper.ShootedSeed) (reconcile.Result, error)
}

// NewDefaultSeedRegistrationControl returns a new instance of the default implementation SeedRegistrationControlInterface that
// implements the documented semantics for registering shooted seeds. You should use an instance returned from
// NewDefaultSeedRegistrationControl() for any scenario other than testing.
func NewDefaultSeedRegistrationControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.Interface, imageVector imagevector.ImageVector, config *config.GardenletConfiguration, recorder record.EventRecorder) SeedRegistrationControlInterface {
	return &defaultSeedRegistrationControl{clientMap, k8sGardenCoreInformers, imageVector, config, recorder}
}

type defaultSeedRegistrationControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.Interface
	imageVector            imagevector.ImageVector
	config                 *config.GardenletConfiguration
	recorder               record.EventRecorder
}

func (c *defaultSeedRegistrationControl) Reconcile(shootObj *gardencorev1beta1.Shoot, shootedSeedConfig *gardencorev1beta1helper.ShootedSeed) (reconcile.Result, error) {
	var (
		ctx         = context.TODO()
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	if shoot.DeletionTimestamp == nil && shootedSeedConfig != nil {
		if shoot.Generation != shoot.Status.ObservedGeneration || shoot.Status.LastOperation == nil || shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
			shootLogger.Infof("[SHOOTED SEED REGISTRATION] Waiting for shoot %s to be reconciled before registering it as seed", shoot.Name)
			return reconcile.Result{
				RequeueAfter: 10 * time.Second,
			}, nil
		}

		seedClient, err := c.clientMap.GetClient(ctx, keys.ForSeedWithName(*shootObj.Spec.SeedName))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
		}

		if shootedSeedConfig.NoGardenlet {
			shootLogger.Infof("[SHOOTED SEED REGISTRATION] Registering %s as seed as configuration says that no gardenlet is desired", shoot.Name)
			if err := registerAsSeed(ctx, gardenClient.Client(), seedClient.Client(), shoot, shootedSeedConfig); err != nil {
				message := fmt.Sprintf("Could not register shoot %q as seed: %+v", shoot.Name, err)
				shootLogger.Errorf(message)
				c.recorder.Event(shoot, corev1.EventTypeWarning, "SeedRegistration", message)
				return reconcile.Result{}, err
			}
		} else {
			shootedSeedClient, err := c.clientMap.GetClient(ctx, keys.ForShoot(shoot))
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to get shooted seed client: %w", err)
			}

			shootLogger.Infof("[SHOOTED SEED REGISTRATION] Deploying gardenlet into %s which will register shoot as seed", shoot.Name)
			if err := deployGardenlet(ctx, gardenClient, seedClient, shootedSeedClient, shoot, shootedSeedConfig, c.imageVector, c.config); err != nil {
				message := fmt.Sprintf("Could not deploy Gardenlet into shoot %q: %+v", shoot.Name, err)
				shootLogger.Errorf(message)
				c.recorder.Event(shoot, corev1.EventTypeWarning, "GardenletDeployment", message)
				return reconcile.Result{}, err
			}
		}
	} else {
		shootLogger.Infof("[SHOOTED SEED REGISTRATION] Deleting `Seed` object for %s", shoot.Name)
		if err := deregisterAsSeed(ctx, gardenClient, shoot); err != nil {
			message := fmt.Sprintf("Could not deregister shoot %q as seed: %+v", shoot.Name, err)
			shootLogger.Errorf(message)
			c.recorder.Event(shoot, corev1.EventTypeWarning, "SeedDeletion", message)
			return reconcile.Result{}, err
		}

		if err := checkSeedAssociations(ctx, gardenClient.Client(), shoot.Name); err != nil {
			message := fmt.Sprintf("Error during check for associated resources for the to-be-deleted shooted seed %q: %+v", shoot.Name, err)
			shootLogger.Errorf(message)
			c.recorder.Event(shoot, corev1.EventTypeWarning, "SeedDeletion", message)
			return reconcile.Result{}, err
		}

		shootedSeedClient, err := c.clientMap.GetClient(ctx, keys.ForShoot(shoot))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get shooted seed client: %w", err)
		}

		shootLogger.Infof("[SHOOTED SEED REGISTRATION] Deleting gardenlet in seed %s", shoot.Name)
		if err := deleteGardenlet(ctx, shootedSeedClient.Client()); err != nil {
			message := fmt.Sprintf("Could not deregister shoot %q as seed: %+v", shoot.Name, err)
			shootLogger.Errorf(message)
			c.recorder.Event(shoot, corev1.EventTypeWarning, "GardenletDeletion", message)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func getShootSecret(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootSecretBinding := &gardencorev1beta1.SecretBinding{}
	if err := c.Get(ctx, kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName), shootSecretBinding); err != nil {
		return nil, err
	}
	shootSecret := &corev1.Secret{}
	err := c.Get(ctx, kutil.Key(shootSecretBinding.SecretRef.Namespace, shootSecretBinding.SecretRef.Name), shootSecret)
	return shootSecret, err
}

func applySeedBackupConfig(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, shootSecret *corev1.Secret, shootedSeedConfig *gardencorev1beta1helper.ShootedSeed) (*gardencorev1beta1.SeedBackup, error) {
	var backupProfile *gardencorev1beta1.SeedBackup
	if shootedSeedConfig.Backup != nil {
		backupProfile = shootedSeedConfig.Backup.DeepCopy()

		if len(backupProfile.Provider) == 0 {
			backupProfile.Provider = shoot.Spec.Provider.Type
		}

		if len(backupProfile.SecretRef.Name) == 0 || len(backupProfile.SecretRef.Namespace) == 0 {
			var (
				backupSecretName      = fmt.Sprintf("backup-%s", shoot.Name)
				backupSecretNamespace = v1beta1constants.GardenNamespace
			)

			backupSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupSecretName,
					Namespace: backupSecretNamespace,
				},
			}

			if _, err := controllerutil.CreateOrUpdate(ctx, c, backupSecret, func() error {
				backupSecret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
				}
				backupSecret.Type = corev1.SecretTypeOpaque
				backupSecret.Data = shootSecret.Data
				return nil
			}); err != nil {
				return nil, err
			}

			backupProfile.SecretRef.Name = backupSecretName
			backupProfile.SecretRef.Namespace = backupSecretNamespace
		}
	}

	return backupProfile, nil
}

func applySeedSecret(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, shootSecret *corev1.Secret, secretName, secretNamespace string) error {
	shootKubeconfigSecret := &corev1.Secret{}
	if err := c.Get(ctx, kutil.Key(shoot.Namespace, fmt.Sprintf("%s.kubeconfig", shoot.Name)), shootKubeconfigSecret); err != nil {
		return err
	}

	seedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, c, seedSecret, func() error {
		seedSecret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		seedSecret.Type = corev1.SecretTypeOpaque
		seedSecret.Data = shootSecret.Data
		seedSecret.Data[kubernetes.KubeConfig] = shootKubeconfigSecret.Data[kubernetes.KubeConfig]
		return nil
	})
	return err
}

func prepareSeedConfig(ctx context.Context, gardenClient client.Client, seedClient client.Client, shoot *gardencorev1beta1.Shoot, shootedSeedConfig *gardencorev1beta1helper.ShootedSeed, secretRef *corev1.SecretReference) (*gardencorev1beta1.SeedSpec, error) {
	shootSecret, err := getShootSecret(ctx, gardenClient, shoot)
	if err != nil {
		return nil, err
	}

	backupProfile, err := applySeedBackupConfig(ctx, gardenClient, shoot, shootSecret, shootedSeedConfig)
	if err != nil {
		return nil, err
	}

	if secretRef != nil {
		if err := applySeedSecret(ctx, gardenClient, shoot, shootSecret, secretRef.Name, secretRef.Namespace); err != nil {
			return nil, err
		}
	}

	var taints []gardencorev1beta1.SeedTaint
	if shootedSeedConfig.Protected != nil && *shootedSeedConfig.Protected {
		taints = append(taints, gardencorev1beta1.SeedTaint{Key: gardencorev1beta1.SeedTaintProtected})
	}

	var volume *gardencorev1beta1.SeedVolume
	if shootedSeedConfig.MinimumVolumeSize != nil {
		minimumSize, err := resource.ParseQuantity(*shootedSeedConfig.MinimumVolumeSize)
		if err != nil {
			return nil, err
		}
		volume = &gardencorev1beta1.SeedVolume{
			MinimumSize: &minimumSize,
		}
	}

	vpaEnabled, err := mustEnableVPA(ctx, seedClient, shoot)
	if err != nil {
		return nil, err
	}

	return &gardencorev1beta1.SeedSpec{
		Provider: gardencorev1beta1.SeedProvider{
			Type:   shoot.Spec.Provider.Type,
			Region: shoot.Spec.Region,
		},
		DNS: gardencorev1beta1.SeedDNS{
			IngressDomain: fmt.Sprintf("%s.%s", common.IngressPrefix, *(shoot.Spec.DNS.Domain)),
		},
		SecretRef: secretRef,
		Networks: gardencorev1beta1.SeedNetworks{
			Pods:          *shoot.Spec.Networking.Pods,
			Services:      *shoot.Spec.Networking.Services,
			Nodes:         shoot.Spec.Networking.Nodes,
			BlockCIDRs:    shootedSeedConfig.BlockCIDRs,
			ShootDefaults: shootedSeedConfig.ShootDefaults,
		},
		Settings: &gardencorev1beta1.SeedSettings{
			ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
				Enabled: shootedSeedConfig.DisableCapacityReservation == nil || !*shootedSeedConfig.DisableCapacityReservation,
			},
			Scheduling: &gardencorev1beta1.SeedSettingScheduling{
				Visible: shootedSeedConfig.Visible == nil || *shootedSeedConfig.Visible,
			},
			ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
				Enabled: shootedSeedConfig.DisableDNS == nil || !*shootedSeedConfig.DisableDNS,
			},
			VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
				Enabled: vpaEnabled,
			},
		},
		Taints: taints,
		Backup: backupProfile,
		Volume: volume,
	}, nil
}

// registerAsSeed registers a Shoot cluster as a Seed in the Garden cluster.
func registerAsSeed(ctx context.Context, gardenClient client.Client, seedClient client.Client, shoot *gardencorev1beta1.Shoot, shootedSeedConfig *gardencorev1beta1helper.ShootedSeed) error {
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return errors.New("cannot register Shoot as Seed if it does not specify a domain")
	}

	var (
		secretRef = &corev1.SecretReference{
			Name:      fmt.Sprintf("seed-%s", shoot.Name),
			Namespace: v1beta1constants.GardenNamespace,
		}

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: shoot.Name,
			},
		}
	)

	seedSpec, err := prepareSeedConfig(ctx, gardenClient, seedClient, shoot, shootedSeedConfig, secretRef)
	if err != nil {
		return err
	}

	_, err = controllerutil.CreateOrUpdate(ctx, gardenClient, seed, func() error {
		seed.Labels = utils.MergeStringMaps(shoot.Labels, map[string]string{
			v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleSeed,
			v1beta1constants.GardenRole:           v1beta1constants.GardenRoleSeed,
		})
		seed.Spec = *seedSpec
		return nil
	})
	return err
}

// deregisterAsSeed de-registers a Shoot cluster as a Seed in the Garden cluster.
func deregisterAsSeed(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) error {
	seed, err := gardenClient.GardenCore().CoreV1beta1().Seeds().Get(ctx, shoot.Name, kubernetes.DefaultGetOptions())
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := gardenClient.GardenCore().CoreV1beta1().Seeds().Delete(ctx, seed.Name, metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
		return err
	}

	var secretRefs []corev1.SecretReference
	if seed.Spec.SecretRef != nil {
		secretRefs = append(secretRefs, *seed.Spec.SecretRef)
	}
	if seed.Spec.Backup != nil {
		secretRefs = append(secretRefs, seed.Spec.Backup.SecretRef)
	}

	for _, secretRef := range secretRefs {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretRef.Name,
				Namespace: secretRef.Namespace,
			},
		}
		if err := gardenClient.Client().Delete(ctx, secret, kubernetes.DefaultDeleteOptions...); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

const (
	gardenletKubeconfigBootstrapSecretName = "gardenlet-kubeconfig-bootstrap"
	gardenletKubeconfigSecretName          = "gardenlet-kubeconfig"
)

func deployGardenlet(ctx context.Context, gardenClient, seedClient, shootedSeedClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, shootedSeedConfig *gardencorev1beta1helper.ShootedSeed, imageVector imagevector.ImageVector, cfg *config.GardenletConfiguration) error {
	// create bootstrap kubeconfig in case there is no existing gardenlet kubeconfig yet
	var bootstrapKubeconfigValues map[string]interface{}
	if err := shootedSeedClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, gardenletKubeconfigSecretName), &corev1.Secret{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		var bootstrapKubeconfig []byte

		restConfig := *gardenClient.RESTConfig()
		if addr := cfg.GardenClientConnection.GardenClusterAddress; addr != nil {
			restConfig.Host = *addr
		}
		if caCert := cfg.GardenClientConnection.GardenClusterCACert; caCert != nil {
			restConfig.TLSClientConfig = rest.TLSClientConfig{
				CAData: caCert,
			}
		}

		if shootedSeedConfig.UseServiceAccountBootstrapping {
			// create temporary service account with bootstrap kubeconfig in order to create CSR
			saName := "gardenlet-bootstrap-" + shoot.Name
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: v1beta1constants.GardenNamespace,
				},
			}
			if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient.Client(), sa, func() error { return nil }); err != nil {
				return err
			}

			if len(sa.Secrets) == 0 {
				return fmt.Errorf("service account token controller has not yet created a secret for the service account")
			}

			saSecret := &corev1.Secret{}
			if err := gardenClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, sa.Secrets[0].Name), saSecret); err != nil {
				return err
			}

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bootstraputil.BuildBootstrapperName(shoot.Name),
				},
			}
			if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient.Client(), clusterRoleBinding, func() error {
				clusterRoleBinding.RoleRef = rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     bootstraputil.GardenerSeedBootstrapper,
				}
				clusterRoleBinding.Subjects = []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      saName,
						Namespace: v1beta1constants.GardenNamespace,
					},
				}
				return nil
			}); err != nil {
				return err
			}

			bootstrapKubeconfig, err = bootstraputil.MarshalKubeconfigWithToken(&restConfig, string(saSecret.Data[corev1.ServiceAccountTokenKey]))
			if err != nil {
				return err
			}
		} else {
			// create bootstrap token with bootstrap kubeconfig in order to create CSR
			var (
				tokenID               = utils.ComputeSHA256Hex([]byte(shoot.Name))[:6]
				validity              = 24 * time.Hour
				refreshBootstrapToken = true
				bootstrapTokenSecret  *corev1.Secret
			)

			secret := &corev1.Secret{}
			if err := gardenClient.Client().Get(ctx, kutil.Key(metav1.NamespaceSystem, bootstraptokenutil.BootstrapTokenSecretName(tokenID)), secret); client.IgnoreNotFound(err) != nil {
				return err
			}

			if expirationTime, ok := secret.Data[bootstraptokenapi.BootstrapTokenExpirationKey]; ok {
				t, err := time.Parse(time.RFC3339, string(expirationTime))
				if err != nil {
					return err
				}

				if !t.Before(metav1.Now().UTC()) {
					bootstrapTokenSecret = secret
					refreshBootstrapToken = false
				}
			}

			if refreshBootstrapToken {
				bootstrapTokenSecret, err = kutil.ComputeBootstrapToken(ctx, gardenClient.Client(), tokenID, fmt.Sprintf("A bootstrap token for the Gardenlet for shooted seed %q.", shoot.Name), validity)
				if err != nil {
					return err
				}
			}

			bootstrapKubeconfig, err = bootstraputil.MarshalKubeconfigWithToken(&restConfig, kutil.BootstrapTokenFrom(bootstrapTokenSecret.Data))
			if err != nil {
				return err
			}
		}

		bootstrapKubeconfigValues = map[string]interface{}{
			"name":       gardenletKubeconfigBootstrapSecretName,
			"namespace":  v1beta1constants.GardenNamespace,
			"kubeconfig": string(bootstrapKubeconfig),
		}
	}

	// convert config from internal version to v1alpha1 as Helm chart is based on v1alpha1
	scheme := runtime.NewScheme()
	if err := config.AddToScheme(scheme); err != nil {
		return err
	}
	if err := configv1alpha1.AddToScheme(scheme); err != nil {
		return err
	}
	external, err := scheme.ConvertToVersion(cfg, configv1alpha1.SchemeGroupVersion)
	if err != nil {
		return err
	}
	externalConfig, ok := external.(*configv1alpha1.GardenletConfiguration)
	if !ok {
		return fmt.Errorf("error converting config to external version")
	}

	var secretRef *corev1.SecretReference
	if shootedSeedConfig.WithSecretRef {
		secretRef = &corev1.SecretReference{
			Name:      fmt.Sprintf("seed-%s", shoot.Name),
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	seedSpec, err := prepareSeedConfig(ctx, gardenClient.Client(), seedClient.Client(), shoot, shootedSeedConfig, secretRef)
	if err != nil {
		return err
	}

	var imageVectorOverwrite string
	if overWritePath := os.Getenv(imagevector.OverrideEnv); len(overWritePath) > 0 {
		data, err := ioutil.ReadFile(overWritePath)
		if err != nil {
			return err
		}
		imageVectorOverwrite = string(data)
	}

	var componentImageVectorOverwrites string
	if overWritePath := os.Getenv(imagevector.ComponentOverrideEnv); len(overWritePath) > 0 {
		data, err := ioutil.ReadFile(overWritePath)
		if err != nil {
			return err
		}
		componentImageVectorOverwrites = string(data)
	}

	gardenletImage, err := imageVector.FindImage("gardenlet")
	if err != nil {
		return err
	}
	var (
		repository = gardenletImage.String()
		tag        = version.Get().GitVersion
	)
	if gardenletImage.Tag != nil {
		repository = gardenletImage.Repository
		tag = *gardenletImage.Tag
	}

	serverTLSCertificate, err := ioutil.ReadFile(externalConfig.Server.HTTPS.TLS.ServerCertPath)
	if err != nil {
		return err
	}
	serverTLSKey, err := ioutil.ReadFile(externalConfig.Server.HTTPS.TLS.ServerKeyPath)
	if err != nil {
		return err
	}

	values := map[string]interface{}{
		"global": map[string]interface{}{
			"gardenlet": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": repository,
					"tag":        tag,
				},
				"revisionHistoryLimit":           0,
				"vpa":                            true,
				"imageVectorOverwrite":           imageVectorOverwrite,
				"componentImageVectorOverwrites": componentImageVectorOverwrites,
				"config": map[string]interface{}{
					"gardenClientConnection": map[string]interface{}{
						"acceptContentTypes":   externalConfig.GardenClientConnection.AcceptContentTypes,
						"contentType":          externalConfig.GardenClientConnection.ContentType,
						"qps":                  externalConfig.GardenClientConnection.QPS,
						"burst":                externalConfig.GardenClientConnection.Burst,
						"gardenClusterAddress": externalConfig.GardenClientConnection.GardenClusterAddress,
						"bootstrapKubeconfig":  bootstrapKubeconfigValues,
						"kubeconfigSecret": map[string]interface{}{
							"name":      gardenletKubeconfigSecretName,
							"namespace": v1beta1constants.GardenNamespace,
						},
					},
					"seedClientConnection":  externalConfig.SeedClientConnection.ClientConnectionConfiguration,
					"shootClientConnection": externalConfig.ShootClientConnection,
					"controllers":           externalConfig.Controllers,
					"leaderElection":        externalConfig.LeaderElection,
					"logLevel":              externalConfig.LogLevel,
					"kubernetesLogLevel":    externalConfig.KubernetesLogLevel,
					"featureGates":          externalConfig.FeatureGates,
					"server": map[string]interface{}{
						"https": map[string]interface{}{
							"bindAddress": externalConfig.Server.HTTPS.BindAddress,
							"port":        externalConfig.Server.HTTPS.Port,
							"tls": map[string]interface{}{
								"crt": string(serverTLSCertificate),
								"key": string(serverTLSKey),
							},
						},
					},
					"seedConfig": &configv1alpha1.SeedConfig{
						Seed: gardencorev1beta1.Seed{
							ObjectMeta: metav1.ObjectMeta{
								Name:   shoot.Name,
								Labels: shoot.Labels,
							},
							Spec: *seedSpec,
						},
					},
				},
			},
		},
	}

	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.GardenNamespace}}
	if err := shootedSeedClient.Client().Create(ctx, gardenNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return shootedSeedClient.ChartApplier().Apply(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values))
}

func deleteGardenlet(ctx context.Context, c client.Client) error {
	vpa := &unstructured.Unstructured{}
	vpa.SetAPIVersion("autoscaling.k8s.io/v1beta2")
	vpa.SetKind("VerticalPodAutoscaler")
	vpa.SetName("gardenlet-vpa")
	vpa.SetNamespace(v1beta1constants.GardenNamespace)

	for _, obj := range []runtime.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet-configmap", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet-imagevector-overwrite", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenletKubeconfigBootstrapSecretName, Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenletKubeconfigSecretName, Namespace: v1beta1constants.GardenNamespace}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet", Namespace: v1beta1constants.GardenNamespace}},
		&policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet", Namespace: v1beta1constants.GardenNamespace}},
		vpa,
	} {
		if err := c.Delete(ctx, obj); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
			return err
		}
	}

	return nil
}

func checkSeedAssociations(ctx context.Context, c client.Client, seedName string) error {
	var (
		results []string
		err     error
	)

	for name, f := range map[string]func(context.Context, client.Client, string) ([]string, error){
		"BackupBuckets":           controllerutils.DetermineBackupBucketAssociations,
		"BackupEntries":           controllerutils.DetermineBackupEntryAssociations,
		"ControllerInstallations": controllerutils.DetermineControllerInstallationAssociations,
		"Shoots":                  controllerutils.DetermineShootAssociations,
	} {
		results, err = f(ctx, c, seedName)
		if err != nil {
			return err
		}

		if len(results) > 0 {
			return fmt.Errorf("still associated %s with seed %q: %+v", name, seedName, results)
		}
	}

	return nil
}

func mustEnableVPA(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if err := c.Get(ctx, kutil.Key(shoot.Status.TechnicalID, "vpa-admission-controller"), &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			// VPA deployment in shoot namespace was not found, so we have to enable the VPA for this seed until it's
			// being deployed.
			return true, nil
		}
		return false, err
	}

	// VPA deployment in shoot namespace was found, so we don't need to enable the VPA for this seed.
	return false, nil
}
