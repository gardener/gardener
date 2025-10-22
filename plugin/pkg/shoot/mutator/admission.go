// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootMutator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// MutateShoot is an implementation of admission.Interface.
type MutateShoot struct {
	*admission.Handler

	cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
	namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister
	seedLister                   gardencorev1beta1listers.SeedLister
	readyFunc                    admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&MutateShoot{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new MutateShoot admission plugin.
func New() (*MutateShoot, error) {
	return &MutateShoot{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (m *MutateShoot) AssignReadyFunc(f admission.ReadyFunc) {
	m.readyFunc = f
	m.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (m *MutateShoot) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	m.cloudProfileLister = cloudProfileInformer.Lister()

	namespacedCloudProfileInformer := f.Core().V1beta1().NamespacedCloudProfiles()
	m.namespacedCloudProfileLister = namespacedCloudProfileInformer.Lister()

	seedInformer := f.Core().V1beta1().Seeds()
	m.seedLister = seedInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		cloudProfileInformer.Informer().HasSynced,
		namespacedCloudProfileInformer.Informer().HasSynced,
		seedInformer.Informer().HasSynced,
	)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (m *MutateShoot) ValidateInitialization() error {
	if m.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if m.namespacedCloudProfileLister == nil {
		return errors.New("missing namespacedCloudProfile lister")
	}
	if m.seedLister == nil {
		return errors.New("missing seed lister")
	}
	return nil
}

var _ admission.MutationInterface = (*MutateShoot)(nil)

// Admit mutates the Shoot.
func (m *MutateShoot) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if m.readyFunc == nil {
		m.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !m.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	var (
		shoot    *core.Shoot
		oldShoot = &core.Shoot{}

		allErrs field.ErrorList
	)

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert object to Shoot")
	}

	if a.GetOperation() == admission.Update {
		oldShoot, ok = a.GetOldObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert old object to Shoot")
		}
	}

	cloudProfileSpec, err := gardenerutils.GetCloudProfileSpec(m.cloudProfileLister, m.namespacedCloudProfileLister, shoot)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced cloud profile: %w", err))
	}
	if cloudProfileSpec == nil {
		return nil
	}

	var seed *gardencorev1beta1.Seed
	if shoot.Spec.SeedName != nil {
		seed, err = m.seedLister.Get(*shoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced seed %q: %w", *shoot.Spec.SeedName, err))
		}
	}

	mutationContext := &mutationContext{
		cloudProfileSpec: cloudProfileSpec,
		seed:             seed,
		shoot:            shoot,
		oldShoot:         oldShoot,
	}

	if a.GetOperation() == admission.Create {
		addCreatedByAnnotation(shoot, a.GetUserInfo().GetName())
	}

	mutationContext.addMetadataAnnotations(a)
	mutationContext.defaultShootNetworks(helper.IsWorkerless(shoot))
	allErrs = append(allErrs, mutationContext.defaultKubernetes()...)
	allErrs = append(allErrs, mutationContext.defaultKubernetesVersionForWorkers()...)

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, allErrs.ToAggregate())
	}

	return nil
}

type mutationContext struct {
	cloudProfileSpec *gardencorev1beta1.CloudProfileSpec
	seed             *gardencorev1beta1.Seed
	shoot            *core.Shoot
	oldShoot         *core.Shoot
}

func addCreatedByAnnotation(shoot *core.Shoot, userName string) {
	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenCreatedBy, userName)
}

