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

package namespace

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ProjectNamespace"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Namespace contains listers and and admission handler.
type Namespace struct {
	*admission.Handler
	kubeClient      kubernetes.Interface
	projectLister   gardenlisters.ProjectLister
	namespaceLister kubecorev1listers.NamespaceLister
	readyFunc       admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&Namespace{})
	_ = admissioninitializer.WantsKubeClientset(&Namespace{})
	_ = admissioninitializer.WantsKubeInformerFactory(&Namespace{})

	readyFuncs = []admission.ReadyFunc{}

	// MissingNamespaceWait is the time how long to wait for a missing namespace before re-checking the cache
	// (and then doing a live lookup).
	MissingNamespaceWait = 50 * time.Millisecond
)

// New creates a new Namespace admission plugin.
func New() (*Namespace, error) {
	return &Namespace{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (n *Namespace) AssignReadyFunc(f admission.ReadyFunc) {
	n.readyFunc = f
	n.SetReadyFunc(f)
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (n *Namespace) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	projectInformer := f.Garden().InternalVersion().Projects()
	n.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, projectInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (n *Namespace) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	namespaceInformer := f.Core().V1().Namespaces()
	n.namespaceLister = namespaceInformer.Lister()

	readyFuncs = append(readyFuncs, namespaceInformer.Informer().HasSynced)
}

// SetKubeClientset gets the clientset from the Kubernetes client.
func (n *Namespace) SetKubeClientset(c kubernetes.Interface) {
	n.kubeClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (n *Namespace) ValidateInitialization() error {
	if n.projectLister == nil {
		return errors.New("missing project lister")
	}
	if n.namespaceLister == nil {
		return errors.New("missing namespace lister")
	}
	return nil
}

// Admit creates the namespace that belongs to a Project. It also checks that no other project already
// references the namespace.
func (n *Namespace) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if n.readyFunc == nil {
		n.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !n.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != garden.Kind("Project") {
		return nil
	}
	// Ignore all operations other than CREATE
	if a.GetOperation() != admission.Create {
		return nil
	}

	project, ok := a.GetObject().(*garden.Project)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Project object")
	}

	// Project wants us to create the namespace.
	if project.Spec.Namespace == nil {
		// TODO: Remove the labels and annotations once all clients have caught up.
		annotations := map[string]string{
			common.ProjectOwner: project.Spec.Owner.Name,
		}

		if project.Spec.Purpose != nil {
			annotations[common.ProjectPurpose] = *project.Spec.Purpose
		}
		if project.Spec.Description != nil {
			annotations[common.ProjectDescription] = *project.Spec.Description
		}

		namespace, err := n.kubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName:    fmt.Sprintf("%s%s", common.ProjectPrefix, project.Name),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(project, gardenv1beta1.SchemeGroupVersion.WithKind("Project"))},
				Labels: map[string]string{
					common.GardenRole:  common.GardenRoleProject,
					common.ProjectName: project.Name,
				},
				Annotations: annotations,
			},
		})
		if err != nil {
			return err
		}

		project.Spec.Namespace = &namespace.Name
		return nil
	}

	// Project explicitly specified a namespace.
	namespace, err := n.lookupNamespace(*project.Spec.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierrors.NewBadRequest(fmt.Sprintf("referenced namespace %s does not exist", *project.Spec.Namespace))
		}
		return err
	}

	if val, ok := namespace.Labels[common.GardenRole]; !ok || val != common.GardenRoleProject {
		return admission.NewForbidden(a, fmt.Errorf("namespace needs project label %s=%s", common.GardenRole, common.GardenRoleProject))
	}

	projectList, err := n.projectLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, existingProject := range projectList {
		if existingProject.Spec.Namespace != nil && *existingProject.Spec.Namespace == namespace.Name {
			return admission.NewForbidden(a, fmt.Errorf("another project already references the same namespace %s", namespace.Name))
		}
	}

	return nil
}

func (n *Namespace) lookupNamespace(name string) (*corev1.Namespace, error) {
	// First try to detect the namespace in the cache.
	namespace, err := n.namespaceLister.Get(name)
	if err == nil {
		return namespace, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Second try to detect the namespace in the cache after the first try failed.
	// Give the cache time to observe the namespace before rejecting a create.
	// This helps when creating a namespace and immediately creating a Project referencing it.
	time.Sleep(MissingNamespaceWait)
	namespace, err = n.namespaceLister.Get(name)
	switch {
	case apierrors.IsNotFound(err):
		// no-op
	case err != nil:
		return namespace, err
	default:
		return namespace, nil
	}

	// Third try to detect the secret, now by doing a live lookup instead of relying on the cache.
	return n.kubeClient.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
}
