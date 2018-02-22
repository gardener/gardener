package shoot

import (
	"fmt"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) shootQuotaAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootQuotaQueue.AddAfter(key, 1*time.Hour)
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
		return nil
	}
	if err != nil {
		return err
	}
	if err := c.quotaControl.CheckQuota(shoot, key); err != nil {
		c.shootQuotaQueue.AddAfter(key, 2*time.Minute)
	}
	return nil
}

// QuotaControlInterface implements the control logic for quota management of Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type QuotaControlInterface interface {
	CheckQuota(shoot *gardenv1beta1.Shoot, key string) error
}

// NewDefaultQuotaControl returns a new instance of the default implementation of QuotaControlInterface
// which implements the semantics for controling the quota handling of Shoot resources.
func NewDefaultQuotaControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface) QuotaControlInterface {
	return &defaultQuotaControl{k8sGardenClient, k8sGardenInformers}
}

type defaultQuotaControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.Interface
}

func (c *defaultQuotaControl) CheckQuota(shootObj *gardenv1beta1.Shoot, key string) error {
	var (
		operationID     = utils.GenerateRandomString(8)
		shoot           = shootObj.DeepCopy()
		shootLogger     = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, operationID)
		quotaReferences []v1.ObjectReference
		clusterLifeTime *int
	)

	switch shoot.Spec.Cloud.SecretBindingRef.Kind {
	case "PrivateSecretBinding":
		psb, err := c.k8sGardenInformers.
			PrivateSecretBindings().
			Lister().
			PrivateSecretBindings(shoot.Namespace).
			Get(shoot.Spec.Cloud.SecretBindingRef.Name)
		if err != nil {
			return err
		}
		quotaReferences = psb.Quotas
	case "CrossSecretBinding":
		csb, err := c.k8sGardenInformers.
			CrossSecretBindings().
			Lister().
			CrossSecretBindings(shoot.Namespace).
			Get(shoot.Spec.Cloud.SecretBindingRef.Name)
		if err != nil {
			return err
		}
		quotaReferences = csb.Quotas
	default:
		return fmt.Errorf("Unknown Binding type %s", shoot.Spec.Cloud.SecretBindingRef.Kind)
	}

	for _, quotaRef := range quotaReferences {
		quota, err := c.k8sGardenInformers.
			Quotas().
			Lister().
			Quotas(quotaRef.Namespace).
			Get(quotaRef.Name)
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

	if clusterLifeTime == nil {
		return nil
	}

	shootExpirationTime := shoot.CreationTimestamp.Add(time.Duration(*clusterLifeTime*24) * time.Hour)
	if time.Now().After(shootExpirationTime) {
		shootLogger.Info("[SHOOT QUOTA] Shoot cluster lifetime expired. Shoot will be deleted.")

		// We have to delete the shoot, because only apiserver can set a deletionTimestamp
		err := c.k8sGardenClient.
			GardenClientset().
			GardenV1beta1().
			Shoots(shoot.Namespace).
			Delete(shoot.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		shootUpdated, err := c.k8sGardenClient.
			GardenClientset().
			GardenV1beta1().
			Shoots(shoot.Namespace).
			Get(shoot.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// After the shoot has set a deletionTimestamp, we can use this timestamp to set the
		// deletionTimestampConfirmation annotation to trigger the shoot deletion process.
		annotations := shootUpdated.ObjectMeta.Annotations
		annotations[common.ConfirmationDeletionTimestamp] = shootUpdated.DeletionTimestamp.Format(time.RFC3339)
		shootUpdated.ObjectMeta.Annotations = annotations

		_, err = c.k8sGardenClient.
			GardenClientset().
			GardenV1beta1().
			Shoots(shootUpdated.Namespace).
			Update(shootUpdated)
		if err != nil {
			return err
		}
	}
	return nil
}