func (c *mutationContext) addMetadataAnnotations(a admission.Attributes) {
	if a.GetOperation() == admission.Create {
		addInfrastructureDeploymentTask(c.shoot)
		addDNSRecordDeploymentTasks(c.shoot)
	}

	var (
		oldIsHibernated = c.oldShoot.Spec.Hibernation != nil && c.oldShoot.Spec.Hibernation.Enabled != nil && *c.oldShoot.Spec.Hibernation.Enabled
		newIsHibernated = c.shoot.Spec.Hibernation != nil && c.shoot.Spec.Hibernation.Enabled != nil && *c.shoot.Spec.Hibernation.Enabled
	)

	if !newIsHibernated && oldIsHibernated {
		addInfrastructureDeploymentTask(c.shoot)
		addDNSRecordDeploymentTasks(c.shoot)
	}

	if !reflect.DeepEqual(c.oldShoot.Spec.Provider.InfrastructureConfig, c.shoot.Spec.Provider.InfrastructureConfig) ||
		c.oldShoot.Spec.Networking != nil && c.oldShoot.Spec.Networking.IPFamilies != nil && !reflect.DeepEqual(c.oldShoot.Spec.Networking.IPFamilies, c.shoot.Spec.Networking.IPFamilies) {
		addInfrastructureDeploymentTask(c.shoot)
	}

	// We rely that SSHAccess is defaulted in the shoot creation, that is why we do not check for nils for the new shoot object.
	if c.oldShoot.Spec.Provider.WorkersSettings != nil &&
		c.oldShoot.Spec.Provider.WorkersSettings.SSHAccess != nil &&
		c.oldShoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled != c.shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if !reflect.DeepEqual(c.oldShoot.Spec.DNS, c.shoot.Spec.DNS) {
		addDNSRecordDeploymentTasks(c.shoot)
	}

	if sets.New(
		v1beta1constants.ShootOperationRotateSSHKeypair,
		v1beta1constants.OperationRotateCredentialsStart,
		v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
	).Has(c.shoot.Annotations[v1beta1constants.GardenerOperation]) {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if c.shoot.Spec.Maintenance != nil &&
		ptr.Deref(c.shoot.Spec.Maintenance.ConfineSpecUpdateRollout, false) &&
		!apiequality.Semantic.DeepEqual(c.oldShoot.Spec, c.shoot.Spec) &&
		c.shoot.Status.LastOperation != nil &&
		c.shoot.Status.LastOperation.State == core.LastOperationStateFailed {
		metav1.SetMetaDataAnnotation(&c.shoot.ObjectMeta, v1beta1constants.FailedShootNeedsRetryOperation, "true")
	}
}

func addInfrastructureDeploymentTask(shoot *core.Shoot) {
	addDeploymentTasks(shoot, v1beta1constants.ShootTaskDeployInfrastructure)
}

func addDNSRecordDeploymentTasks(shoot *core.Shoot) {
	addDeploymentTasks(shoot,
		v1beta1constants.ShootTaskDeployDNSRecordInternal,
		v1beta1constants.ShootTaskDeployDNSRecordExternal,
		v1beta1constants.ShootTaskDeployDNSRecordIngress,
	)
}

func addDeploymentTasks(shoot *core.Shoot, tasks ...string) {
	if shoot.Annotations == nil {
		shoot.Annotations = make(map[string]string)
	}
	controllerutils.AddTasks(shoot.Annotations, tasks...)
}

func (c *mutationContext) defaultShootNetworks(workerless bool) {
	if c.seed == nil {
		return
	}

	c.defaultPodsNetwork(workerless)
	c.defaultServicesNetwork(workerless)
}

func (c *mutationContext) defaultPodsNetwork(workerless bool) {
	if c.shoot.Spec.Networking.Pods != nil || workerless {
		return
	}

	defaults := c.seed.Spec.Networks.ShootDefaults
	if defaults == nil || defaults.Pods == nil {
		return
	}

	if cidrMatchesIPFamily(*defaults.Pods, c.shoot.Spec.Networking.IPFamilies) {
		c.shoot.Spec.Networking.Pods = defaults.Pods
	}
}

func (c *mutationContext) defaultServicesNetwork(workerless bool) {
	if c.shoot.Spec.Networking.Services != nil {
		return
	}

	defaults := c.seed.Spec.Networks.ShootDefaults
	if defaults != nil && defaults.Services != nil &&
		cidrMatchesIPFamily(*defaults.Services, c.shoot.Spec.Networking.IPFamilies) {
		c.shoot.Spec.Networking.Services = defaults.Services
		return
	}

	if workerless && slices.Contains(c.shoot.Spec.Networking.IPFamilies, core.IPFamilyIPv6) {
		ulaCIDR := generateULAServicesCIDR()
		c.shoot.Spec.Networking.Services = &ulaCIDR
	}
}

func cidrMatchesIPFamily(cidr string, ipfamilies []core.IPFamily) bool {
	ip, _, _ := net.ParseCIDR(cidr)
	return ip != nil && (ip.To4() != nil && slices.Contains(ipfamilies, core.IPFamilyIPv4) || ip.To4() == nil && slices.Contains(ipfamilies, core.IPFamilyIPv6))
}

// generateULAServicesCIDR generates a /112 ULA (Unique Local Address) CIDR for IPv6 services.
func generateULAServicesCIDR() string {
	// Generate a random 40-bit Global ID (5 bytes) for the ULA
	// ULA format: fd + 40-bit Global ID + 16-bit Subnet-ID + 64-bit Interface ID
	// For services, we use a /112 which leaves 16 bits for service IPs

	var globalID [5]byte
	if _, err := rand.Read(globalID[:]); err != nil {
		// Fallback to a deterministic value if random generation fails
		return "fd00:10:2::/112"
	}

	// Format as fd + 5 bytes of Global ID + 2 bytes of Subnet ID (using 0000 for services)
	return fmt.Sprintf("fd%02x:%02x%02x:%02x%02x::/112",
		globalID[0], globalID[1], globalID[2], globalID[3], globalID[4])
}

func (c *mutationContext) defaultKubernetes() field.ErrorList {
	var path = field.NewPath("spec", "kubernetes")

	defaultVersion, errList := defaultKubernetesVersion(c.cloudProfileSpec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, path.Child("version"))
	if len(errList) > 0 {
		return errList
	}

	if defaultVersion != nil {
		c.shoot.Spec.Kubernetes.Version = *defaultVersion
	}

	return nil
}

func defaultKubernetesVersion(constraints []gardencorev1beta1.ExpirableVersion, shootVersion string, fldPath *field.Path) (*string, field.ErrorList) {
	var (
		allErrs           = field.ErrorList{}
		shootVersionMajor *uint64
		shootVersionMinor *uint64
		versionParts      = strings.Split(shootVersion, ".")
	)

	if len(versionParts) == 3 {
		return nil, allErrs
	}
	if len(versionParts) == 2 && len(versionParts[1]) > 0 {
		v, err := strconv.ParseUint(versionParts[1], 10, 0)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, versionParts[1], "must be a semantic version"))
			return nil, allErrs
		}
		shootVersionMinor = ptr.To(v)
	}
	if len(versionParts) >= 1 && len(versionParts[0]) > 0 {
		v, err := strconv.ParseUint(versionParts[0], 10, 0)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, versionParts[0], "must be a semantic version"))
			return nil, allErrs
		}
		shootVersionMajor = ptr.To(v)
	}

	if latestVersion := findLatestSupportedVersion(constraints, shootVersionMajor, shootVersionMinor); latestVersion != nil {
		return ptr.To(latestVersion.String()), nil
	}

	allErrs = append(allErrs, field.Invalid(fldPath, shootVersion, fmt.Sprintf("couldn't find a suitable version for %s. Suitable versions have a non-expired expiration date and are no 'preview' versions. 'Preview'-classified versions have to be selected explicitly", shootVersion)))
	return nil, allErrs
}

