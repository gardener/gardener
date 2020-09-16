// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShootClients bundles together several clients for the shoot cluster.
type ShootClients interface {
	Client() client.Client
	Clientset() kubernetes.Interface
	GardenerClientset() gardenerkubernetes.Interface
	ChartApplier() gardenerkubernetes.ChartApplier
	Version() *version.Info
}

type shootClients struct {
	c                 client.Client
	clientset         kubernetes.Interface
	gardenerClientset gardenerkubernetes.Interface
	chartApplier      gardenerkubernetes.ChartApplier
	version           *version.Info
}

func (s *shootClients) Client() client.Client                           { return s.c }
func (s *shootClients) Clientset() kubernetes.Interface                 { return s.clientset }
func (s *shootClients) GardenerClientset() gardenerkubernetes.Interface { return s.gardenerClientset }
func (s *shootClients) ChartApplier() gardenerkubernetes.ChartApplier   { return s.chartApplier }
func (s *shootClients) Version() *version.Info                          { return s.version }

// NewShootClients creates a new shoot client interface based on the given clients.
func NewShootClients(c client.Client, clientset kubernetes.Interface, gardenerClientset gardenerkubernetes.Interface, chartApplier gardenerkubernetes.ChartApplier, version *version.Info) ShootClients {
	return &shootClients{
		c:                 c,
		clientset:         clientset,
		gardenerClientset: gardenerClientset,
		chartApplier:      chartApplier,
		version:           version,
	}
}

// NewClientForShoot returns the rest config and the client for the given shoot namespace.
func NewClientForShoot(ctx context.Context, c client.Client, namespace string, opts client.Options) (*rest.Config, client.Client, error) {
	var (
		gardenerSecret = &corev1.Secret{}
		err            error
	)

	if err = c.Get(ctx, kutil.Key(namespace, v1beta1constants.SecretNameGardenerInternal), gardenerSecret); err != nil && apierrors.IsNotFound(err) {
		err = c.Get(ctx, kutil.Key(namespace, v1beta1constants.SecretNameGardener), gardenerSecret)
	}
	if err != nil {
		return nil, nil, err
	}

	shootRESTConfig, err := NewRESTConfigFromKubeconfig(gardenerSecret.Data[secrets.DataKeyKubeconfig])
	if err != nil {
		return nil, nil, err
	}
	shootClient, err := client.New(shootRESTConfig, opts)
	if err != nil {
		return nil, nil, err
	}
	return shootRESTConfig, shootClient, nil
}

// NewClientsForShoot is a utility function that creates a new clientset and a chart applier for the shoot cluster.
// It uses the 'gardener' secret in the given shoot namespace. It also returns the Kubernetes version of the cluster.
func NewClientsForShoot(ctx context.Context, c client.Client, namespace string, opts client.Options) (ShootClients, error) {
	shootRESTConfig, shootClient, err := NewClientForShoot(ctx, c, namespace, opts)
	if err != nil {
		return nil, err
	}
	shootClientset, err := kubernetes.NewForConfig(shootRESTConfig)
	if err != nil {
		return nil, err
	}
	shootGardenerClientset, err := gardenerkubernetes.NewWithConfig(gardenerkubernetes.WithRESTConfig(shootRESTConfig), gardenerkubernetes.WithClientOptions(opts))
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
