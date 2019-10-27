/*
Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var hvpalog = logf.Log.WithName("hvpa-resource")

// SetupWebhookWithManager sets up manager with a new webhook and r as the reconcile.Reconciler
func (r *Hvpa) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-autoscaling-k8s-io-v1alpha1-hvpa,mutating=true,failurePolicy=fail,groups=autoscaling.k8s.io,resources=hvpas,verbs=create;update,versions=v1alpha1,name=mhvpa.kb.io

var _ webhook.Defaulter = &Hvpa{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Hvpa) Default() {
	hvpalog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// +kubebuilder:webhook:path=/validate-autoscaling-k8s-io-v1alpha1-hvpa,mutating=false,failurePolicy=fail,groups=autoscaling.k8s.io,resources=hvpas,verbs=create;update,versions=v1alpha1,name=vhvpa.kb.io

var _ webhook.Validator = &Hvpa{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Hvpa) ValidateCreate() error {
	hvpalog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Hvpa) ValidateUpdate(old runtime.Object) error {
	hvpalog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Hvpa) ValidateDelete() error {
	hvpalog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}
