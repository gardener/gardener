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

package validator

import (
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/core"
	informers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"

	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "PlantValidator"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// ValidatePlant contains listers and and admission handler.
type ValidatePlant struct {
	*admission.Handler
	coreClient    coreclientset.Interface
	projectLister listers.ProjectLister
	readyFunc     admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreClientset(&ValidatePlant{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ValidatePlant admission plugin.
func New() (*ValidatePlant, error) {
	return &ValidatePlant{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ValidatePlant) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreClientset gets the clientset from the Kubernetes client.
func (v *ValidatePlant) SetInternalCoreClientset(c coreclientset.Interface) {
	v.coreClient = c
}

// SetInternalGardenInformerFactory get the garden informer factory and adds it the validator
func (v *ValidatePlant) SetInternalGardenInformerFactory(i informers.SharedInformerFactory) {
	projectInformer := i.Garden().InternalVersion().Projects()
	v.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, projectInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidatePlant) ValidateInitialization() error {
	return nil
}

// Validate makes admissions decisions based on the resources specified in a Plant object.
// It does reject the request if there another plant managing the cluster, if the plant name is invalid
// or the project that contains the plant resource is deleted
func (v *ValidatePlant) Validate(a admission.Attributes) error {
	// Wait until the caches have been synced
	if v.readyFunc == nil {
		v.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !v.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Plant.
	if a.GetKind().GroupKind() != core.Kind("Plant") {
		return nil
	}
	plant, ok := a.GetObject().(*core.Plant)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Plant object")
	}

	// Live lookup to prevent missing any data
	plantList, err := v.coreClient.Core().Plants(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	project, err := admissionutils.GetProject(plant.Namespace, v.projectLister)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced project: %+v", err.Error()))
	}

	// We don't want new Plants to be created in Projects which were already marked for deletion.
	if project.DeletionTimestamp != nil {
		return admission.NewForbidden(a, fmt.Errorf("cannot create plant '%s' in project '%s' already marked for deletion", plant.Name, project.Name))
	}

	// no two plant resources can be mapped to the same cluster (harder checking can be done by comparing the base64 kubeconfig of the secret)
	for _, obj := range plantList.Items {
		if obj.Name != plant.Name {
			if obj.Spec.SecretRef.Name == plant.Spec.SecretRef.Name && obj.Spec.SecretRef.Namespace == plant.Spec.SecretRef.Namespace {
				return admission.NewForbidden(a, fmt.Errorf("another plant resource already exists that maps to cluster %s", obj.Spec.SecretRef.Name))
			}
		}
	}

	return nil
}
