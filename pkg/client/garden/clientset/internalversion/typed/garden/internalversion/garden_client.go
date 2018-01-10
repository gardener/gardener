package internalversion

import (
	"github.com/gardener/gardener/pkg/client/garden/clientset/internalversion/scheme"
	rest "k8s.io/client-go/rest"
)

type GardenInterface interface {
	RESTClient() rest.Interface
	CloudProfilesGetter
	CrossSecretBindingsGetter
	PrivateSecretBindingsGetter
	QuotasGetter
	SeedsGetter
	ShootsGetter
}

// GardenClient is used to interact with features provided by the garden.sapcloud.io group.
type GardenClient struct {
	restClient rest.Interface
}

func (c *GardenClient) CloudProfiles() CloudProfileInterface {
	return newCloudProfiles(c)
}

func (c *GardenClient) CrossSecretBindings(namespace string) CrossSecretBindingInterface {
	return newCrossSecretBindings(c, namespace)
}

func (c *GardenClient) PrivateSecretBindings(namespace string) PrivateSecretBindingInterface {
	return newPrivateSecretBindings(c, namespace)
}

func (c *GardenClient) Quotas(namespace string) QuotaInterface {
	return newQuotas(c, namespace)
}

func (c *GardenClient) Seeds() SeedInterface {
	return newSeeds(c)
}

func (c *GardenClient) Shoots(namespace string) ShootInterface {
	return newShoots(c, namespace)
}

// NewForConfig creates a new GardenClient for the given config.
func NewForConfig(c *rest.Config) (*GardenClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &GardenClient{client}, nil
}

// NewForConfigOrDie creates a new GardenClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *GardenClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new GardenClient for the given RESTClient.
func New(c rest.Interface) *GardenClient {
	return &GardenClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("garden.sapcloud.io")
	if err != nil {
		return err
	}

	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != g.GroupVersion.Group {
		gv := g.GroupVersion
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *GardenClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
