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

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	importsv1alpha1 "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1"
	importvalidation "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	controlplanecontroller "github.com/gardener/gardener/landscaper/pkg/controlplane/controller"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/util"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	landscaperconstants "github.com/gardener/landscaper/apis/deployer/container"
	"github.com/sirupsen/logrus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	admissioncontrollerconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	controllermanagerconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	schedulerconfig "github.com/gardener/gardener/pkg/scheduler/apis/config"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/component-base/version/verflag"
)

// NewCommandStartLandscaperControlplane creates a *cobra.Command object with default parameters
func NewCommandStartLandscaperControlplane(ctx context.Context) *cobra.Command {
	opts := NewOptions()

	cmd := &cobra.Command{
		Use:   "landscaper-controlplane",
		Short: "Launch the landscaper component for the controlplane.",
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			log := &logrus.Logger{
				Out:   os.Stderr,
				Level: logrus.InfoLevel,
				Formatter: &logrus.TextFormatter{
					DisableColors: true,
				},
			}

			utilruntime.Must(opts.InitializeFromEnvironment())
			utilruntime.Must(opts.validate())

			if err := run(ctx, opts, log); err != nil {
				panic(err)
			}

			log.Infof("Execution finished successfully.")
			return nil
		},
	}
	return cmd
}

func run(ctx context.Context, opts *Options, log *logrus.Logger) error {
	imports, err := loadImportsFromFile(opts.ImportsPath)
	if err != nil {
		return fmt.Errorf("unable to load landscaper imports: %w", err)
	}

	if errs := importvalidation.ValidateLandscaperImports(imports); len(errs) > 0 {
		return fmt.Errorf("errors validating the landscaper imports: %+v", errs)
	}

	runtimeTargetConfig := &landscaperv1alpha1.KubernetesClusterTargetConfig{}
	if err := json.Unmarshal(imports.RuntimeCluster.Spec.Configuration.RawMessage, runtimeTargetConfig); err != nil {
		return fmt.Errorf("failed to parse the Runtime cluster kubeconfig : %w", err)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(corescheme.AddToScheme(scheme))
	utilruntime.Must(hvpav1alpha1.AddToScheme(scheme))
	utilruntime.Must(autoscalingv1beta2.AddToScheme(scheme))
	utilruntime.Must(apiregistrationv1.AddToScheme(scheme))

	// Create Runtime client
	runtimeClient, err := kubernetes.NewClientFromBytes([]byte(runtimeTargetConfig.Kubeconfig), kubernetes.WithClientOptions(
		client.Options{
			Scheme: scheme,
		}))
	if err != nil {
		return fmt.Errorf("failed to create the Runtime cluster client: %w", err)
	}

	// Create Virtual Garden client
	var virtualGardenClient *kubernetes.Interface
	if imports.VirtualGarden != nil && imports.VirtualGarden.Enabled {
		gardenClusterTargetConfig := &landscaperv1alpha1.KubernetesClusterTargetConfig{}
		if err := json.Unmarshal(imports.VirtualGarden.Kubeconfig.Spec.Configuration.RawMessage, gardenClusterTargetConfig); err != nil {
			return fmt.Errorf("failed to parse the virtual-garden cluster kubeconfig : %w", err)
		}

		vGardenClient, err := kubernetes.NewClientFromBytes([]byte(gardenClusterTargetConfig.Kubeconfig), kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.GardenScheme,
			}))
		if err != nil {
			return fmt.Errorf("failed to create the Virtual Garden cluster client: %w", err)
		}
		virtualGardenClient = &vGardenClient
	}

	operation, err := controlplanecontroller.NewOperation(runtimeClient, virtualGardenClient, log, imports, opts.ComponentDescriptorPath)
	if err != nil {
		return err
	}
	log.Infof("Initialization of operation complete.")

	if opts.OperationType == landscaperconstants.OperationReconcile {
		exports, err := operation.Reconcile(ctx)
		if err != nil {
			return err
		}

		log.Infof("Writing exports file to EXPORTS_PATH(%s)", opts.ExportsPath)
		err = util.ExportsToFile(exports, opts.ExportsPath)
		if err != nil {
			return err
		}

		return nil
	} else if opts.OperationType == landscaperconstants.OperationDelete {
		return operation.Delete(ctx)
	}
	return fmt.Errorf("unknown operation type: %q", opts.OperationType)
}

// loadImportsFromFile loads the content of file and decodes it as a
// imports object.
func loadImportsFromFile(file string) (*imports.Imports, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	landscaperImport := &imports.Imports{}

	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	if err := imports.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := importsv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	// Adding internal and v1alpha1 config types.
	// Required to parse the component configs
	if err := controllermanagerconfig.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := controllermanagerconfigv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := schedulerconfig.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := schedulerconfigv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := admissioncontrollerconfig.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := admissioncontrollerconfigv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	//  for API server import configuration
	if err := apiserverv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := apiserverconfigv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := auditv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := hvpav1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if _, _, err := codecs.UniversalDecoder().Decode(data, nil, landscaperImport); err != nil {
		return nil, err
	}

	return landscaperImport, nil
}