func findLatestSupportedVersion(constraints []gardencorev1beta1.ExpirableVersion, major, minor *uint64) *semver.Version {
	var latestVersion *semver.Version
	for _, versionConstraint := range constraints {
		if v1beta1helper.CurrentLifecycleClassification(versionConstraint) != gardencorev1beta1.ClassificationSupported {
			continue
		}

		// CloudProfile cannot contain invalid semVer shootVersion
		cpVersion := semver.MustParse(versionConstraint.Version)

		// defaulting on patch level: version has to have the same major and minor kubernetes version
		if major != nil && cpVersion.Major() != *major {
			continue
		}

		if minor != nil && cpVersion.Minor() != *minor {
			continue
		}

		if latestVersion == nil || cpVersion.GreaterThan(latestVersion) {
			latestVersion = cpVersion
		}
	}

	return latestVersion
}

func (c *mutationContext) defaultKubernetesVersionForWorkers() field.ErrorList {
	var path = field.NewPath("spec", "provider")

	for i, worker := range c.shoot.Spec.Provider.Workers {
		idxPath := path.Child("workers").Index(i)

		if worker.Kubernetes != nil {
			if errList := c.defaultKubernetesVersionForWorker(idxPath, worker); len(errList) > 0 {
				return errList
			}
		}
	}

	return nil
}

func (c *mutationContext) defaultKubernetesVersionForWorker(idxPath *field.Path, worker core.Worker) field.ErrorList {
	if worker.Kubernetes.Version == nil {
		return nil
	}

	defaultVersion, errList := defaultKubernetesVersion(c.cloudProfileSpec.Kubernetes.Versions, *worker.Kubernetes.Version, idxPath.Child("kubernetes", "version"))
	if len(errList) > 0 {
		return errList
	}

	if defaultVersion != nil {
		worker.Kubernetes.Version = defaultVersion
	}
	return nil
}
