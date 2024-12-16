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

// ShootStatesGetter has a method to return a ShootStateInterface.
// A group's client should implement this interface.
type ShootStatesGetter interface {
	ShootStates(namespace string) ShootStateInterface
}

// ShootStateInterface has methods to work with ShootState resources.
type ShootStateInterface interface {
	Create(ctx context.Context, shootState *corev1beta1.ShootState, opts v1.CreateOptions) (*corev1beta1.ShootState, error)
	Update(ctx context.Context, shootState *corev1beta1.ShootState, opts v1.UpdateOptions) (*corev1beta1.ShootState, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*corev1beta1.ShootState, error)
	List(ctx context.Context, opts v1.ListOptions) (*corev1beta1.ShootStateList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *corev1beta1.ShootState, err error)
	ShootStateExpansion
}

// shootStates implements ShootStateInterface
type shootStates struct {
	*gentype.ClientWithList[*corev1beta1.ShootState, *corev1beta1.ShootStateList]
}

// newShootStates returns a ShootStates
func newShootStates(c *CoreV1beta1Client, namespace string) *shootStates {
	return &shootStates{
		gentype.NewClientWithList[*corev1beta1.ShootState, *corev1beta1.ShootStateList](
			"shootstates",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *corev1beta1.ShootState { return &corev1beta1.ShootState{} },
			func() *corev1beta1.ShootStateList { return &corev1beta1.ShootStateList{} },
		),
	}
}
