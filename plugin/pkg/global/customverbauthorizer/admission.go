// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package customverbauthorizer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

const (
	// CustomVerbModifyProjectTolerationsWhitelist is a constant for the custom verb that allows modifying the
	// `.spec.tolerations.whitelist` field in `Project` resources.
	CustomVerbModifyProjectTolerationsWhitelist = "modify-spec-tolerations-whitelist"
	// CustomVerbProjectManageMembers is a constant for the custom verb that allows to manage human users or
	// groups subjects in the `.spec.members` field in `Project` resources.
	CustomVerbProjectManageMembers = "manage-members"

	// CustomVerbNamespacedCloudProfileModifyKubernetes is a constant for the custom verb that allows modifying the
	// `.spec.kubernetes` field in `NamespacedCloudProfile` resources.
	CustomVerbNamespacedCloudProfileModifyKubernetes = "modify-spec-kubernetes"
	// CustomVerbNamespacedCloudProfileModifyMachineImages is a constant for the custom verb that allows modifying the
	// `.spec.machineImages` field in `NamespacedCloudProfile` resources.
	CustomVerbNamespacedCloudProfileModifyMachineImages = "modify-spec-machineimages"
	// CustomVerbNamespacedCloudProfileModifyProviderConfig is a constant for the custom verb that allows modifying the
	// `.spec.providerConfig` field in `NamespacedCloudProfile` resources.
	CustomVerbNamespacedCloudProfileModifyProviderConfig = "modify-spec-providerconfig"
	// CustomVerbNamespacedCloudProfileRaiseLimits is a constant for the custom verb that allows raising the
	// `.spec.limits` limits in `NamespacedCloudProfile` resources above values defined in the parent `CloudProfile`.
	CustomVerbNamespacedCloudProfileRaiseLimits = "raise-spec-limits"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameCustomVerbAuthorizer, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(_ io.Reader) (admission.Interface, error) {
	return New()
}

// CustomVerbAuthorizer contains an admission handler and listers.
type CustomVerbAuthorizer struct {
	*admission.Handler
	cloudProfileLister gardencorev1beta1listers.CloudProfileLister
	authorizer         authorizer.Authorizer
	readyFunc          admission.ReadyFunc
}

var (
	_          = admissioninitializer.WantsAuthorizer(&CustomVerbAuthorizer{})
	_          = admissioninitializer.WantsCoreInformerFactory(&CustomVerbAuthorizer{})
	readyFuncs []admission.ReadyFunc
)

// New creates a new CustomVerbAuthorizer admission plugin.
func New() (*CustomVerbAuthorizer, error) {
	return &CustomVerbAuthorizer{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (c *CustomVerbAuthorizer) AssignReadyFunc(f admission.ReadyFunc) {
	c.readyFunc = f
	c.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (c *CustomVerbAuthorizer) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	c.cloudProfileLister = cloudProfileInformer.Lister()

	readyFuncs = append(readyFuncs, cloudProfileInformer.Informer().HasSynced)
}

// SetAuthorizer gets the authorizer.
func (c *CustomVerbAuthorizer) SetAuthorizer(authorizer authorizer.Authorizer) {
	c.authorizer = authorizer
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (c *CustomVerbAuthorizer) ValidateInitialization() error {
	if c.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	return nil
}

var _ admission.ValidationInterface = &CustomVerbAuthorizer{}

// Validate makes admissions decisions based on custom verbs.
func (c *CustomVerbAuthorizer) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	switch a.GetKind().GroupKind() {
	case core.Kind("Project"):
		return c.admitProjects(ctx, a)
	case core.Kind("NamespacedCloudProfile"):
		return c.admitNamespacedCloudProfiles(ctx, a)
	}

	return nil
}

func (c *CustomVerbAuthorizer) admitProjects(ctx context.Context, a admission.Attributes) error {
	var (
		oldObj = &core.Project{}
		obj    *core.Project
		ok     bool
	)

	obj, ok = a.GetObject().(*core.Project)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Project object")
	}

	if a.GetOperation() == admission.Update {
		oldObj, ok = a.GetOldObject().(*core.Project)
		if !ok {
			return apierrors.NewBadRequest("could not convert old resource into Project object")
		}
	}

	if mustCheckProjectTolerationsWhitelist(oldObj.Spec.Tolerations, obj.Spec.Tolerations) {
		return c.authorize(ctx, a, CustomVerbModifyProjectTolerationsWhitelist, "modify .spec.tolerations.whitelist")
	}

	if mustCheckProjectMembers(oldObj.Spec.Members, obj.Spec.Members, obj.Spec.Owner, a.GetUserInfo()) {
		return c.authorize(ctx, a, CustomVerbProjectManageMembers, "manage human users or groups in .spec.members")
	}

	return nil
}

func (c *CustomVerbAuthorizer) admitNamespacedCloudProfiles(ctx context.Context, a admission.Attributes) error {
	var (
		oldObj = &core.NamespacedCloudProfile{}
		obj    *core.NamespacedCloudProfile
		ok     bool
	)

	obj, ok = a.GetObject().(*core.NamespacedCloudProfile)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into NamespacedCloudProfile object")
	}

	parentCloudProfileName := obj.Spec.Parent.Name
	parentCloudProfile, err := c.cloudProfileLister.Get(parentCloudProfileName)
	if err != nil {
		return apierrors.NewBadRequest("parent CloudProfile could not be found")
	}

	if a.GetOperation() == admission.Update {
		oldObj, ok = a.GetOldObject().(*core.NamespacedCloudProfile)
		if !ok {
			return apierrors.NewBadRequest("could not convert old resource into NamespacedCloudProfile object")
		}
	}

	if mustCheckKubernetes(oldObj.Spec.Kubernetes, obj.Spec.Kubernetes) {
		err := c.authorize(ctx, a, CustomVerbNamespacedCloudProfileModifyKubernetes, "modify .spec.kubernetes")
		if err != nil {
			return err
		}
	}

	if mustCheckMachineImages(oldObj.Spec.MachineImages, obj.Spec.MachineImages) {
		err := c.authorize(ctx, a, CustomVerbNamespacedCloudProfileModifyMachineImages, "modify .spec.machineImages")
		if err != nil {
			return err
		}
	}

	if mustCheckProviderConfig(oldObj.Spec.ProviderConfig, obj.Spec.ProviderConfig) {
		err := c.authorize(ctx, a, CustomVerbNamespacedCloudProfileModifyProviderConfig, "modify .spec.providerConfig")
		if err != nil {
			return err
		}
	}

	if mustCheckLimits(oldObj.Spec.Limits, obj.Spec.Limits, parentCloudProfile) {
		err := c.authorize(ctx, a, CustomVerbNamespacedCloudProfileRaiseLimits, "increase .spec.limits above parent CloudProfile limits")
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *CustomVerbAuthorizer) authorize(ctx context.Context, a admission.Attributes, verb, operation string) error {
	var (
		userInfo  = a.GetUserInfo()
		resource  = a.GetResource()
		namespace = a.GetNamespace()
		name      = a.GetName()
	)

	decision, _, err := c.authorizer.Authorize(ctx, authorizer.AttributesRecord{
		User:            userInfo,
		APIGroup:        resource.Group,
		Resource:        resource.Resource,
		Namespace:       namespace,
		Name:            name,
		Verb:            verb,
		ResourceRequest: true,
	})
	if err != nil {
		return err
	}
	if decision != authorizer.DecisionAllow {
		return admission.NewForbidden(a, fmt.Errorf("user %q is not allowed to %s for %q", userInfo.GetName(), operation, resource.Resource))
	}

	return nil
}

func mustCheckProjectTolerationsWhitelist(oldTolerations, tolerations *core.ProjectTolerations) bool {
	if apiequality.Semantic.DeepEqual(oldTolerations, tolerations) {
		return false
	}
	if oldTolerations == nil && tolerations != nil {
		return !apiequality.Semantic.DeepEqual(nil, tolerations.Whitelist)
	}
	if oldTolerations != nil && tolerations == nil {
		return !apiequality.Semantic.DeepEqual(oldTolerations.Whitelist, nil)
	}
	return !apiequality.Semantic.DeepEqual(oldTolerations.Whitelist, tolerations.Whitelist)
}

func mustCheckProjectMembers(oldMembers, members []core.ProjectMember, owner *rbacv1.Subject, userInfo user.Info) bool {
	if apiequality.Semantic.DeepEqual(oldMembers, members) {
		return false
	}

	// If the user submitting this admission request is the owner then it will be/is bound to the `manage-members`
	// custom verb anyway, so let's allow him to add/remove human users.
	if userIsOwner(userInfo, owner) {
		return false
	}

	var oldHumanUsers, newHumanUsers = findHumanUsers(oldMembers), findHumanUsers(members)
	// Remove owner subject from `members` list to always allow it to be added
	if owner != nil && isHumanUser(*owner) {
		oldHumanUsers.Delete(humanMemberKey(*owner))
		newHumanUsers.Delete(humanMemberKey(*owner))
	}

	return !oldHumanUsers.Equal(newHumanUsers)
}

func findHumanUsers(members []core.ProjectMember) sets.Set[string] {
	result := sets.New[string]()

	for _, member := range members {
		if isHumanUser(member.Subject) {
			result.Insert(humanMemberKey(member.Subject))
		}
	}

	return result
}

func isHumanUser(subject rbacv1.Subject) bool {
	return subject.Kind == rbacv1.UserKind && !strings.HasPrefix(subject.Name, serviceaccount.ServiceAccountUsernamePrefix)
}

func humanMemberKey(subject rbacv1.Subject) string {
	return subject.Kind + subject.Name
}

func userIsOwner(userInfo user.Info, owner *rbacv1.Subject) bool {
	if owner == nil { // no explicit owner is set, i.e., the creator will be defaulted to the owner
		return true
	}

	switch owner.Kind {
	case rbacv1.ServiceAccountKind:
		namespace, name, err := serviceaccount.SplitUsername(userInfo.GetName())
		if err != nil {
			return false
		}
		return owner.Name == name && owner.Namespace == namespace

	case rbacv1.UserKind:
		return owner.Name == userInfo.GetName()

	case rbacv1.GroupKind:
		return sets.New(userInfo.GetGroups()...).Has(owner.Name)
	}

	return false
}

func mustCheckKubernetes(oldKubernetes, kubernetes *core.KubernetesSettings) bool {
	return !apiequality.Semantic.DeepEqual(oldKubernetes, kubernetes)
}

func mustCheckMachineImages(oldMachineImages, machineImages []core.MachineImage) bool {
	return !apiequality.Semantic.DeepEqual(oldMachineImages, machineImages)
}

func mustCheckProviderConfig(oldProviderConfig, providerConfig *runtime.RawExtension) bool {
	return !apiequality.Semantic.DeepEqual(oldProviderConfig, providerConfig)
}

func mustCheckLimits(oldLimits, limits *core.Limits, parentCloudProfile *v1beta1.CloudProfile) bool {
	return limits != nil &&
		!apiequality.Semantic.DeepEqual(oldLimits, limits) &&
		parentCloudProfile.Spec.Limits != nil &&
		parentCloudProfile.Spec.Limits.MaxNodesTotal != nil &&
		ptr.Deref(limits.MaxNodesTotal, 0) > ptr.Deref(parentCloudProfile.Spec.Limits.MaxNodesTotal, 0)
}
