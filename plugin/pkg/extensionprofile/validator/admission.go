// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"

	gardencoreapi "github.com/gardener/gardener/pkg/api"
	gardencorehelper "github.com/gardener/gardener/pkg/api/core/helper"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	celutils "github.com/gardener/gardener/pkg/utils/cel"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameExtensionProfileValidator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ExtensionProfileValidator contains listers and admission handler.
type ExtensionProfileValidator struct {
	*admission.Handler

	extensionProfileLister gardencorev1beta1listers.ExtensionProfileLister
	shootLister            gardencorev1beta1listers.ShootLister
	celCompiler            *celutils.Compiler
	readyFunc              admission.ReadyFunc
}

var (
	_          = admissioninitializer.WantsCoreInformerFactory(&ExtensionProfileValidator{})
	readyFuncs []admission.ReadyFunc
)

// New creates a new ExtensionProfileValidator admission plugin.
func New() (*ExtensionProfileValidator, error) {
	compiler, err := celutils.NewCompiler()
	if err != nil {
		return nil, fmt.Errorf("could not create CEL compiler: %w", err)
	}

	return &ExtensionProfileValidator{
		Handler:     admission.NewHandler(admission.Create, admission.Update, admission.Delete),
		celCompiler: compiler,
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ExtensionProfileValidator) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ExtensionProfileValidator) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	extensionProfileInformer := f.Core().V1beta1().ExtensionProfiles()
	v.extensionProfileLister = extensionProfileInformer.Lister()

	shootInformer := f.Core().V1beta1().Shoots()
	v.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, extensionProfileInformer.Informer().HasSynced, shootInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ExtensionProfileValidator) ValidateInitialization() error {
	if v.extensionProfileLister == nil {
		return errors.New("missing extensionProfile lister")
	}
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

var _ admission.ValidationInterface = (*ExtensionProfileValidator)(nil)

// Validate validates the ExtensionProfile and Shoot resources against each other.
func (v *ExtensionProfileValidator) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced.
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

	// The whole feature is guarded by the ExtensionComponentVersions feature gate.
	if !features.DefaultFeatureGate.Enabled(features.ExtensionComponentVersions) {
		return nil
	}

	if a.GetSubresource() != "" {
		return nil
	}

	switch a.GetKind().GroupKind() {
	case gardencore.Kind("ExtensionProfile"):
		return v.validateExtensionProfile(a)
	case gardencore.Kind("Shoot"):
		return v.validateShoot(ctx, a)
	default:
		return nil
	}
}

// validateExtensionProfile rejects removing an offered version, or deleting the whole profile, while a
// shoot still pins that version.
func (v *ExtensionProfileValidator) validateExtensionProfile(a admission.Attributes) error {
	if a.GetOperation() != admission.Update && a.GetOperation() != admission.Delete {
		return nil
	}

	oldProfile, ok := a.GetOldObject().(*gardencore.ExtensionProfile)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert old resource into ExtensionProfile object"))
	}

	// Determine which (component, version) pairs still exist after this request.
	remaining := map[string]map[string]struct{}{}
	if a.GetOperation() == admission.Update {
		newProfile, ok := a.GetObject().(*gardencore.ExtensionProfile)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert resource into ExtensionProfile object"))
		}
		for _, component := range newProfile.Spec.Components {
			versions := map[string]struct{}{}
			for _, version := range component.Versions {
				versions[version.Version] = struct{}{}
			}
			remaining[component.Name] = versions
		}
	}

	pinningShoots, err := v.shootsPinningRemovedVersions(oldProfile, remaining)
	if err != nil {
		return err
	}
	if len(pinningShoots) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("cannot remove version(s) of ExtensionProfile %q still in use by shoot(s): %s", oldProfile.Name, formatShootList(pinningShoots)))
	}

	return nil
}

// maxReportedShoots bounds the number of shoots named in an in-use error; the full count can be large.
const maxReportedShoots = 10

