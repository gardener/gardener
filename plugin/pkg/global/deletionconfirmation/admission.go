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
	"sync"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	"github.com/gardener/gardener/pkg/client/garden/clientset/internalversion"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"

	multierror "github.com/hashicorp/go-multierror"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "DeletionConfirmation"
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
	gardenClient  internalversion.Interface
	shootLister   gardenlisters.ShootLister
	projectLister gardenlisters.ProjectLister
	readyFunc     admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&DeletionConfirmation{})
	_ = admissioninitializer.WantsInternalGardenClientset(&DeletionConfirmation{})

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

	projectInformer := f.Garden().InternalVersion().Projects()
	d.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced, projectInformer.Informer().HasSynced)
}

// SetInternalGardenClientset gets the clientset from the Kubernetes client.
func (d *DeletionConfirmation) SetInternalGardenClientset(c internalversion.Interface) {
	d.gardenClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (d *DeletionConfirmation) ValidateInitialization() error {
	if d.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if d.projectLister == nil {
		return errors.New("missing project lister")
	}
	return nil
}

// Validate makes admissions decisions based on deletion confirmation annotation.
func (d *DeletionConfirmation) Validate(a admission.Attributes, o admission.ObjectInterfaces) error {
	var (
		obj         metav1.Object
		listFunc    func() ([]metav1.Object, error)
		cacheLookup func() (metav1.Object, error)
		liveLookup  func() (metav1.Object, error)
		checkFunc   func(metav1.Object) error

		objectGroupKind = a.GetKind().GroupKind()
		kindShoot       = garden.Kind("Shoot")
		kindProject     = garden.Kind("Project")
	)

	// Ignore all kinds other than Shoot or Project.
	// TODO: in future the Kinds should be configurable
	// https://v1-9.docs.kubernetes.io/docs/admin/admission-controllers/#imagepolicywebhook
	switch objectGroupKind {
	case kindShoot:
		listFunc = func() ([]metav1.Object, error) {
			list, err := d.shootLister.Shoots(a.GetNamespace()).List(labels.Everything())
			if err != nil {
				return nil, err
			}
			result := make([]metav1.Object, 0, len(list))
			for _, obj := range list {
				result = append(result, obj)
			}
			return result, nil
		}
		cacheLookup = func() (metav1.Object, error) {
			return d.shootLister.Shoots(a.GetNamespace()).Get(a.GetName())
		}
		liveLookup = func() (metav1.Object, error) {
			return d.gardenClient.Garden().Shoots(a.GetNamespace()).Get(a.GetName(), metav1.GetOptions{})
		}
		checkFunc = func(obj metav1.Object) error {
			if shootIgnored(obj) {
				return fmt.Errorf("cannot delete shoot if %s annotation is set", common.ShootIgnore)
			}
			return checkIfDeletionIsConfirmed(obj)
		}

	case kindProject:
		listFunc = func() ([]metav1.Object, error) {
			list, err := d.projectLister.List(labels.Everything())
			if err != nil {
				return nil, err
			}
			result := make([]metav1.Object, 0, len(list))
			for _, obj := range list {
				result = append(result, obj)
			}
			return result, nil
		}
		cacheLookup = func() (metav1.Object, error) {
			return d.projectLister.Get(a.GetName())
		}
		liveLookup = func() (metav1.Object, error) {
			return d.gardenClient.Garden().Projects().Get(a.GetName(), metav1.GetOptions{})
		}
		checkFunc = checkIfDeletionIsConfirmed

	default:
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

	// The "delete endpoint" handler of the k8s.io/apiserver library calls the admission controllers
	// handling DELETECOLLECTION requests with empty resource names:
	// https://github.com/kubernetes/apiserver/blob/kubernetes-1.12.1/pkg/endpoints/handlers/delete.go#L265-L283
	// Consequently, a.GetName() equals "". This is for the admission controllers to know that all
	// resources of this kind shall be deleted. We only allow this request if all objects have been
	// properly annotated with the deletion confirmation.
	if a.GetName() == "" {
		objList, err := listFunc()
		if err != nil {
			return err
		}

		var (
			wg     sync.WaitGroup
			result error
			output = make(chan error)
		)

		for _, obj := range objList {
			wg.Add(1)

			go func(obj metav1.Object) {
				defer wg.Done()
				output <- d.Validate(admission.NewAttributesRecord(a.GetObject(), a.GetOldObject(), a.GetKind(), a.GetNamespace(), obj.GetName(), a.GetResource(), a.GetSubresource(), a.GetOperation(), a.IsDryRun(), a.GetUserInfo()), o)
			}(obj)
		}

		go func() {
			wg.Wait()
			close(output)
		}()

		for out := range output {
			if out != nil {
				result = multierror.Append(result, out)
			}
		}

		if result == nil {
			return nil
		}
		return admission.NewForbidden(a, result)
	}

	// Read the object from the cache
	obj, err := cacheLookup()
	if err == nil {
		if checkFunc(obj) == nil {
			return nil
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	// If the first try does not succeed we do a live lookup to really ensure that the deletion cannot be processed
	// (similar to what we do in the ResourceReferenceManager when ensuring the existence of a secret).
	// This is to allow clients to send annotate+delete requests subsequently very fast.
	obj, err = liveLookup()
	if err != nil {
		return err
	}

	if err := checkFunc(obj); err != nil {
		return admission.NewForbidden(a, err)
	}
	return nil
}

func checkIfDeletionIsConfirmed(obj metav1.Object) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return annotationRequiredError()
	}
	if present, _ := strconv.ParseBool(annotations[common.ConfirmationDeletion]); !present {
		return annotationRequiredError()
	}
	return nil
}

func shootIgnored(obj metav1.Object) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	ignore := false
	if value, ok := annotations[common.ShootIgnore]; ok {
		ignore, _ = strconv.ParseBool(value)
	}
	return ignore
}

func annotationRequiredError() error {
	return fmt.Errorf("must have a %q annotation to delete", common.ConfirmationDeletion)
}
