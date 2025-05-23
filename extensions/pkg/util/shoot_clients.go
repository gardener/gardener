// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	extensionsconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// ShootClients bundles together several clients for the shoot cluster.
type ShootClients interface {
	Client() client.Client
	Clientset() kubernetes.Interface
	GardenerClientset() kubernetesclient.Interface
	ChartApplier() kubernetesclient.ChartApplier
	Version() *version.Info
}

type shootClients struct {
	c                 client.Client
	clientset         kubernetes.Interface
	gardenerClientset kubernetesclient.Interface
	chartApplier      kubernetesclient.ChartApplier
	version           *version.Info
}

func (s *shootClients) Client() client.Client                         { return s.c }
func (s *shootClients) Clientset() kubernetes.Interface               { return s.clientset }
func (s *shootClients) GardenerClientset() kubernetesclient.Interface { return s.gardenerClientset }
func (s *shootClients) ChartApplier() kubernetesclient.ChartApplier   { return s.chartApplier }
func (s *shootClients) Version() *version.Info                        { return s.version }

// NewShootClients creates a new shoot client interface based on the given clients.
func NewShootClients(c client.Client, clientset kubernetes.Interface, gardenerClientset kubernetesclient.Interface, chartApplier kubernetesclient.ChartApplier, version *version.Info) ShootClients {
	return &shootClients{
		c:                 c,
		clientset:         clientset,
		gardenerClientset: gardenerClientset,
		chartApplier:      chartApplier,
		version:           version,
	}
}

// ApplyRESTOptions applies RESTOptions to the given rest.Config
func ApplyRESTOptions(restConfig *rest.Config, restOptions extensionsconfigv1alpha1.RESTOptions) *rest.Config {
	restConfig.QPS = ptr.Deref(restOptions.QPS, restConfig.QPS)
	restConfig.Burst = ptr.Deref(restOptions.Burst, restConfig.Burst)
	restConfig.Timeout = ptr.Deref(restOptions.Timeout, restConfig.Timeout)
	return restConfig
}

// NewClientForShoot returns the rest config and the client for the given shoot namespace. It first looks to use the "internal" kubeconfig
// (the one with in-cluster address) as in-cluster traffic is free of charge. If it cannot find that, then it fallbacks to the "external" kubeconfig
// (the one with external DNS name or load balancer address) and this usually translates to egress traffic costs.
// However, if the environment variable GARDENER_SHOOT_CLIENT=external, then it *only* checks for the external endpoint,
// i.e. v1beta1constants.SecretNameGardener. This is useful when connecting from outside the seed cluster on which the shoot kube-apiserver
// is running.
func NewClientForShoot(ctx context.Context, c client.Client, namespace string, opts client.Options, restOptions extensionsconfigv1alpha1.RESTOptions) (*rest.Config, client.Client, error) {
	var (
		gardenerSecret = &corev1.Secret{}
		err            error
	)

	if os.Getenv("GARDENER_SHOOT_CLIENT") != "external" {
		if err = c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameGardenerInternal}, gardenerSecret); err != nil && apierrors.IsNotFound(err) {
			err = c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameGardener}, gardenerSecret)
		}
	} else {
		err = c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameGardener}, gardenerSecret)
	}
	if err != nil {
		return nil, nil, err
	}

	shootRESTConfig, err := NewRESTConfigFromKubeconfig(gardenerSecret.Data[secrets.DataKeyKubeconfig])
	if err != nil {
		return nil, nil, err
	}
	ApplyRESTOptions(shootRESTConfig, restOptions)

	if opts.Mapper == nil {
		httpClient, err := rest.HTTPClientFor(shootRESTConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get HTTP client for config: %w", err)
		}

		mapper, err := apiutil.NewDynamicRESTMapper(shootRESTConfig, httpClient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create new DynamicRESTMapper: %w", err)
		}
		opts.Mapper = mapper
	}

	shootClient, err := client.New(shootRESTConfig, opts)
	if err != nil {
		return nil, nil, err
	}
	return shootRESTConfig, shootClient, nil
}

// NewClientsForShoot is a utility function that creates a new clientset and a chart applier for the shoot cluster.
// It uses the 'gardener' secret in the given shoot namespace. It also returns the Kubernetes version of the cluster.
func NewClientsForShoot(ctx context.Context, c client.Client, namespace string, opts client.Options, restOptions extensionsconfigv1alpha1.RESTOptions) (ShootClients, error) {
	shootRESTConfig, shootClient, err := NewClientForShoot(ctx, c, namespace, opts, restOptions)
	if err != nil {
		return nil, err
	}
	ApplyRESTOptions(shootRESTConfig, restOptions)
	shootClientset, err := kubernetes.NewForConfig(shootRESTConfig)
	if err != nil {
		return nil, err
	}
	shootGardenerClientset, err := kubernetesclient.NewWithConfig(kubernetesclient.WithRESTConfig(shootRESTConfig), kubernetesclient.WithClientOptions(opts))
	if err != nil {
		return nil, err
	}
	shootVersion, err := shootClientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	shootChartApplier := shootGardenerClientset.ChartApplier()

	return &shootClients{
		c:                 shootClient,
		clientset:         shootClientset,
		gardenerClientset: shootGardenerClientset,
		chartApplier:      shootChartApplier,
		version:           shootVersion,
	}, nil
}

// NewChartRendererForShoot creates a new chartrenderer.Interface for the shoot cluster.
func NewChartRendererForShoot(version string) (chartrenderer.Interface, error) {
	v, err := VersionInfo(version)
	if err != nil {
		return nil, err
	}
	return chartrenderer.NewWithServerVersion(v), nil
}