// formatShootList renders a bounded, human-readable list of shoot names, summarizing the remainder.
func formatShootList(shoots []string) string {
	if len(shoots) <= maxReportedShoots {
		return strings.Join(shoots, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(shoots[:maxReportedShoots], ", "), len(shoots)-maxReportedShoots)
}

// shootsPinningRemovedVersions returns the shoots that pin a (component, version) of the profile that is
// no longer present in the `remaining` set. The informer lister only supports label selectors, so all
// shoots are listed and filtered in memory (as other admission plugins do); listing stops once the
// reported cap is reached, since the exact total is not surfaced.
func (v *ExtensionProfileValidator) shootsPinningRemovedVersions(profile *gardencore.ExtensionProfile, remaining map[string]map[string]struct{}) ([]string, error) {
	shoots, err := v.shootLister.List(labels.Everything())
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not list shoots: %w", err))
	}

	var pinningShoots []string
	for _, shoot := range shoots {
		for _, extension := range shoot.Spec.Extensions {
			if extension.Type != profile.Name {
				continue
			}
			for _, component := range extension.Components {
				if component.Version == nil {
					continue
				}
				versions, componentRemains := remaining[component.Name]
				if _, versionRemains := versions[*component.Version]; componentRemains && versionRemains {
					continue
				}
				pinningShoots = append(pinningShoots, shoot.Namespace+"/"+shoot.Name)
			}
		}
		if len(pinningShoots) > maxReportedShoots {
			return pinningShoots, nil
		}
	}

	return pinningShoots, nil
}

// validateShoot validates a shoot's pinned component versions against the matching ExtensionProfile,
// only for pins that changed on this request.
func (v *ExtensionProfileValidator) validateShoot(ctx context.Context, a admission.Attributes) error {
	if a.GetOperation() != admission.Create && a.GetOperation() != admission.Update {
		return nil
	}

	shoot, ok := a.GetObject().(*gardencore.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	var oldShoot *gardencore.Shoot
	if a.GetOperation() == admission.Update {
		oldShoot, ok = a.GetOldObject().(*gardencore.Shoot)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into Shoot object"))
		}
	}

	// Convert the shoot to v1beta1 for CEL evaluation, which binds the `shoot` variable to the external type.
	shootV1beta1 := &gardencorev1beta1.Shoot{}
	if err := gardencoreapi.Scheme.Convert(shoot, shootV1beta1, nil); err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not convert shoot to v1beta1 for compatibility evaluation: %w", err))
	}
	var oldShootV1beta1 *gardencorev1beta1.Shoot
	if oldShoot != nil {
		oldShootV1beta1 = &gardencorev1beta1.Shoot{}
		if err := gardencoreapi.Scheme.Convert(oldShoot, oldShootV1beta1, nil); err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not convert old shoot to v1beta1 for compatibility evaluation: %w", err))
		}
	}

	for _, extension := range shoot.Spec.Extensions {
		for _, component := range extension.Components {
			if component.Version == nil {
				continue
			}

			profile, err := v.getProfile(extension.Type)
			if err != nil {
				return err
			}

			versionEntry, found := findVersion(profile, component.Name, *component.Version)
			if !found {
				return admission.NewForbidden(a, fmt.Errorf("extension %q component %q: version %q is not offered by the ExtensionProfile", extension.Type, component.Name, *component.Version))
			}

			if pinChanged(oldShoot, extension.Type, component.Name, *component.Version) {
				if err := validateClassification(a, extension.Type, component.Name, *component.Version, versionEntry); err != nil {
					return err
				}
			}

			if err := v.validateCompatibility(ctx, a, extension.Type, component.Name, versionEntry, shootV1beta1, oldShootV1beta1); err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *ExtensionProfileValidator) getProfile(extensionType string) (*gardencore.ExtensionProfile, error) {
	profileV1beta1, err := v.extensionProfileLister.Get(extensionType)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("no ExtensionProfile found for extension type %q; pinning a component version requires a matching ExtensionProfile", extensionType))
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("could not get ExtensionProfile %q: %w", extensionType, err))
	}

	profile := &gardencore.ExtensionProfile{}
	if err := gardencoreapi.Scheme.Convert(profileV1beta1, profile, nil); err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not convert ExtensionProfile %q to internal version: %w", extensionType, err))
	}
	return profile, nil
}

