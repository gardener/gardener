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

// ControllerRegistrationsGetter has a method to return a ControllerRegistrationInterface.
// A group's client should implement this interface.
type ControllerRegistrationsGetter interface {
	ControllerRegistrations() ControllerRegistrationInterface
}

// ControllerRegistrationInterface has methods to work with ControllerRegistration resources.
type ControllerRegistrationInterface interface {
	Create(ctx context.Context, controllerRegistration *core.ControllerRegistration, opts v1.CreateOptions) (*core.ControllerRegistration, error)
	Update(ctx context.Context, controllerRegistration *core.ControllerRegistration, opts v1.UpdateOptions) (*core.ControllerRegistration, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*core.ControllerRegistration, error)
	List(ctx context.Context, opts v1.ListOptions) (*core.ControllerRegistrationList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.ControllerRegistration, err error)
	ControllerRegistrationExpansion
}

// controllerRegistrations implements ControllerRegistrationInterface
type controllerRegistrations struct {
	client rest.Interface
}

// newControllerRegistrations returns a ControllerRegistrations
func newControllerRegistrations(c *CoreClient) *controllerRegistrations {
	return &controllerRegistrations{
		client: c.RESTClient(),
	}
}

// Get takes name of the controllerRegistration, and returns the corresponding controllerRegistration object, and an error if there is any.
func (c *controllerRegistrations) Get(ctx context.Context, name string, options v1.GetOptions) (result *core.ControllerRegistration, err error) {
	result = &core.ControllerRegistration{}
	err = c.client.Get().
		Resource("controllerregistrations").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ControllerRegistrations that match those selectors.
func (c *controllerRegistrations) List(ctx context.Context, opts v1.ListOptions) (result *core.ControllerRegistrationList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &core.ControllerRegistrationList{}
	err = c.client.Get().
		Resource("controllerregistrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested controllerRegistrations.
func (c *controllerRegistrations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("controllerregistrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a controllerRegistration and creates it.  Returns the server's representation of the controllerRegistration, and an error, if there is any.
func (c *controllerRegistrations) Create(ctx context.Context, controllerRegistration *core.ControllerRegistration, opts v1.CreateOptions) (result *core.ControllerRegistration, err error) {
	result = &core.ControllerRegistration{}
	err = c.client.Post().
		Resource("controllerregistrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(controllerRegistration).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a controllerRegistration and updates it. Returns the server's representation of the controllerRegistration, and an error, if there is any.
func (c *controllerRegistrations) Update(ctx context.Context, controllerRegistration *core.ControllerRegistration, opts v1.UpdateOptions) (result *core.ControllerRegistration, err error) {
	result = &core.ControllerRegistration{}
	err = c.client.Put().
		Resource("controllerregistrations").
		Name(controllerRegistration.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(controllerRegistration).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the controllerRegistration and deletes it. Returns an error if one occurs.
func (c *controllerRegistrations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("controllerregistrations").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *controllerRegistrations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("controllerregistrations").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched controllerRegistration.
func (c *controllerRegistrations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.ControllerRegistration, err error) {
	result = &core.ControllerRegistration{}
	err = c.client.Patch(pt).
		Resource("controllerregistrations").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
