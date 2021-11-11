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
	"github.com/gardener/gardener/charts"
	exports "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/exports"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	admissionconfighelper "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/helper"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfighelper "github.com/gardener/gardener/pkg/controllermanager/apis/config/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	schedulerconfighelper "github.com/gardener/gardener/pkg/scheduler/apis/config/helper"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
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

	// admissionControllerConfig is the parsed configuration of the Gardener Admission Controller
	admissionControllerConfig *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration

	// controllerManagerConfig is the parsed configuration of the Gardener Controller Manager
	controllerManagerConfig *controllermanagerconfigv1alpha1.ControllerManagerConfiguration

	// schedulerConfig is the parsed configuration of the Gardener Scheduler
	schedulerConfig *schedulerconfigv1alpha1.SchedulerConfiguration

	// ImageReferences contains the image references for the control plane component
	// parsed  from the component descriptor
	ImageReferences ImageReferences

	// VirtualGardenClusterEndpoint is the cluster-internal endpoint of the Virtual Garden API server
	// Used when generating the kubeconfigs for the control plane components
	VirtualGardenClusterEndpoint *string

	// VirtualGardenKubeconfigGardenerAPIServer is the generated Kubeconfig for the Gardener API Server
	// Only generated when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerAPIServer *string

	// VirtualGardenKubeconfigGardenerControllerManager is the generated Kubeconfig for the Gardener Controller Manager
	// Only generated when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerControllerManager *string

	// VirtualGardenKubeconfigGardenerScheduler is the generated Kubeconfig for the Gardener Scheduler
	// Only generated when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerScheduler *string

	// VirtualGardenKubeconfigGardenerAdmissionController is the generated Kubeconfig for the Gardener Admission Controller
	// Only generated when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerAdmissionController *string

	// exports will be filled during the reconciliation
	// with data to be exported to other components
	exports exports.Exports

	// chartPath is the path on the local filesystem to the chart directory
	// exposed for testing
	chartPath string
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
		chartPath:           charts.Path,
	}

	if err := op.parseComponentDescriptor(componentDescriptorPath); err != nil {
		return nil, err
	}

	var err error
	if imports.GardenerAdmissionController.Enabled &&
		imports.GardenerAdmissionController.ComponentConfiguration != nil {
		// imports.GardenerAdmissionController.Config.Configuration != nil {

		op.admissionControllerConfig, err = admissionconfighelper.ConvertAdmissionControllerConfigurationExternal(imports.GardenerAdmissionController.ComponentConfiguration.Config)
		if err != nil {
			return nil, fmt.Errorf("could not convert to external admission controller configuration: %v", err)
		}
	}

	if imports.GardenerControllerManager.ComponentConfiguration != nil &&
		imports.GardenerControllerManager.ComponentConfiguration.Config != nil {

		op.controllerManagerConfig, err = controllermanagerconfighelper.ConvertControllerManagerConfigurationExternal(imports.GardenerControllerManager.ComponentConfiguration.Config)
		if err != nil {
			return nil, fmt.Errorf("could not convert to external controller manager configuration: %v", err)
		}
	}

	if imports.GardenerScheduler != nil &&
		imports.GardenerControllerManager.ComponentConfiguration != nil &&
		imports.GardenerControllerManager.ComponentConfiguration.Config != nil {

		op.schedulerConfig, err = schedulerconfighelper.ConvertSchedulerConfigurationExternal(imports.GardenerScheduler.ComponentConfiguration.Config)
		if err != nil {
			return nil, fmt.Errorf("could not convert to external scheduler configuration: %v", err)
		}
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
