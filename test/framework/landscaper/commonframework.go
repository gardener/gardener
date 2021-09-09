// Copyright 2021 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package landscaper

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	landscaperhealth "github.com/gardener/gardener/landscaper/utils/health"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	. "github.com/gardener/gardener/test/framework"
)

var landscaperCommonConfigFromFlags = &LandscaperCommonConfig{}

// CommonFrameworkLandscaper is a common framework for the testing
// of landscaper components.
type CommonFrameworkLandscaper struct {
	TestDescription

	Config           *LandscaperCommonConfig
	Logger           *logrus.Entry
	LandscaperClient client.Client

	// ResourceSuffix is a random suffix for landscaper resources created
	// by this integration test
	ResourceSuffix string
}

// ComponentConfig contains all the information to uniquely identify
// a component descriptor in a registry
// landscaper tests create Installation CRDs referencing this component descriptor
// this implies that prior to the test execution, the component descriptor must already exist
// in the registry and must reference a blueprint.
type ComponentConfig struct {
	baseUrl string
	name    string
	version string
}

// LandscaperCommonConfig is the configuration for the landscaper
type LandscaperCommonConfig struct {
	KubeconfigPathLandscaperCluster string
	TargetNamespace                 string
	ComponentConfig                 ComponentConfig
}

// NewLandscaperCommonFramework creates a new landscaper common framework.
func NewLandscaperCommonFramework(cfg *LandscaperCommonConfig) *CommonFrameworkLandscaper {
	suffix, err := utils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	ExpectNoError(err)
	suffix = fmt.Sprintf("tm-%s", suffix)

	return &CommonFrameworkLandscaper{
		TestDescription: NewTestDescription("LANDSCAPER_COMMON"),
		Config:          cfg,
		Logger:          logrus.NewEntry(logrus.New()).WithField("framework", "LANDSCAPER_COMMON"),
		ResourceSuffix:  suffix,
	}
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It merges the landscaper configuration with the configuration from flags
// and creates the landscaper client
func (f *CommonFrameworkLandscaper) BeforeEach() {
	f.Config = mergeLandscaperCommonConfig(f.Config, landscaperCommonConfigFromFlags)

	validateLandscaperCommonConfig(f.Config)

	landscaperScheme := runtime.NewScheme()
	gomega.Expect(landscaperv1alpha1.AddToScheme(landscaperScheme)).ToNot(gomega.HaveOccurred())
	gomega.Expect(corev1.AddToScheme(landscaperScheme)).ToNot(gomega.HaveOccurred())
	client, err := kubernetes.NewClientFromFile("", f.Config.KubeconfigPathLandscaperCluster, kubernetes.WithClientOptions(
		client.Options{
			Scheme: landscaperScheme,
		}),
	)

	ExpectNoError(err)
	f.LandscaperClient = client.Client()
}

// RegisterLandscaperCommonFrameworkFlags adds all flags that are needed to configure a landscaper common framework to the provided flagset.
func RegisterLandscaperCommonFrameworkFlags() *LandscaperCommonConfig {
	newCfg := &LandscaperCommonConfig{}

	flag.StringVar(&newCfg.KubeconfigPathLandscaperCluster, "landscaper-kubecfg-path", "", "the path to the kubeconfig  of the landscaper cluster that will be used for integration tests")
	flag.StringVar(&newCfg.TargetNamespace, "landscaper-target-namespace", "ls-system", "the namespace to create the landscaper resources in")
	flag.StringVar(&newCfg.ComponentConfig.baseUrl, "repository-context-url", "", "the base url of the component descriptor (e.g. eu.gcr.io/gardener-project/landscaper/dev/components)")
	flag.StringVar(&newCfg.ComponentConfig.name, "component-name", "", "the name of the component descriptor (e.g. github.com/gardener/gardener)")
	flag.StringVar(&newCfg.ComponentConfig.version, "component-version", "", "the version of the component descriptor (e.g. 1.19.0)")

	landscaperCommonConfigFromFlags = newCfg

	return landscaperCommonConfigFromFlags
}

func validateLandscaperCommonConfig(cfg *LandscaperCommonConfig) {
	if cfg == nil {
		ginkgo.Fail("no landscaper common framework configuration provided")
	}

	if !StringSet(cfg.KubeconfigPathLandscaperCluster) {
		ginkgo.Fail("Need to set the path to the kubeconfig of the cluster where the landscaper is installed (flag: -landscaper-kubecfg-path)")
	}

	if !StringSet(cfg.ComponentConfig.name) {
		ginkgo.Fail("Need to set the name of the component descriptor (e.g. github.com/gardener/gardener) (flag: componentName) ")
	}

	if !StringSet(cfg.ComponentConfig.baseUrl) {
		ginkgo.Fail("Need to set the base url of the component descriptor (e.g. eu.gcr.io/gardener-project/landscaper/dev/components) (flag: componentBaseUrl) ")
	}

	if !StringSet(cfg.ComponentConfig.version) {
		ginkgo.Fail("Need to set the version of the component descriptor (e.g. 1.19.0) (flag: componentVersion) ")
	}
}

func mergeLandscaperCommonConfig(base, overwrite *LandscaperCommonConfig) *LandscaperCommonConfig {
	if base == nil {
		return overwrite
	}

	if overwrite == nil {
		return base
	}

	if StringSet(overwrite.KubeconfigPathLandscaperCluster) {
		base.KubeconfigPathLandscaperCluster = overwrite.KubeconfigPathLandscaperCluster
	}

	if StringSet(overwrite.ComponentConfig.name) {
		base.ComponentConfig.name = overwrite.ComponentConfig.name
	}

	if StringSet(overwrite.ComponentConfig.version) {
		base.ComponentConfig.version = overwrite.ComponentConfig.version
	}

	if StringSet(overwrite.ComponentConfig.baseUrl) {
		base.ComponentConfig.baseUrl = overwrite.ComponentConfig.baseUrl
	}
	return base
}

// GetInstallation creates and returns a landscaperv1alpha1.Installation for the given name.
func (f *CommonFrameworkLandscaper) GetInstallation(name string) *landscaperv1alpha1.Installation {
	name = fmt.Sprintf("%s-%s", name, f.ResourceSuffix)

	return &landscaperv1alpha1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.Config.TargetNamespace,
		},
		Spec: landscaperv1alpha1.InstallationSpec{
			Blueprint: landscaperv1alpha1.BlueprintDefinition{
				Reference: &landscaperv1alpha1.RemoteBlueprintReference{
					// the component descriptor has to contain the blueprint as a resource
					// with type:blueprint and name: gardenlet-blueprint
					// in the future, this could be made configurable
					ResourceName: "gardenlet-blueprint",
				},
			},
			ComponentDescriptor: &landscaperv1alpha1.ComponentDescriptorDefinition{
				Reference: &landscaperv1alpha1.ComponentDescriptorReference{
					RepositoryContext: &cdv2.RepositoryContext{
						Type:    cdv2.OCIRegistryType,
						BaseURL: f.Config.ComponentConfig.baseUrl,
					},
					ComponentName: f.Config.ComponentConfig.name,
					Version:       f.Config.ComponentConfig.version,
				},
			},
		},
	}
}

