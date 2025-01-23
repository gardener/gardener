// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ClientSetBuilder is a builder for fake ClientSets
type ClientSetBuilder struct {
	applier       kubernetes.Applier
	chartRenderer chartrenderer.Interface
	chartApplier  kubernetes.ChartApplier
	podExecutor   kubernetes.PodExecutor
	restConfig    *rest.Config
	client        client.Client
	apiReader     client.Reader
	cache         cache.Cache
	kubernetes    kubernetesclientset.Interface
	restClient    rest.Interface
	version       string
}

// NewClientSetBuilder return a new builder for building fake ClientSets
func NewClientSetBuilder() *ClientSetBuilder {
	return &ClientSetBuilder{}
}

// WithApplier sets the applier attribute of the builder.
func (b *ClientSetBuilder) WithApplier(applier kubernetes.Applier) *ClientSetBuilder {
	b.applier = applier
	return b
}

// WithChartRenderer sets the chartRenderer attribute of the builder.
func (b *ClientSetBuilder) WithChartRenderer(chartRenderer chartrenderer.Interface) *ClientSetBuilder {
	b.chartRenderer = chartRenderer
	return b
}

// WithChartApplier sets the chartApplier attribute of the builder.
func (b *ClientSetBuilder) WithChartApplier(chartApplier kubernetes.ChartApplier) *ClientSetBuilder {
	b.chartApplier = chartApplier
	return b
}

// WithPodExecutor sets the podExecutor attribute of the builder.
func (b *ClientSetBuilder) WithPodExecutor(podExecutor kubernetes.PodExecutor) *ClientSetBuilder {
	b.podExecutor = podExecutor
	return b
}

// WithRESTConfig sets the restConfig attribute of the builder.
func (b *ClientSetBuilder) WithRESTConfig(config *rest.Config) *ClientSetBuilder {
	b.restConfig = config
	return b
}

// WithClient sets the client attribute of the builder.
func (b *ClientSetBuilder) WithClient(client client.Client) *ClientSetBuilder {
	b.client = client
	return b
}

// WithAPIReader sets the apiReader attribute of the builder.
func (b *ClientSetBuilder) WithAPIReader(apiReader client.Reader) *ClientSetBuilder {
	b.apiReader = apiReader
	return b
}

// WithCache sets the cache attribute of the builder.
func (b *ClientSetBuilder) WithCache(cache cache.Cache) *ClientSetBuilder {
	b.cache = cache
	return b
}

// WithKubernetes sets the kubernetes attribute of the builder.
func (b *ClientSetBuilder) WithKubernetes(kubernetes kubernetesclientset.Interface) *ClientSetBuilder {
	b.kubernetes = kubernetes
	return b
}

// WithRESTClient sets the restClient attribute of the builder.
func (b *ClientSetBuilder) WithRESTClient(restClient rest.Interface) *ClientSetBuilder {
	b.restClient = restClient
	return b
}

// WithVersion sets the version attribute of the builder.
func (b *ClientSetBuilder) WithVersion(version string) *ClientSetBuilder {
	b.version = version
	return b
}

// Build builds the ClientSet.
func (b *ClientSetBuilder) Build() *ClientSet {
	return &ClientSet{
		applier:       b.applier,
		chartRenderer: b.chartRenderer,
		chartApplier:  b.chartApplier,
		podExecutor:   b.podExecutor,
		restConfig:    b.restConfig,
		client:        b.client,
		apiReader:     b.apiReader,
		cache:         b.cache,
		kubernetes:    b.kubernetes,
		restClient:    b.restClient,
		version:       b.version,
	}
}
