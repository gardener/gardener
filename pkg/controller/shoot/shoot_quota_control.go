package shoot

import (
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) shootQuotaAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootQuotaQueue.Add(key)
}

func (c *Controller) shootQuotaDelete(obj interface{}) {
	shoot, ok := obj.(*gardenv1beta1.Shoot)
	if shoot == nil || !ok {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootQuotaQueue.Done(key)
}

func (c *Controller) reconcileShootQuotaKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT QUOTA] %s - skipping because Shoot has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT QUOTA] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.quotaControl.CheckQuota(shoot, key); err != nil {
		c.shootQuotaQueue.AddAfter(key, 2*time.Minute)
		return nil
	}
	c.shootQuotaQueue.AddAfter(key, c.config.Controllers.ShootQuota.SyncPeriod.Duration)
	return nil
}

// QuotaControlInterface implements the control logic for quota management of Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type QuotaControlInterface interface {
	CheckQuota(shoot *gardenv1beta1.Shoot, key string) error
}

// NewDefaultQuotaControl returns a new instance of the default implementation of QuotaControlInterface
// which implements the semantics for controlling the quota handling of Shoot resources.
func NewDefaultQuotaControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface) QuotaControlInterface {
	return &defaultQuotaControl{k8sGardenClient, k8sGardenInformers}
}

type defaultQuotaControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.Interface
}

func (c *defaultQuotaControl) CheckQuota(shootObj *gardenv1beta1.Shoot, key string) error {
	var (
		clusterLifeTime *int
		shoot           = shootObj.DeepCopy()
		shootLogger     = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, "")
	)

	secretBinding, err := c.k8sGardenInformers.SecretBindings().Lister().SecretBindings(shoot.Namespace).Get(shoot.Spec.Cloud.SecretBindingRef.Name)
	if err != nil {
		return err
	}
	for _, quotaRef := range secretBinding.Quotas {
		quota, err := c.k8sGardenInformers.Quotas().Lister().Quotas(quotaRef.Namespace).Get(quotaRef.Name)
		if err != nil {
			return err
		}

		if quota.Spec.ClusterLifetimeDays == nil {
			continue
		}
		if clusterLifeTime == nil || *quota.Spec.ClusterLifetimeDays < *clusterLifeTime {
			clusterLifeTime = quota.Spec.ClusterLifetimeDays
		}
	}

	// If the Shoot has no Quotas referenced (anymore) or if the referenced Quotas does not have a clusterLifetime,
	// then we will not check for cluster lifetime expiration, even if the Shoot has a clusterLifetime timestamp already annotated.
	if clusterLifeTime == nil {
		return nil
	}

	expirationTime, exits := shoot.Annotations[common.ShootExpirationTimestamp]
	if !exits {
		annotations := shoot.Annotations
		annotations[common.ShootExpirationTimestamp] = shoot.CreationTimestamp.Add(time.Duration(*clusterLifeTime*24) * time.Hour).Format(time.RFC3339)
		shoot.Annotations = annotations

		_, err := c.k8sGardenClient.GardenClientset().GardenV1beta1().Shoots(shoot.Namespace).Update(shoot)
		if err != nil {
			return err
		}

		expirationTime = annotations[common.ShootExpirationTimestamp]
	}
	expirationTimeParsed, err := time.Parse(time.RFC3339, expirationTime)
	if err != nil {
		return err
	}

	if time.Now().After(expirationTimeParsed) {
		shootLogger.Info("[SHOOT QUOTA] Shoot cluster lifetime expired. Shoot will be deleted.")

		// We have to delete the shoot, because only apiserver can set a deletionTimestamp
		err := c.k8sGardenClient.GardenClientset().GardenV1beta1().Shoots(shoot.Namespace).Delete(shoot.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		shootUpdated, err := c.k8sGardenClient.GardenClientset().GardenV1beta1().Shoots(shoot.Namespace).Get(shoot.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// After the shoot has got a deletionTimestamp, we use this timestamp to set the
		// deletionTimestampConfirmation annotation, which initiate the shoot deletion process.
		annotations := shootUpdated.ObjectMeta.Annotations
		annotations[common.ConfirmationDeletionTimestamp] = shootUpdated.DeletionTimestamp.Format(time.RFC3339)
		shootUpdated.ObjectMeta.Annotations = annotations

		_, err = c.k8sGardenClient.GardenClientset().GardenV1beta1().Shoots(shootUpdated.Namespace).Update(shootUpdated)
		if err != nil {
			return err
		}
	}
	return nil
}