// CreateInstallation creates a new installation
func (f *CommonFrameworkLandscaper) createInstallationAndWaitToBecomeHealthy(ctx context.Context, installation *landscaperv1alpha1.Installation) error {
	// Create the installation
	if err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.LandscaperClient.Create(ctx, installation)
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) || apierrors.IsAlreadyExists(err) {
			return retry.SevereError(err)
		}
		if err != nil {
			f.Logger.Infof("Could not create installation %s: %s", installation.Name, err.Error())
			return retry.MinorError(err)
		}
		f.Logger.Infof("Waiting for installation (%s/%s) to be created", installation.Namespace, installation.Name)
		return retry.Ok()
	}); err != nil {
		return err
	}

	// Wait for the installation to be created and successfully reconciled
	return f.waitForInstallationToBeCreated(ctx, installation)
}

// waitForInstallationToBeCreated waits for the given installation
// to be created and successfully reconciled.
func (f *CommonFrameworkLandscaper) waitForInstallationToBeCreated(ctx context.Context, installation *landscaperv1alpha1.Installation) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 10*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.LandscaperClient.Get(ctx, client.ObjectKeyFromObject(installation), installation)
		if err != nil {
			f.Logger.Infof("Could not get installation (%s/%s): %s", installation.Namespace, installation.Name, err.Error())
			return retry.MinorError(err)
		}
		err = landscaperhealth.CheckInstallation(installation)
		if err != nil {
			f.Logger.Infof("Waiting for Installation (%s/%s) to be reconciled successfully. %v", installation.Namespace, installation.Name, err)
			return retry.MinorError(fmt.Errorf("installation %s not reconciled successfully: %w", installation.Name, err))
		}
		return retry.Ok()
	})
}

