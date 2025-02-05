// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package publicinfo

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/workqueue"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	name           = "gardener-apiserver"
	controllerName = name + "_public-info"
	configMapName  = "gardener-info"
)

// Controller is a kubernetes controller with leader election that reconciles
// the gardener-apiserver public information into the gardener-system-public/gardener-info ConfigMap.
type Controller struct {
	clientSet      *kubernetes.Clientset
	payload        []byte
	leaderElection leaderelection.LeaderElectionConfig

	queue  workqueue.TypedRateLimitingInterface[string]
	lister corev1listers.ConfigMapLister
}

// NewController creates new Controller. <data> parameter holds the information that is
// put in the gardener-info ConfigMap under the gardenerAPIServer data key.
func NewController(client *kubernetes.Clientset, data []byte) (*Controller, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %w", err)
	}

	lock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		metav1.NamespaceSystem,
		name,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: hostname + "_" + string(uuid.NewUUID()),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create resource lock %q: %w", name, err)
	}

	l := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: time.Second * 15,
		RenewDeadline: time.Second * 10,
		RetryPeriod:   time.Second * 2,
	}

	return &Controller{
		clientSet:      client,
		payload:        data,
		leaderElection: l,
	}, nil
}

// Start starts the controller and exits without an error on success.
func (c *Controller) Start(ctx context.Context, log logr.Logger) error {
	log = log.WithName(controllerName)

	c.queue = workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: controllerName},
	)

	informerFactory := informers.NewSharedInformerFactoryWithOptions(c.clientSet, time.Minute,
		informers.WithNamespace(gardencorev1beta1.GardenerSystemPublicNamespace),
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.FieldSelector = fields.OneTermEqualSelector("metadata.name", configMapName).String()
		}),
	)
	informer := informerFactory.Core().V1().ConfigMaps().Informer()
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				c.queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
	}); err != nil {
		return fmt.Errorf("cannot register event handler: %w", err)
	}

	c.lister = informerFactory.Core().V1().ConfigMaps().Lister()
	go informerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("controller %q, timed out waiting for caches to sync", controllerName)
	}

	var cancel context.CancelFunc
	c.leaderElection.Callbacks = leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			// enqueue the ConfigMap on start-up to ensure it is created
			c.queue.Add(gardencorev1beta1.GardenerSystemPublicNamespace + "/" + configMapName)

			ctx, cancel = context.WithCancel(ctx)
			c.start(ctx, log)
		},
		OnStoppedLeading: func() {
			cancel()
		},
	}

	elector, err := leaderelection.NewLeaderElector(c.leaderElection)
	if err != nil {
		return err
	}
	go func() { elector.Run(ctx) }()

	return nil
}

func (c *Controller) start(ctx context.Context, log logr.Logger) {
	defer c.queue.ShutDown()
	defer func() {
		log.Info("Stopping controller")
	}()
	log.Info("Starting controller")

	go wait.Until(
		func() {
			c.runWorker(ctx, log)
		},
		time.Minute,
		ctx.Done(),
	)

	<-ctx.Done()
}

func (c *Controller) runWorker(ctx context.Context, log logr.Logger) {
	log.Info("Starting worker for the controller")
	for c.processNextWorkItem(ctx, log) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context, log logr.Logger) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	log = log.WithValues("key", key)
	log.V(1).Info("Processing new item")

	err, shouldReenqueue := c.reconcileConfigMap(ctx, key, log)
	if err != nil {
		log.Error(err, "Failed to process item")
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	if shouldReenqueue {
		c.queue.AddAfter(key, time.Minute)
	}
	return true

}

func (c *Controller) reconcileConfigMap(ctx context.Context, key string, log logr.Logger) (error, bool) {
	const gardenerAPIServerDataKey = "gardenerAPIServer"

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err, false
	}
	if namespace != gardencorev1beta1.GardenerSystemPublicNamespace {
		log.V(1).Info("Skipping item, not the expected namespace", "namespace", namespace)
		return nil, false
	}
	if name != configMapName {
		log.V(1).Info("Skipping item, not the expected name", "name", name)
		return nil, false
	}

	if err, inTermination := c.ensureNamespace(ctx, namespace, log); err != nil {
		return err, false
	} else if inTermination {
		return nil, true
	}

	configMap, err := c.lister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err, false
		}

		log.Info("Creating the configMap")
		configMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gardencorev1beta1.GardenerSystemPublicNamespace,
				Name:      configMapName,
			},
			Data: map[string]string{
				gardenerAPIServerDataKey: string(c.payload),
			},
		}
		_, err = c.clientSet.CoreV1().ConfigMaps(gardencorev1beta1.GardenerSystemPublicNamespace).Create(ctx, &configMap, metav1.CreateOptions{})
		return err, false
	}

	if configMap.DeletionTimestamp != nil {
		log.Info("ConfigMap is being deleted, retry later")
		return nil, true
	}

	original := configMap.DeepCopy()
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	configMap.Data[gardenerAPIServerDataKey] = string(c.payload)
	if reflect.DeepEqual(original, configMap) {
		log.V(1).Info("ConfigMap is already up to date")
		return nil, false
	}

	log.Info("Updating the configMap")
	_, err = c.clientSet.CoreV1().ConfigMaps(gardencorev1beta1.GardenerSystemPublicNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
	return err, false
}

func (c *Controller) ensureNamespace(ctx context.Context, namespace string, log logr.Logger) (error, bool) {
	log = log.WithValues("namespaceName", namespace)
	ns, err := c.clientSet.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		if ns.DeletionTimestamp != nil {
			log.Info("Namespace is being terminated, retry later")
			return nil, true
		}

		log.V(1).Info("Namespace already exist")
		return nil, false
	}

	if !apierrors.IsNotFound(err) {
		return err, false
	}

	log.Info("Creating the namespace")
	_, err = c.clientSet.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
	return err, false
}
