package internalversion

import (
	garden "github.com/gardener/gardener/pkg/apis/garden"
	scheme "github.com/gardener/gardener/pkg/client/garden/clientset/internalversion/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PrivateSecretBindingsGetter has a method to return a PrivateSecretBindingInterface.
// A group's client should implement this interface.
type PrivateSecretBindingsGetter interface {
	PrivateSecretBindings(namespace string) PrivateSecretBindingInterface
}

// PrivateSecretBindingInterface has methods to work with PrivateSecretBinding resources.
type PrivateSecretBindingInterface interface {
	Create(*garden.PrivateSecretBinding) (*garden.PrivateSecretBinding, error)
	Update(*garden.PrivateSecretBinding) (*garden.PrivateSecretBinding, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*garden.PrivateSecretBinding, error)
	List(opts v1.ListOptions) (*garden.PrivateSecretBindingList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *garden.PrivateSecretBinding, err error)
	PrivateSecretBindingExpansion
}

// privateSecretBindings implements PrivateSecretBindingInterface
type privateSecretBindings struct {
	client rest.Interface
	ns     string
}

// newPrivateSecretBindings returns a PrivateSecretBindings
func newPrivateSecretBindings(c *GardenClient, namespace string) *privateSecretBindings {
	return &privateSecretBindings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the privateSecretBinding, and returns the corresponding privateSecretBinding object, and an error if there is any.
func (c *privateSecretBindings) Get(name string, options v1.GetOptions) (result *garden.PrivateSecretBinding, err error) {
	result = &garden.PrivateSecretBinding{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("privatesecretbindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PrivateSecretBindings that match those selectors.
func (c *privateSecretBindings) List(opts v1.ListOptions) (result *garden.PrivateSecretBindingList, err error) {
	result = &garden.PrivateSecretBindingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("privatesecretbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested privateSecretBindings.
func (c *privateSecretBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("privatesecretbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a privateSecretBinding and creates it.  Returns the server's representation of the privateSecretBinding, and an error, if there is any.
func (c *privateSecretBindings) Create(privateSecretBinding *garden.PrivateSecretBinding) (result *garden.PrivateSecretBinding, err error) {
	result = &garden.PrivateSecretBinding{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("privatesecretbindings").
		Body(privateSecretBinding).
		Do().
		Into(result)
	return
}

// Update takes the representation of a privateSecretBinding and updates it. Returns the server's representation of the privateSecretBinding, and an error, if there is any.
func (c *privateSecretBindings) Update(privateSecretBinding *garden.PrivateSecretBinding) (result *garden.PrivateSecretBinding, err error) {
	result = &garden.PrivateSecretBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("privatesecretbindings").
		Name(privateSecretBinding.Name).
		Body(privateSecretBinding).
		Do().
		Into(result)
	return
}

// Delete takes name of the privateSecretBinding and deletes it. Returns an error if one occurs.
func (c *privateSecretBindings) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("privatesecretbindings").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *privateSecretBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("privatesecretbindings").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched privateSecretBinding.
func (c *privateSecretBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *garden.PrivateSecretBinding, err error) {
	result = &garden.PrivateSecretBinding{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("privatesecretbindings").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
