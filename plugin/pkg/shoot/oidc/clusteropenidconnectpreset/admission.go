// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusteropenidconnectpreset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	settingsinformers "github.com/gardener/gardener/pkg/client/settings/informers/externalversions"
	settingsv1alpha1lister "github.com/gardener/gardener/pkg/client/settings/listers/settings/v1alpha1"
	plugin "github.com/gardener/gardener/plugin/pkg"
	applier "github.com/gardener/gardener/plugin/pkg/shoot/oidc"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameClusterOpenIDConnectPreset, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ClusterOpenIDConnectPreset contains listers and admission handler.
type ClusterOpenIDConnectPreset struct {
	*admission.Handler

	projectLister     gardencorev1beta1listers.ProjectLister
	clusterOIDCLister settingsv1alpha1lister.ClusterOpenIDConnectPresetLister
	readyFunc         admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ClusterOpenIDConnectPreset{})
	_ = admissioninitializer.WantsSettingsInformerFactory(&ClusterOpenIDConnectPreset{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new OpenIDConnectPreset admission plugin.
func New() (*ClusterOpenIDConnectPreset, error) {
	return &ClusterOpenIDConnectPreset{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (c *ClusterOpenIDConnectPreset) AssignReadyFunc(f admission.ReadyFunc) {
	c.readyFunc = f
	c.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (c *ClusterOpenIDConnectPreset) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	projectInformer := f.Core().V1beta1().Projects()
	c.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, projectInformer.Informer().HasSynced)
}

// SetSettingsInformerFactory gets Lister from SharedInformerFactory.
func (c *ClusterOpenIDConnectPreset) SetSettingsInformerFactory(f settingsinformers.SharedInformerFactory) {
	oidc := f.Settings().V1alpha1().OpenIDConnectPresets()

	clusterOIDC := f.Settings().V1alpha1().ClusterOpenIDConnectPresets()
	c.clusterOIDCLister = clusterOIDC.Lister()

	readyFuncs = append(readyFuncs, oidc.Informer().HasSynced, clusterOIDC.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (c *ClusterOpenIDConnectPreset) ValidateInitialization() error {
	if c.clusterOIDCLister == nil {
		return errors.New("missing clusteropenidpreset lister")
	}
	if c.projectLister == nil {
		return errors.New("missing project lister")
	}
	return nil
}

var _ admission.MutationInterface = &ClusterOpenIDConnectPreset{}

// Admit tries to determine a OpenIDConnectPreset hosted zone for the Shoot's external domain.
func (c *ClusterOpenIDConnectPreset) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if c.readyFunc == nil {
		c.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !c.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	// Ignore all subresource calls
	// Ignore all operations other than CREATE
	if len(a.GetSubresource()) != 0 || a.GetKind().GroupKind() != core.Kind("Shoot") || a.GetOperation() != admission.Create {
		return nil
	}
	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	// If the Shoot manifest specifies OIDCConfig.
	if shoot.Spec.Kubernetes.KubeAPIServer != nil && shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig != nil {
		return nil
	}

	coidcs, err := c.clusterOIDCLister.List(labels.Everything())
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not list existing clusteropenidconnectpresets: %w", err))
	}
	if len(coidcs) == 0 {
		return nil
	}

	projects, err := c.projectLister.List(labels.Everything())
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not list existing projects: %w", err))
	}
	if len(projects) == 0 {
		return nil
	}

	var foundProject *gardencorev1beta1.Project
	for _, project := range projects {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == shoot.Namespace && project.Status.Phase == gardencorev1beta1.ProjectReady {
			foundProject = project
			break
		}
	}
	if foundProject == nil {
		return nil
	}

	preset, err := filterClusterOIDCs(coidcs, shoot, foundProject)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	// We have an OpenIDConnectPreset, use it.
	if preset != nil {
		applier.ApplyOIDCConfiguration(shoot, preset)
		return nil
	}

	return nil
}

func filterClusterOIDCs(oidcs []*settingsv1alpha1.ClusterOpenIDConnectPreset, shoot *core.Shoot, project *gardencorev1beta1.Project) (*settingsv1alpha1.OpenIDConnectPresetSpec, error) {
	var matchedPreset *settingsv1alpha1.ClusterOpenIDConnectPreset

	for _, oidc := range oidcs {
		spec := oidc.Spec
		projectSelector, err := metav1.LabelSelectorAsSelector(spec.ProjectSelector)
		if err != nil {
			return nil, fmt.Errorf("label selector conversion failed: %v for projectSelector: %w", *spec.ShootSelector, err)
		}
		shootSelector, err := metav1.LabelSelectorAsSelector(spec.ShootSelector)
		if err != nil {
			return nil, fmt.Errorf("label selector conversion failed: %v for shootSelector: %w", *spec.ShootSelector, err)
		}

		// check if the Shoot / project labels match the selector
		if !projectSelector.Matches(labels.Set(project.Labels)) || !shootSelector.Matches(labels.Set(shoot.Labels)) {
			continue
		}

		if matchedPreset == nil {
			matchedPreset = oidc
		} else if spec.Weight >= matchedPreset.Spec.Weight {
			if spec.Weight > matchedPreset.Spec.Weight {
				matchedPreset = oidc
			} else if strings.Compare(oidc.Name, matchedPreset.Name) > 0 {
				matchedPreset = oidc
			}
		}
	}

	if matchedPreset == nil {
		return nil, nil
	}
	return &matchedPreset.Spec.OpenIDConnectPresetSpec, nil
}
