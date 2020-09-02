// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shootstate

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/core"
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	externalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	corev1alpha1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootStateDeletionValidator"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// ValidateShootStateDeletion contains listers and admission handler.
type ValidateShootStateDeletion struct {
	*admission.Handler
	shootStateLister corev1alpha1listers.ShootStateLister
	coreClient       coreclientset.Interface
	readyFunc        admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsExternalCoreInformerFactory(&ValidateShootStateDeletion{})
	_ = admissioninitializer.WantsInternalCoreClientset(&ValidateShootStateDeletion{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ShootStateDeletion admission plugin.
func New() (*ValidateShootStateDeletion, error) {
	return &ValidateShootStateDeletion{
		Handler: admission.NewHandler(admission.Delete),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (d *ValidateShootStateDeletion) AssignReadyFunc(f admission.ReadyFunc) {
	d.readyFunc = f
	d.SetReadyFunc(f)
}

// SetExternalCoreInformerFactory sets the external garden core informer factory.
func (d *ValidateShootStateDeletion) SetExternalCoreInformerFactory(f externalcoreinformers.SharedInformerFactory) {
	shootStateInformer := f.Core().V1alpha1().ShootStates()
	d.shootStateLister = shootStateInformer.Lister()

	readyFuncs = append(readyFuncs, shootStateInformer.Informer().HasSynced)
}

// SetInternalCoreClientset sets the garden core clientset.
func (d *ValidateShootStateDeletion) SetInternalCoreClientset(c coreclientset.Interface) {
	d.coreClient = c
}

func (d *ValidateShootStateDeletion) waitUntilReady(attrs admission.Attributes) error {
	// Wait until the caches have been synced
	if d.readyFunc == nil {
		d.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}

	if !d.WaitForReady() {
		return admission.NewForbidden(attrs, errors.New("not yet ready to handle request"))
	}

	return nil
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (d *ValidateShootStateDeletion) ValidateInitialization() error {
	if d.shootStateLister == nil {
		return errors.New("missing ShootState lister")
	}

	if d.coreClient == nil {
		return errors.New("missing garden core client")
	}

	return nil
}

var _ admission.ValidationInterface = &ValidateShootStateDeletion{}

// Validate makes admissions decisions based on deletion confirmation annotation.
func (d *ValidateShootStateDeletion) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	if a.GetKind().GroupKind() != core.Kind("ShootState") {
		return nil
	}

	if err := d.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready %v", err)
	}

	if a.GetName() == "" {
		return d.validateDeleteCollection(ctx, a)
	}

	return d.validateDelete(ctx, a)
}

func (d *ValidateShootStateDeletion) validateDeleteCollection(ctx context.Context, attrs admission.Attributes) error {
	shootStateList, err := d.shootStateLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, shootState := range shootStateList {
		if err := d.validateDelete(ctx, d.createAttributesWithName(shootState.Name, attrs)); err != nil {
			return err
		}
	}

	return nil
}

func (d *ValidateShootStateDeletion) validateDelete(ctx context.Context, attrs admission.Attributes) error {
	if _, err := d.coreClient.Core().Shoots(attrs.GetNamespace()).Get(ctx, attrs.GetName(), kubernetes.DefaultGetOptions()); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error retrieving Shoot %s in namespace %s: %v", attrs.GetName(), attrs.GetNamespace(), err)
	}
	return admission.NewForbidden(attrs, fmt.Errorf("shoot %s in namespace %s still exists", attrs.GetName(), attrs.GetNamespace()))
}

func (d *ValidateShootStateDeletion) createAttributesWithName(objName string, a admission.Attributes) admission.Attributes {
	return admission.NewAttributesRecord(a.GetObject(),
		a.GetOldObject(),
		a.GetKind(),
		a.GetNamespace(),
		objName,
		a.GetResource(),
		a.GetSubresource(),
		a.GetOperation(),
		a.GetOperationOptions(),
		a.IsDryRun(),
		a.GetUserInfo())
}
