// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	context "context"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	scheme "github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// ControllerRegistrationsGetter has a method to return a ControllerRegistrationInterface.
// A group's client should implement this interface.
type ControllerRegistrationsGetter interface {
	ControllerRegistrations() ControllerRegistrationInterface
}

// ControllerRegistrationInterface has methods to work with ControllerRegistration resources.
type ControllerRegistrationInterface interface {
	Create(ctx context.Context, controllerRegistration *corev1beta1.ControllerRegistration, opts v1.CreateOptions) (*corev1beta1.ControllerRegistration, error)
	Update(ctx context.Context, controllerRegistration *corev1beta1.ControllerRegistration, opts v1.UpdateOptions) (*corev1beta1.ControllerRegistration, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*corev1beta1.ControllerRegistration, error)
	List(ctx context.Context, opts v1.ListOptions) (*corev1beta1.ControllerRegistrationList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *corev1beta1.ControllerRegistration, err error)
	ControllerRegistrationExpansion
}

// controllerRegistrations implements ControllerRegistrationInterface
type controllerRegistrations struct {
	*gentype.ClientWithList[*corev1beta1.ControllerRegistration, *corev1beta1.ControllerRegistrationList]
}

// newControllerRegistrations returns a ControllerRegistrations
func newControllerRegistrations(c *CoreV1beta1Client) *controllerRegistrations {
	return &controllerRegistrations{
		gentype.NewClientWithList[*corev1beta1.ControllerRegistration, *corev1beta1.ControllerRegistrationList](
			"controllerregistrations",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *corev1beta1.ControllerRegistration { return &corev1beta1.ControllerRegistration{} },
			func() *corev1beta1.ControllerRegistrationList { return &corev1beta1.ControllerRegistrationList{} },
		),
	}
}
