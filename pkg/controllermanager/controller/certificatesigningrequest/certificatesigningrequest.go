// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificatesigningrequest

import (
	"context"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllermanager"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	kubeinformers "k8s.io/client-go/informers"
	certificatesv1beta1lister "k8s.io/client-go/listers/certificates/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// Controller controls CertificateSigningRequests.
type Controller struct {
	clientMap clientmap.ClientMap

	control  ControlInterface
	recorder record.EventRecorder

	csrLister certificatesv1beta1lister.CertificateSigningRequestLister
	csrQueue  workqueue.RateLimitingInterface
	csrSynced cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewCSRController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <kubeInformerFactory>, and a <recorder> for
// event recording. It creates a new CSR controller.
func NewCSRController(clientMap clientmap.ClientMap, kubeInformerFactory kubeinformers.SharedInformerFactory, recorder record.EventRecorder) *Controller {
	var (
		certificatesV1beta1Informer = kubeInformerFactory.Certificates().V1beta1()
		csrInformer                 = certificatesV1beta1Informer.CertificateSigningRequests()
		csrLister                   = csrInformer.Lister()
	)

	csrController := &Controller{
		clientMap: clientMap,
		control:   NewDefaultControl(clientMap),
		recorder:  recorder,
		csrLister: csrLister,
		csrQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CertificateSigningRequest"),
		workerCh:  make(chan int),
	}

	csrInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    csrController.csrAdd,
		UpdateFunc: csrController.csrUpdate,
	})
	csrController.csrSynced = csrInformer.Informer().HasSynced

	return csrController
}

// Run runs the Controller until the given context <ctx> is alive.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.csrSynced) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running CertificateSigningRequest workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("CertificateSigningRequest controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.DeprecatedCreateWorker(ctx, c.csrQueue, "CertificateSigningRequest", c.reconcileCertificateSigningRequestKey, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.csrQueue.ShutDown()

	for {
		if c.csrQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Debug("No running CertificateSigningRequest worker and no items left in the queues. Terminated CertificateSigningRequest controller...")
			break
		}
		logger.Logger.Debugf("Waiting for %d CertificateSigningRequest worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.csrQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *Controller) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(controllermanager.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "csr")
	if err != nil {
		controllermanager.ScrapeFailures.With(prometheus.Labels{"kind": "csr-controller"}).Inc()
		return
	}
	ch <- metric
}
