// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package internalversion

import (
	"context"
	"time"

	core "github.com/gardener/gardener/pkg/apis/core"
	scheme "github.com/gardener/gardener/pkg/client/core/clientset/internalversion/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ShootsGetter has a method to return a ShootInterface.
// A group's client should implement this interface.
type ShootsGetter interface {
	Shoots(namespace string) ShootInterface
}

// ShootInterface has methods to work with Shoot resources.
type ShootInterface interface {
	Create(ctx context.Context, shoot *core.Shoot, opts v1.CreateOptions) (*core.Shoot, error)
	Update(ctx context.Context, shoot *core.Shoot, opts v1.UpdateOptions) (*core.Shoot, error)
	UpdateStatus(ctx context.Context, shoot *core.Shoot, opts v1.UpdateOptions) (*core.Shoot, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*core.Shoot, error)
	List(ctx context.Context, opts v1.ListOptions) (*core.ShootList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.Shoot, err error)
	ShootExpansion
}

// shoots implements ShootInterface
type shoots struct {
	client rest.Interface
	ns     string
}

// newShoots returns a Shoots
func newShoots(c *CoreClient, namespace string) *shoots {
	return &shoots{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the shoot, and returns the corresponding shoot object, and an error if there is any.
func (c *shoots) Get(ctx context.Context, name string, options v1.GetOptions) (result *core.Shoot, err error) {
	result = &core.Shoot{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("shoots").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Shoots that match those selectors.
func (c *shoots) List(ctx context.Context, opts v1.ListOptions) (result *core.ShootList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &core.ShootList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("shoots").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested shoots.
func (c *shoots) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("shoots").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a shoot and creates it.  Returns the server's representation of the shoot, and an error, if there is any.
func (c *shoots) Create(ctx context.Context, shoot *core.Shoot, opts v1.CreateOptions) (result *core.Shoot, err error) {
	result = &core.Shoot{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("shoots").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(shoot).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a shoot and updates it. Returns the server's representation of the shoot, and an error, if there is any.
func (c *shoots) Update(ctx context.Context, shoot *core.Shoot, opts v1.UpdateOptions) (result *core.Shoot, err error) {
	result = &core.Shoot{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("shoots").
		Name(shoot.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(shoot).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *shoots) UpdateStatus(ctx context.Context, shoot *core.Shoot, opts v1.UpdateOptions) (result *core.Shoot, err error) {
	result = &core.Shoot{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("shoots").
		Name(shoot.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(shoot).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the shoot and deletes it. Returns an error if one occurs.
func (c *shoots) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("shoots").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *shoots) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("shoots").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched shoot.
func (c *shoots) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.Shoot, err error) {
	result = &core.Shoot{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("shoots").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
