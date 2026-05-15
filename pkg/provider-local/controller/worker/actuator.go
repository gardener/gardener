// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type delegateFactory struct {
	gardenReader  client.Reader
	runtimeClient client.Client
	decoder       runtime.Decoder
	restConfig    *rest.Config
	scheme        *runtime.Scheme
}

type actuator struct {
	worker.Actuator

	workerDelegate *delegateFactory
}

// NewActuator creates a new Actuator that updates the status of the handled WorkerPoolConfigs.
func NewActuator(mgr manager.Manager, gardenCluster cluster.Cluster) worker.Actuator {
	workerDelegate := &delegateFactory{
		runtimeClient: mgr.GetClient(),
		decoder:       serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		restConfig:    mgr.GetConfig(),
		scheme:        mgr.GetScheme(),
	}

	if gardenCluster != nil {
		workerDelegate.gardenReader = gardenCluster.GetAPIReader()
	}

	return &actuator{
		Actuator:       genericactuator.NewActuator(mgr, gardenCluster, workerDelegate, nil),
		workerDelegate: workerDelegate,
	}
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	if err := genericactuator.RestoreWithoutReconcile(ctx, log, a.workerDelegate.gardenReader, a.workerDelegate.runtimeClient, a.workerDelegate, worker, cluster); err != nil {
		return fmt.Errorf("failed restoring the worker state: %w", err)
	}

	return a.Reconcile(ctx, log, worker, cluster)
}

func (d *delegateFactory) WorkerDelegate(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (genericactuator.WorkerDelegate, error) {
	clientset, err := kubernetes.NewForConfig(d.restConfig)
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	return NewWorkerDelegate(
		ctx,
		logf.FromContext(ctx),
		d.runtimeClient,
		d.restConfig,
		d.decoder,
		d.scheme,
		serverVersion.GitVersion,
		worker,
		cluster,
	)
}

type workerDelegate struct {
	// runtimeClient uses provider-local's in-cluster config, e.g., for the seed/bootstrap cluster it runs in.
	// It's used to interact with extension objects. By default, it's also used as the provider client to interact with
	// infrastructure resources, unless a kubeconfig is specified in the cloudprovider secret.
	runtimeClient client.Client
	// providerClient is a client for the cluster in which provider-local should manage infrastructure resources,
	// e.g., Services, NetworkPolicies, machine Pods, etc. If the provider secret contains a kubeconfig, a client for that
	// kubeconfig is created. Otherwise, the given client for the runtime cluster is returned.
	// See https://github.com/gardener/gardener/blob/master/docs/extensions/provider-local.md#credentials.
	providerClient client.Client
	decoder        runtime.Decoder
	scheme         *runtime.Scheme

	podExecutor         kubernetesclient.PodExecutor
	serverVersion       string
	cloudProfileConfig  *api.CloudProfileConfig
	cluster             *extensionscontroller.Cluster
	worker              *extensionsv1alpha1.Worker
	machineClassSecrets []*corev1.Secret
	machineClasses      []*machinev1alpha1.MachineClass
	machineImages       []api.MachineImage
	machineDeployments  worker.MachineDeployments
}

// NewWorkerDelegate creates a new context for a worker reconciliation.
func NewWorkerDelegate(
	ctx context.Context,
	log logr.Logger,
	runtimeClient client.Client,
	restConfig *rest.Config,
	decoder runtime.Decoder,
	scheme *runtime.Scheme,
	serverVersion string,
	worker *extensionsv1alpha1.Worker,
	cluster *extensionscontroller.Cluster,
) (
	genericactuator.WorkerDelegate,
	error,
) {
	config, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	providerSecret, err := kubernetesutils.GetSecretByReference(ctx, runtimeClient, &worker.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve provider secret: %w", err)
	}

	var (
		providerClient = runtimeClient
		podExecutor    = kubernetesclient.NewPodExecutor(restConfig)
	)

	if len(providerSecret.Data[kubernetesclient.KubeConfig]) == 0 {
		log.Info("Using in-cluster config for provider client as no kubeconfig is specified in the provider secret")
	} else {
		clientSet, err := kubernetesclient.NewClientFromBytes(providerSecret.Data[kubernetesclient.KubeConfig],
			kubernetesclient.WithClientOptions(client.Options{Scheme: kubernetesclient.SeedScheme}),
			kubernetesclient.WithDisabledCachedClient(),
		)
		if err != nil {
			return nil, fmt.Errorf("could not create client from provider secret: %w", err)
		}

		log.Info("Using kubeconfig from provider secret for provider client")

		providerClient = clientSet.Client()
		podExecutor = clientSet.PodExecutor()
	}

	return &workerDelegate{
		scheme:             scheme,
		runtimeClient:      runtimeClient,
		providerClient:     providerClient,
		decoder:            decoder,
		podExecutor:        podExecutor,
		serverVersion:      serverVersion,
		cloudProfileConfig: config,
		cluster:            cluster,
		worker:             worker,
	}, nil
}
