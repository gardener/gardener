package fake

import (
	internalversion "github.com/gardener/gardener/pkg/client/garden/clientset/internalversion/typed/garden/internalversion"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeGarden struct {
	*testing.Fake
}

func (c *FakeGarden) CloudProfiles() internalversion.CloudProfileInterface {
	return &FakeCloudProfiles{c}
}

func (c *FakeGarden) CrossSecretBindings(namespace string) internalversion.CrossSecretBindingInterface {
	return &FakeCrossSecretBindings{c, namespace}
}

func (c *FakeGarden) PrivateSecretBindings(namespace string) internalversion.PrivateSecretBindingInterface {
	return &FakePrivateSecretBindings{c, namespace}
}

func (c *FakeGarden) Quotas(namespace string) internalversion.QuotaInterface {
	return &FakeQuotas{c, namespace}
}

func (c *FakeGarden) Seeds() internalversion.SeedInterface {
	return &FakeSeeds{c}
}

func (c *FakeGarden) Shoots(namespace string) internalversion.ShootInterface {
	return &FakeShoots{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeGarden) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
