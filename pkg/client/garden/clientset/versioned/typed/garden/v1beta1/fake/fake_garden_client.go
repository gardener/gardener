package fake

import (
	v1beta1 "github.com/gardener/gardener/pkg/client/garden/clientset/versioned/typed/garden/v1beta1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeGardenV1beta1 struct {
	*testing.Fake
}

func (c *FakeGardenV1beta1) CloudProfiles() v1beta1.CloudProfileInterface {
	return &FakeCloudProfiles{c}
}

func (c *FakeGardenV1beta1) CrossSecretBindings(namespace string) v1beta1.CrossSecretBindingInterface {
	return &FakeCrossSecretBindings{c, namespace}
}

func (c *FakeGardenV1beta1) PrivateSecretBindings(namespace string) v1beta1.PrivateSecretBindingInterface {
	return &FakePrivateSecretBindings{c, namespace}
}

func (c *FakeGardenV1beta1) Quotas(namespace string) v1beta1.QuotaInterface {
	return &FakeQuotas{c, namespace}
}

func (c *FakeGardenV1beta1) Seeds() v1beta1.SeedInterface {
	return &FakeSeeds{c}
}

func (c *FakeGardenV1beta1) Shoots(namespace string) v1beta1.ShootInterface {
	return &FakeShoots{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeGardenV1beta1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
