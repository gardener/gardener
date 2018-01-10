package fake

import (
	garden "github.com/gardener/gardener/pkg/apis/garden"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeCrossSecretBindings implements CrossSecretBindingInterface
type FakeCrossSecretBindings struct {
	Fake *FakeGarden
	ns   string
}

var crosssecretbindingsResource = schema.GroupVersionResource{Group: "garden.sapcloud.io", Version: "", Resource: "crosssecretbindings"}

var crosssecretbindingsKind = schema.GroupVersionKind{Group: "garden.sapcloud.io", Version: "", Kind: "CrossSecretBinding"}

// Get takes name of the crossSecretBinding, and returns the corresponding crossSecretBinding object, and an error if there is any.
func (c *FakeCrossSecretBindings) Get(name string, options v1.GetOptions) (result *garden.CrossSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(crosssecretbindingsResource, c.ns, name), &garden.CrossSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*garden.CrossSecretBinding), err
}

// List takes label and field selectors, and returns the list of CrossSecretBindings that match those selectors.
func (c *FakeCrossSecretBindings) List(opts v1.ListOptions) (result *garden.CrossSecretBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(crosssecretbindingsResource, crosssecretbindingsKind, c.ns, opts), &garden.CrossSecretBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &garden.CrossSecretBindingList{}
	for _, item := range obj.(*garden.CrossSecretBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested crossSecretBindings.
func (c *FakeCrossSecretBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(crosssecretbindingsResource, c.ns, opts))

}

// Create takes the representation of a crossSecretBinding and creates it.  Returns the server's representation of the crossSecretBinding, and an error, if there is any.
func (c *FakeCrossSecretBindings) Create(crossSecretBinding *garden.CrossSecretBinding) (result *garden.CrossSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(crosssecretbindingsResource, c.ns, crossSecretBinding), &garden.CrossSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*garden.CrossSecretBinding), err
}

// Update takes the representation of a crossSecretBinding and updates it. Returns the server's representation of the crossSecretBinding, and an error, if there is any.
func (c *FakeCrossSecretBindings) Update(crossSecretBinding *garden.CrossSecretBinding) (result *garden.CrossSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(crosssecretbindingsResource, c.ns, crossSecretBinding), &garden.CrossSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*garden.CrossSecretBinding), err
}

// Delete takes name of the crossSecretBinding and deletes it. Returns an error if one occurs.
func (c *FakeCrossSecretBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(crosssecretbindingsResource, c.ns, name), &garden.CrossSecretBinding{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCrossSecretBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(crosssecretbindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &garden.CrossSecretBindingList{})
	return err
}

// Patch applies the patch and returns the patched crossSecretBinding.
func (c *FakeCrossSecretBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *garden.CrossSecretBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(crosssecretbindingsResource, c.ns, name, data, subresources...), &garden.CrossSecretBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*garden.CrossSecretBinding), err
}
