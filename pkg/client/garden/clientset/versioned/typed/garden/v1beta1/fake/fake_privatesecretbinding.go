package fake

import (
	v1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePrivateSecretBindings implements PrivateSecretBindingInterface
type FakePrivateSecretBindings struct {
	Fake *FakeGardenV1beta1
	ns   string
}

var privatesecretbindingsResource = schema.GroupVersionResource{Group: "garden.sapcloud.io", Version: "v1beta1", Resource: "privatesecretbindings"}

var privatesecretbindingsKind = schema.GroupVersionKind{Group: "garden.sapcloud.io", Version: "v1beta1", Kind: "PrivateSecretBinding"}

// Get takes name of the privateSecretBinding, and returns the corresponding privateSecretBinding object, and an error if there is any.
func (c *FakePrivateSecretBindings) Get(name string, options v1.GetOptions) (result *v1beta1.PrivateSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(privatesecretbindingsResource, c.ns, name), &v1beta1.PrivateSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PrivateSecretBinding), err
}

// List takes label and field selectors, and returns the list of PrivateSecretBindings that match those selectors.
func (c *FakePrivateSecretBindings) List(opts v1.ListOptions) (result *v1beta1.PrivateSecretBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(privatesecretbindingsResource, privatesecretbindingsKind, c.ns, opts), &v1beta1.PrivateSecretBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.PrivateSecretBindingList{}
	for _, item := range obj.(*v1beta1.PrivateSecretBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested privateSecretBindings.
func (c *FakePrivateSecretBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(privatesecretbindingsResource, c.ns, opts))

}

// Create takes the representation of a privateSecretBinding and creates it.  Returns the server's representation of the privateSecretBinding, and an error, if there is any.
func (c *FakePrivateSecretBindings) Create(privateSecretBinding *v1beta1.PrivateSecretBinding) (result *v1beta1.PrivateSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(privatesecretbindingsResource, c.ns, privateSecretBinding), &v1beta1.PrivateSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PrivateSecretBinding), err
}

// Update takes the representation of a privateSecretBinding and updates it. Returns the server's representation of the privateSecretBinding, and an error, if there is any.
func (c *FakePrivateSecretBindings) Update(privateSecretBinding *v1beta1.PrivateSecretBinding) (result *v1beta1.PrivateSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(privatesecretbindingsResource, c.ns, privateSecretBinding), &v1beta1.PrivateSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PrivateSecretBinding), err
}

// Delete takes name of the privateSecretBinding and deletes it. Returns an error if one occurs.
func (c *FakePrivateSecretBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(privatesecretbindingsResource, c.ns, name), &v1beta1.PrivateSecretBinding{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePrivateSecretBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(privatesecretbindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1beta1.PrivateSecretBindingList{})
	return err
}

// Patch applies the patch and returns the patched privateSecretBinding.
func (c *FakePrivateSecretBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.PrivateSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(privatesecretbindingsResource, c.ns, name, data, subresources...), &v1beta1.PrivateSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PrivateSecretBinding), err
}
