// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"
	"os"
	"strings"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/apis/v2/cdutils"
	"github.com/gardener/component-spec/bindings-go/codec"
	exports "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/exports"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Interface is an interface for the operation.
type Interface interface {
	// Reconcile performs a reconcile operation.
	Reconcile(context.Context) (*exports.Exports, error)
	// Delete performs a delete operation.
	Delete(context.Context) error
}

const (
	gardenerComponentName                = "github.com/gardener/gardener"
	gardenerAPIServerImageName           = "apiserver"
	gardenerControllerManagerImageName   = "controller-manager"
	gardenerSchedulerImageName           = "scheduler"
	gardenerAdmissionControllerImageName = "admission-controller"
	prefix                               = "controlplane"
)

// operation contains the configuration for an operation.
type operation struct {
	// client is the Kubernetes client for the hosting cluster.
	client client.Client

	// runtimeClient is the client for the runtime cluster
	runtimeClient kubernetes.Interface

	// runtimeClient is the client for the virtual-garden cluster
	virtualGardenClient *kubernetes.Interface

	// log is a logger.
	log logrus.FieldLogger

	// namespace is the namespace in the runtime cluster into which the controlplane shall be deployed.
	namespace string

	// imports contains the imports configuration.
	imports *imports.Imports

	// ImageReferences contains the image references for the control plane component
	// parsed  from the component descriptor
	ImageReferences ImageReferences

	// exports will be filled during the reconciliation
	// with data to be exported to other components
	exports exports.Exports
}

// ImageReferences contains the parsed image from the component descriptor
// for all control plane components
type ImageReferences struct {
	ApiServer           Image
	ControllerManager   Image
	AdmissionController Image
	Scheduler           Image
}

// Image represents an OCI image in a registry
type Image struct {
	Repository string
	Tag        string
}

// NewOperation returns a new operation structure that implements Interface.
func NewOperation(
	runtimeCLient kubernetes.Interface,
	virtualGardenClient *kubernetes.Interface,
	log *logrus.Logger,
	imports *imports.Imports,
	componentDescriptorPath string,
) (Interface, error) {
	op := &operation{
		runtimeClient:       runtimeCLient,
		virtualGardenClient: virtualGardenClient,
		log:                 log,
		imports:             imports,
	}

	if err := op.parseComponentDescriptor(componentDescriptorPath); err != nil {
		return nil, err
	}

	return op, nil
}

func (o *operation) parseComponentDescriptor(componentDescriptorPath string) error {
	componentDescriptorData, err := os.ReadFile(componentDescriptorPath)
	if err != nil {
		return fmt.Errorf("failed to parse the Gardenlet component descriptor: %w", err)
	}

	componentDescriptorList := &v2.ComponentDescriptorList{}
	err = codec.Decode(componentDescriptorData, componentDescriptorList)
	if err != nil {
		return fmt.Errorf("failed to parse the Gardenlet component descriptor: %w", err)
	}

	o.ImageReferences.ApiServer.Repository, o.ImageReferences.ApiServer.Tag, err = getImageReference(componentDescriptorList, gardenerAPIServerImageName)
	if err != nil {
		return fmt.Errorf("failed to get the Gardener API server image from the component descriptor: %w", err)
	}

	o.ImageReferences.ControllerManager.Repository, o.ImageReferences.ControllerManager.Tag, err = getImageReference(componentDescriptorList, gardenerControllerManagerImageName)
	if err != nil {
		return fmt.Errorf("failed to get the Gardener APIserver image from the component descriptor: %w", err)
	}

	o.ImageReferences.Scheduler.Repository, o.ImageReferences.Scheduler.Tag, err = getImageReference(componentDescriptorList, gardenerSchedulerImageName)
	if err != nil {
		return fmt.Errorf("failed to get the Gardener APIserver image from the component descriptor: %w", err)
	}

	o.ImageReferences.AdmissionController.Repository, o.ImageReferences.AdmissionController.Tag, err = getImageReference(componentDescriptorList, gardenerAdmissionControllerImageName)
	if err != nil {
		return fmt.Errorf("failed to get the Gardener APIserver image from the component descriptor: %w", err)
	}

	return nil
}

func getImageReference(componentDescriptorList *v2.ComponentDescriptorList, imageName string) (string, string, error) {
	imageReference, err := cdutils.GetImageReferenceFromList(componentDescriptorList, gardenerComponentName, imageName)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse the component descriptor: %w", err)
	}

	repo, tag, _, err := cdutils.ParseImageReference(imageReference)
	if err != nil {
		return "", "", err
	}
	return repo, tag, nil
}

func (o *operation) progressReporter(_ context.Context, stats *flow.Stats) {
	if stats.ProgressPercent() == 0 && stats.Running.Len() == 0 {
		return
	}

	executionNow := ""
	if stats.Running.Len() > 0 {
		executionNow = fmt.Sprintf(" - Executing now: %q", strings.Join(stats.Running.StringList(), ", "))
	}
	o.log.Infof("%d%% of all tasks completed (%d/%d)%s", stats.ProgressPercent(), stats.Failed.Len()+stats.Succeeded.Len(), stats.All.Len(), executionNow)
}

func (o *operation) getGardenClient() kubernetes.Interface {
	if o.virtualGardenClient != nil {
		return *o.virtualGardenClient
	}
	return o.runtimeClient
}
