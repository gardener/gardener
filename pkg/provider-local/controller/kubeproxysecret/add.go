// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubeproxysecret

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
)

const (
	// ControllerName is the name of the controller.
	ControllerName = "kubeproxysecret"

	labelKeyComponent   = "component"
	labelValueKubeProxy = "kube-proxy"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{Controller: controller.Options{MaxConcurrentReconciles: 1}}
)

// AddOptions are options to apply when adding the kube-proxy secret controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
}

// AddToManagerWithOptions adds a controller with the given options to the given manager.
func AddToManagerWithOptions(_ context.Context, mgr manager.Manager, opts AddOptions) error {
	reconciler := &Reconciler{
		Client:  mgr.GetClient(),
		Decoder: serializer.NewCodecFactory(kubernetes.ShootScheme).UniversalDeserializer(),
		Codec:   kubeproxy.NewConfigCodec(),
	}

	return builder.ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Secret{}, builder.WithPredicates(kubeProxySecretPredicate())).
		WithOptions(opts.Controller).
		Complete(reconciler)
}

// AddToManager adds a controller with the default options to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}

// kubeProxySecretPredicate only reacts to the kube-proxy ManagedResource secrets, which carry the
// "component: kube-proxy" label set by getManagedResourceLabels in pkg/component/kubernetes/proxy.
func kubeProxySecretPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()[labelKeyComponent] == labelValueKubeProxy
	})
}