// DeleteInstallation deletes the given installation and waits for it to be deleted.
func (f *CommonFrameworkLandscaper) DeleteInstallation(ctx context.Context, name string) error {
	installation := &landscaperv1alpha1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, f.ResourceSuffix),
			Namespace: f.Config.TargetNamespace,
		},
	}

	// Delete the installation
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.LandscaperClient.Delete(ctx, installation)
		if err != nil {
			f.Logger.Infof("Could not delete installation %s: %s", installation.Name, err.Error())
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	// Wait for the installation to be deleted
	return f.waitForInstallationToBeDeleted(ctx, installation)
}

// waitForInstallationToBeDeleted waits for the given installation to be deleted.
func (f *CommonFrameworkLandscaper) waitForInstallationToBeDeleted(ctx context.Context, installation *landscaperv1alpha1.Installation) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 10*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.LandscaperClient.Get(ctx, client.ObjectKeyFromObject(installation), installation)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			f.Logger.Infof("Could not get installation %s: %s", installation.Name, err.Error())
			return retry.MinorError(err)
		}
		f.Logger.Infof("Waiting for Installation (%s/%s) to be deleted", installation.Namespace, installation.Name)
		return retry.MinorError(fmt.Errorf("installation %s still exists", installation.Name))
	})
}

// DeleteTarget deletes a given target resource with retries
func (f *CommonFrameworkLandscaper) DeleteTarget(ctx context.Context, name string) error {
	target := &landscaperv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, f.ResourceSuffix),
			Namespace: f.Config.TargetNamespace,
		},
	}

	return retry.UntilTimeout(ctx, 20*time.Second, 1*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.LandscaperClient.Delete(ctx, target)
		if err != nil {
			f.Logger.Debugf("Could not delete target (%s/%s): %s", target.Namespace, target.Name, err.Error())
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
}

// CreateConfigMap creates a new config map with retries
func (f *CommonFrameworkLandscaper) CreateConfigMap(ctx context.Context, name string, data map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, f.ResourceSuffix),
			Namespace: f.Config.TargetNamespace,
		},
		Data: data,
	}

	// Create the config map
	return retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.LandscaperClient.Create(ctx, cm)
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) || apierrors.IsAlreadyExists(err) {
			return retry.SevereError(err)
		}
		if err != nil {
			f.Logger.Infof("Could not create config map %s: %s", cm.Name, err.Error())
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
}

// DeleteConfigMap deletes a given config map resource with retries
func (f *CommonFrameworkLandscaper) DeleteConfigMap(ctx context.Context, name string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, f.ResourceSuffix),
			Namespace: f.Config.TargetNamespace,
		},
	}

	return retry.UntilTimeout(ctx, 20*time.Second, 1*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.LandscaperClient.Delete(ctx, cm)
		if err != nil {
			f.Logger.Debugf("Could not delete config map (%s/%s): %s", cm.Namespace, cm.Name, err.Error())
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
}

// CreateTargetWithKubeconfig creates the landscaper target from a given name and kubeconfig path on the local filesystem
func (f *CommonFrameworkLandscaper) CreateTargetWithKubeconfig(ctx context.Context, name, kubeconfigPath string) error {
	seedKubeconfigBytes, err := os.ReadFile(kubeconfigPath)
	ExpectNoError(err)

	toJSON, err := yaml.ToJSON(seedKubeconfigBytes)
	ExpectNoError(err)

	anyJSON := landscaperv1alpha1.AnyJSON{
		RawMessage: json.RawMessage(toJSON),
	}

	seedCluster := &landscaperv1alpha1.Target{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, f.ResourceSuffix),
			Namespace: f.Config.TargetNamespace,
		},
		Spec: landscaperv1alpha1.TargetSpec{
			Type:          landscaperv1alpha1.KubernetesClusterTargetType,
			Configuration: anyJSON,
		},
	}

	return f.LandscaperClient.Create(ctx, seedCluster)
}
