// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package deletionconfirmation

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootDeletionConfirmation"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// DeletionConfirmation contains an admission handler and listers.
type DeletionConfirmation struct {
	*admission.Handler
	shootLister gardenlisters.ShootLister
	readyFunc   admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&DeletionConfirmation{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new DeletionConfirmation admission plugin.
func New() (*DeletionConfirmation, error) {
	return &DeletionConfirmation{
		Handler: admission.NewHandler(admission.Delete),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (d *DeletionConfirmation) AssignReadyFunc(f admission.ReadyFunc) {
	d.readyFunc = f
	d.SetReadyFunc(f)
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (d *DeletionConfirmation) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	shootInformer := f.Garden().InternalVersion().Shoots()
	d.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (d *DeletionConfirmation) ValidateInitialization() error {
	if d.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

// Admit makes admissions decisions based on deletion confirmation annotation.
func (d *DeletionConfirmation) Admit(a admission.Attributes) error {
	// Ignore all kinds other than Shoot.
	// TODO: in future the Kinds should be configurable
	// https://v1-9.docs.kubernetes.io/docs/admin/admission-controllers/#imagepolicywebhook
	if a.GetKind().GroupKind() != garden.Kind("Shoot") {
		return nil
	}

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
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	shoot, err := d.shootLister.Shoots(a.GetNamespace()).Get(a.GetName())
	if err != nil {
		return err
	}

	annotations := shoot.GetAnnotations()
	if annotations == nil {
		return admission.NewForbidden(a, annotationRequiredError())
	}
	if present, _ := strconv.ParseBool(annotations[common.ConfirmationDeletion]); !present {
		return admission.NewForbidden(a, annotationRequiredError())
	}
	return nil
}

func annotationRequiredError() error {
	return fmt.Errorf("must have a %q annotation to delete", common.ConfirmationDeletion)
}
