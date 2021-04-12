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

package plant

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

// Handler contains gardenlisters and and admission handler.
type Handler struct {
	*admission.Handler
	projectLister gardencorelisters.ProjectLister
	plantLister   gardencorelisters.PlantLister
	readyFunc     admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&Handler{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new Handler admission plugin.
func New() (*Handler, error) {
	return &Handler{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (a *Handler) AssignReadyFunc(f admission.ReadyFunc) {
	a.readyFunc = f
	a.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory gets the garden core informer factory and adds it.
func (a *Handler) SetInternalCoreInformerFactory(i gardencoreinformers.SharedInformerFactory) {
	plantsInformer := i.Core().InternalVersion().Plants()
	a.plantLister = plantsInformer.Lister()

	projectInformer := i.Core().InternalVersion().Projects()
	a.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, plantsInformer.Informer().HasSynced, projectInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (a *Handler) ValidateInitialization() error {
	return nil
}

func (a *Handler) waitUntilReady(attrs admission.Attributes) error {
	// Wait until the caches have been synced
	if a.readyFunc == nil {
		a.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !a.WaitForReady() {
		return admission.NewForbidden(attrs, errors.New("not yet ready to handle request"))
	}
	return nil
}

var _ admission.MutationInterface = &Handler{}

// Admit ensures that the plant is correctly annotated
func (a *Handler) Admit(ctx context.Context, attrs admission.Attributes, o admission.ObjectInterfaces) error {
	if err := a.waitUntilReady(attrs); err != nil {
		return err
	}

	// Ignore all kinds other than Plant.
	if attrs.GetKind().GroupKind() != core.Kind("Plant") {
		return nil
	}

	var attrsObj = attrs.GetObject()
	plant, ok := attrsObj.(*core.Plant)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Plant object")
	}

	if admissionutils.SkipVerification(attrs.GetOperation(), plant.ObjectMeta) {
		return nil
	}

	if attrs.GetOperation() == admission.Create {
		metav1.SetMetaDataAnnotation(&plant.ObjectMeta, v1beta1constants.GardenCreatedBy, attrs.GetUserInfo().GetName())
	}

	return nil
}

var _ admission.ValidationInterface = &Handler{}

// Validate makes admissions decisions based on the resources specified in a Plant object.
// It does reject the request if there another plant managing the cluster, if the plant name is invalid
// or the project that contains the plant resource is deleted
func (a *Handler) Validate(ctx context.Context, attrs admission.Attributes, o admission.ObjectInterfaces) error {
	if err := a.waitUntilReady(attrs); err != nil {
		return err
	}

	// Ignore all kinds other than Plant.
	if attrs.GetKind().GroupKind() != core.Kind("Plant") {
		return nil
	}

	var attrsObj = attrs.GetObject()
	plant, ok := attrsObj.(*core.Plant)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Plant object")
	}

	if admissionutils.SkipVerification(attrs.GetOperation(), plant.ObjectMeta) {
		return nil
	}

	return a.validate(plant, attrs)
}

func (a *Handler) validate(plant *core.Plant, attrs admission.Attributes) error {
	plantList, err := a.plantLister.Plants(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return err
	}

	project, err := gutil.ProjectForNamespaceFromInternalLister(a.projectLister, plant.Namespace)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced project: %+v", err.Error()))
	}

	// We don't want new Plants to be created in Projects which were already marked for deletion.
	if project.DeletionTimestamp != nil {
		return admission.NewForbidden(attrs, fmt.Errorf("cannot create plant '%s' in project '%s' already marked for deletion", plant.Name, project.Name))
	}

	// no two plant resources can be mapped to the same cluster (harder checking can be done by comparing the base64 kubeconfig of the secret)
	for _, plantObj := range plantList {
		if plantObj.Name != plant.Name && plantObj.Spec.SecretRef.Name == plant.Spec.SecretRef.Name && plantObj.Namespace == plant.Namespace {
			return admission.NewForbidden(attrs, fmt.Errorf("another plant resource already exists that maps to cluster %s", plantObj.Spec.SecretRef.Name))
		}
	}
	return nil
}