func validateClassification(a admission.Attributes, extensionType, componentName, version string, versionEntry *gardencore.ExtensionVersion) error {
	classification := gardencorehelper.CurrentLifecycleClassification(gardencore.ExpirableVersion{Lifecycle: versionEntry.Lifecycle})
	switch classification {
	case gardencore.ClassificationPreview, gardencore.ClassificationSupported, gardencore.ClassificationDeprecated:
		return nil
	default:
		return admission.NewForbidden(a, fmt.Errorf("extension %q component %q: version %q is classified %q and cannot be pinned", extensionType, componentName, version, classification))
	}
}

func (v *ExtensionProfileValidator) validateCompatibility(ctx context.Context, a admission.Attributes, extensionType, componentName string, versionEntry *gardencore.ExtensionVersion, shoot, oldShoot *gardencorev1beta1.Shoot) error {
	if versionEntry.Compatibility == nil {
		return nil
	}

	for _, rule := range versionEntry.Compatibility.Validations {
		compiled, err := v.celCompiler.Compile(rule.Rule)
		if err != nil {
			return admission.NewForbidden(a, fmt.Errorf("extension %q component %q: compatibility rule %q could not be compiled: %w", extensionType, componentName, rule.Rule, err))
		}

		newResult, err := compiled.EvalBool(ctx, shoot)
		if err != nil {
			return admission.NewForbidden(a, fmt.Errorf("extension %q component %q: compatibility rule %q could not be evaluated: %w", extensionType, componentName, rule.Rule, err))
		}
		if newResult {
			continue
		}

		// Reject only a violation this request introduces. If the old shoot already failed the same rule, an
		// unrelated change (e.g. editing some other field) must not be blocked by a pre-existing
		// incompatibility; the dedicated auto-/force-upgrade paths are responsible for dragging such a shoot
		// back into compatibility. Example: an operator tightens a component's compatibility rule after shoots
		// already pinned that version; those shoots keep reconciling, and only a change that newly breaks the
		// rule is rejected. If the old shoot cannot be evaluated (err != nil), we do not treat it as
		// previously-passing and fall through to rejecting the new violation.
		if oldShoot != nil {
			oldResult, err := compiled.EvalBool(ctx, oldShoot)
			if err == nil && !oldResult {
				continue
			}
		}

		message := fmt.Sprintf("compatibility rule %q is not satisfied", rule.Rule)
		if rule.Message != "" {
			message = fmt.Sprintf("%s (rule %q)", rule.Message, rule.Rule)
		}
		return admission.NewForbidden(a, fmt.Errorf("extension %q component %q: %s", extensionType, componentName, message))
	}

	return nil
}

// findVersion returns the offered version entry for the given component and version, if present. It only
// matches exact versions: a partial pin has already been resolved to a concrete offered version by the
// ShootMutator (mutating admission), which runs before this validating admission plugin.
func findVersion(profile *gardencore.ExtensionProfile, componentName, version string) (*gardencore.ExtensionVersion, bool) {
	for _, component := range profile.Spec.Components {
		if component.Name != componentName {
			continue
		}
		for i, v := range component.Versions {
			if v.Version == version {
				return &component.Versions[i], true
			}
		}
	}
	return nil, false
}

// pinChanged reports whether the pinned version for the given extension type and component differs from
// the old shoot (true on create, or when the old shoot did not pin the same version).
func pinChanged(oldShoot *gardencore.Shoot, extensionType, componentName, version string) bool {
	if oldShoot == nil {
		return true
	}
	for _, extension := range oldShoot.Spec.Extensions {
		if extension.Type != extensionType {
			continue
		}
		for _, component := range extension.Components {
			if component.Name == componentName && component.Version != nil && *component.Version == version {
				return false
			}
		}
	}
	return true
}
