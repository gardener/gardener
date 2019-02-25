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

package common

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"

	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	jsoniter "github.com/json-iterator/go"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var json = jsoniter.ConfigFastest

// ApplyChart takes a Kubernetes client <k8sClient>, chartRender <renderer>, path to a chart <chartPath>, name of the release <name>,
// release's namespace <namespace> and two maps <defaultValues>, <additionalValues>, and renders the template
// based on the merged result of both value maps. The resulting manifest will be applied to the cluster the
// Kubernetes client has been created for.
func ApplyChart(k8sClient kubernetes.Interface, renderer chartrenderer.ChartRenderer, chartPath, name, namespace string, defaultValues, additionalValues map[string]interface{}) error {
	return ApplyChartWithOptions(k8sClient, renderer, chartPath, name, namespace, defaultValues, additionalValues, kubernetes.DefaultApplierOptions)
}

// ApplyChartWithOptions takes a Kubernetes client <k8sClient>, chartRender <renderer>, path to a chart <chartPath>, name of the release <name>,
// release's namespace <namespace> and two maps <defaultValues>, <additionalValues>, and renders the template
// based on the merged result of both value maps. The resulting manifest will be applied to the cluster the
// Kubernetes client has been created for.
// <options> determines how the apply logic is executed.
func ApplyChartWithOptions(k8sClient kubernetes.Interface, renderer chartrenderer.ChartRenderer, chartPath, name, namespace string, defaultValues, additionalValues map[string]interface{}, options kubernetes.ApplierOptions) error {
	release, err := renderer.Render(chartPath, name, namespace, utils.MergeMaps(defaultValues, additionalValues))
	if err != nil {
		return err
	}
	manifestReader := kubernetes.NewManifestReader(release.Manifest())

	return k8sClient.Applier().ApplyManifest(context.Background(), manifestReader, options)
}

// ApplyChartInNamespace is the same as ApplyChart except that it forces the namespace for chart objects when applying the chart, this is because sometimes native chart
// objects do not come with a Release.Namespace option and leave the namespace field empty.
func ApplyChartInNamespace(ctx context.Context, k8sClient kubernetes.Interface, renderer chartrenderer.ChartRenderer, chartPath, name, namespace string, defaultValues, additionalValues map[string]interface{}) error {
	release, err := renderer.Render(chartPath, name, namespace, utils.MergeMaps(defaultValues, additionalValues))
	if err != nil {
		return err
	}

	manifestReader := kubernetes.NewManifestReader(release.Manifest())
	nameSpaceSettingsReader := kubernetes.NewNamespaceSettingReader(manifestReader, namespace)
	return k8sClient.Applier().ApplyManifest(ctx, nameSpaceSettingsReader, kubernetes.DefaultApplierOptions)
}

// GetSecretKeysWithPrefix returns a list of keys of the given map <m> which are prefixed with <kind>.
func GetSecretKeysWithPrefix(kind string, m map[string]*corev1.Secret) []string {
	result := []string{}
	for key := range m {
		if strings.HasPrefix(key, kind) {
			result = append(result, key)
		}
	}
	return result
}

// DistributeOverZones is a function which is used to determine how many nodes should be used
// for each availability zone. It takes the number of availability zones (<zoneSize>), the
// index of the current zone (<zoneIndex>) and the number of nodes which must be distributed
// over the zones (<size>) and returns the number of nodes which should be placed in the zone
// of index <zoneIndex>.
// The distribution happens equally. In case of an uneven number <size>, the last zone will have
// one more node than the others.
func DistributeOverZones(zoneIndex, size, zoneSize int) int {
	first := size / zoneSize
	second := 0
	if zoneIndex < (size % zoneSize) {
		second = 1
	}
	return first + second
}

// DistributePercentOverZones distributes a given percentage value over zones in relation to
// the given total value. In case the total value is evenly divisible over the zones, this
// always just returns the initial percentage. Otherwise, the total value is used to determine
// the weight of a specific zone in relation to the other zones and adapt the given percentage
// accordingly.
func DistributePercentOverZones(zoneIndex int, percent string, zoneSize int, total int) string {
	percents, err := strconv.Atoi(percent[:len(percent)-1])
	if err != nil {
		panic(fmt.Sprintf("given value %q is not a percent value", percent))
	}

	var weightedPercents int
	if total%zoneSize == 0 {
		// Zones are evenly sized, we don't need to adapt the percentage per zone
		weightedPercents = percents
	} else {
		// Zones are not evenly sized, we need to calculate the ratio of each zone
		// and modify the percentage depending on that ratio.
		zoneTotal := DistributeOverZones(zoneIndex, total, zoneSize)
		absoluteTotalRatio := float64(total) / float64(zoneSize)
		ratio := 100.0 / absoluteTotalRatio * float64(zoneTotal)
		// Optimistic rounding up, this will cause an actual max surge / max unavailable percentage to be a bit higher.
		weightedPercents = int(math.Ceil(ratio * float64(percents) / 100.0))
	}

	return fmt.Sprintf("%d%%", weightedPercents)
}

// DistributePositiveIntOrPercent distributes a given int or percentage value over zones in relation to
// the given total value. In case the total value is evenly divisible over the zones, this
// always just returns the initial percentage. Otherwise, the total value is used to determine
// the weight of a specific zone in relation to the other zones and adapt the given percentage
// accordingly.
func DistributePositiveIntOrPercent(zoneIndex int, intOrPercent intstr.IntOrString, zoneSize int, total int) intstr.IntOrString {
	if intOrPercent.Type == intstr.String {
		return intstr.FromString(DistributePercentOverZones(zoneIndex, intOrPercent.StrVal, zoneSize, total))
	}
	return intstr.FromInt(DistributeOverZones(zoneIndex, int(intOrPercent.IntVal), zoneSize))
}

// IdentifyAddressType takes a string containing an address (hostname or IP) and tries to parse it
// to an IP address in order to identify whether it is a DNS name or not.
// It returns a tuple whereby the first element is either "ip" or "hostname", and the second the
// parsed IP address of type net.IP (in case the loadBalancer is an IP address, otherwise it is nil).
func IdentifyAddressType(address string) (string, net.IP) {
	addr := net.ParseIP(address)
	addrType := "hostname"
	if addr != nil {
		addrType = "ip"
	}
	return addrType, addr
}

// ComputeClusterIP parses the provided <cidr> and sets the last byte to the value of <lastByte>.
// For example, <cidr> = 100.64.0.0/11 and <lastByte> = 10 the result would be 100.64.0.10
func ComputeClusterIP(cidr gardenv1beta1.CIDR, lastByte byte) string {
	ip, _, _ := net.ParseCIDR(string(cidr))
	ip = ip.To4()
	ip[3] = lastByte
	return ip.String()
}

// DiskSize extracts the numerical component of DiskSize strings, i.e. strings like "10Gi" and
// returns it as string, i.e. "10" will be returned. If the conversion to integer fails or if
// the pattern does not match, it will return 0.
func DiskSize(size string) int {
	regex, _ := regexp.Compile("^(\\d+)")
	i, err := strconv.Atoi(regex.FindString(size))
	if err != nil {
		return 0
	}
	return i
}

// MachineClassHash returns the SHA256-hash value of the <val> struct's representation concatenated with the
// provided <version>.
func MachineClassHash(machineClassSpec map[string]interface{}, version string) string {
	return utils.ComputeSHA256Hex([]byte(fmt.Sprintf("%s-%s", utils.HashForMap(machineClassSpec), version)))[:5]
}

// GenerateAddonConfig returns the provided <values> in case <enabled> is true. Otherwise, nil is
// being returned.
func GenerateAddonConfig(values map[string]interface{}, enabled bool) map[string]interface{} {
	v := map[string]interface{}{
		"enabled": enabled,
	}
	if enabled {
		for key, value := range values {
			v[key] = value
		}
	}
	return v
}

// GetLoadBalancerIngress takes a K8SClient, a namespace and a service name. It queries for a load balancer's technical name
// (ip address or hostname). It returns the value of the technical name whereby it always prefers the IP address (if given)
// over the hostname. It also returns the list of all load balancer ingresses.
func GetLoadBalancerIngress(client kubernetes.Interface, namespace, name string) (string, []corev1.LoadBalancerIngress, error) {
	var (
		loadBalancerIngress  string
		serviceStatusIngress []corev1.LoadBalancerIngress
	)

	service, err := client.GetService(namespace, name)
	if err != nil {
		return "", nil, err
	}

	serviceStatusIngress = service.Status.LoadBalancer.Ingress
	length := len(serviceStatusIngress)
	if length == 0 {
		return "", nil, errors.New("`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created (is your quota limit exceeded/reached?)")
	}

	if serviceStatusIngress[length-1].IP != "" {
		loadBalancerIngress = serviceStatusIngress[length-1].IP
	} else if serviceStatusIngress[length-1].Hostname != "" {
		loadBalancerIngress = serviceStatusIngress[length-1].Hostname
	} else {
		return "", nil, errors.New("`.status.loadBalancer.ingress[]` has an element which does neither contain `.ip` nor `.hostname`")
	}
	return loadBalancerIngress, serviceStatusIngress, nil
}

// ExtractShootName returns Shoot resource name extracted from provided <backupInfrastructureName>.
func ExtractShootName(backupInfrastructureName string) string {
	tokens := strings.Split(backupInfrastructureName, "-")
	return strings.Join(tokens[:len(tokens)-1], "-")
}

// GenerateBackupInfrastructureName returns BackupInfrastructure resource name created from provided <seedNamespace> and <shootUID>.
func GenerateBackupInfrastructureName(seedNamespace string, shootUID types.UID) string {
	// TODO: Remove this and use only "--" as separator, once we have all shoots deployed as per new naming conventions.
	if IsFollowingNewNamingConvention(seedNamespace) {
		return fmt.Sprintf("%s--%s", seedNamespace, utils.ComputeSHA1Hex([]byte(shootUID))[:5])
	}
	return fmt.Sprintf("%s-%s", seedNamespace, utils.ComputeSHA1Hex([]byte(shootUID))[:5])
}

// GenerateBackupNamespaceName returns Backup namespace name created from provided <backupInfrastructureName>.
func GenerateBackupNamespaceName(backupInfrastructureName string) string {
	return fmt.Sprintf("%s--%s", BackupNamespacePrefix, backupInfrastructureName)
}

// IsFollowingNewNamingConvention determines whether the new naming convention followed for shoot resources.
// TODO: Remove this and use only "--" as separator, once we have all shoots deployed as per new naming conventions.
func IsFollowingNewNamingConvention(seedNamespace string) bool {
	return len(strings.Split(seedNamespace, "--")) > 2
}

// ReplaceCloudProviderConfigKey replaces a key with the new value in the given cloud provider config.
func ReplaceCloudProviderConfigKey(cloudProviderConfig, separator, key, value string) string {
	keyValueRegexp := regexp.MustCompile(fmt.Sprintf(`(\Q%s\E%s)([^\n]*)`, key, separator))
	return keyValueRegexp.ReplaceAllString(cloudProviderConfig, fmt.Sprintf(`${1}%q`, strings.Replace(value, `$`, `$$`, -1)))
}

type errorWithCode struct {
	code    gardenv1beta1.ErrorCode
	message string
}

// NewErrorWithCode creates a new error that additionally exposes the given code via the Coder interface.
func NewErrorWithCode(code gardenv1beta1.ErrorCode, message string) error {
	return &errorWithCode{code, message}
}

func (e *errorWithCode) Code() gardenv1beta1.ErrorCode {
	return e.code
}

func (e *errorWithCode) Error() string {
	return e.message
}

var (
	unauthorizedRegexp           = regexp.MustCompile(`(?i)(Unauthorized|InvalidClientTokenId|SignatureDoesNotMatch|Authentication failed|AuthFailure|AuthorizationFailed|invalid character|invalid_grant|invalid_client|Authorization Profile was not found|cannot fetch token|no active subscriptions)`)
	quotaExceededRegexp          = regexp.MustCompile(`(?i)(LimitExceeded|Quota)`)
	insufficientPrivilegesRegexp = regexp.MustCompile(`(?i)(AccessDenied|Forbidden|deny|denied)`)
	dependenciesRegexp           = regexp.MustCompile(`(?i)(PendingVerification|Access Not Configured|accessNotConfigured|DependencyViolation|OptInRequired|DeleteConflict|Conflict)`)
)

func determineErrorCode(message string) gardenv1beta1.ErrorCode {
	switch {
	case unauthorizedRegexp.MatchString(message):
		return gardenv1beta1.ErrorInfraUnauthorized
	case quotaExceededRegexp.MatchString(message):
		return gardenv1beta1.ErrorInfraQuotaExceeded
	case insufficientPrivilegesRegexp.MatchString(message):
		return gardenv1beta1.ErrorInfraInsufficientPrivileges
	case dependenciesRegexp.MatchString(message):
		return gardenv1beta1.ErrorInfraDependencies
	default:
		return ""
	}
}

// DetermineError determines the Garden error code for the given error message.
func DetermineError(message string) error {
	code := determineErrorCode(message)
	if code == "" {
		return errors.New(message)
	}

	return &errorWithCode{code, message}
}

// ProjectForNamespace returns the project object responsible for a given <namespace>. It tries to identify the project object by looking for the namespace
// name in the project statuses.
func ProjectForNamespace(projectLister gardenlisters.ProjectLister, namespaceName string) (*gardenv1beta1.Project, error) {
	projectList, err := projectLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, project := range projectList {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespaceName {
			return project, nil
		}
	}

	return nil, apierrors.NewNotFound(gardenv1beta1.Resource("Project"), fmt.Sprintf("for namespace %s", namespaceName))
}

// ProjectNameForNamespace determines the project name for a given <namespace>. It tries to identify it first per the namespace's ownerReferences.
// If it doesn't help then it will check whether the project name is a label on the namespace object. If it doesn't help then the name can be inferred
// from the namespace name in case it is prefixed with the project prefix. If none of those approaches the namespace name itself is returned as project
// name.
func ProjectNameForNamespace(namespace *corev1.Namespace) string {
	for _, ownerReference := range namespace.OwnerReferences {
		if ownerReference.Kind == "Project" {
			return ownerReference.Name
		}
	}

	if name, ok := namespace.Labels[ProjectName]; ok {
		return name
	}

	if nameSplit := strings.Split(namespace.Name, ProjectPrefix); len(nameSplit) > 1 {
		return nameSplit[1]
	}

	return namespace.Name
}

// MergeOwnerReferences merges the newReferences with the list of existing references.
func MergeOwnerReferences(references []metav1.OwnerReference, newReferences ...metav1.OwnerReference) []metav1.OwnerReference {
	uids := make(map[types.UID]struct{})
	for _, reference := range references {
		uids[reference.UID] = struct{}{}
	}

	for _, newReference := range newReferences {
		if _, ok := uids[newReference.UID]; !ok {
			references = append(references, newReference)
		}
	}

	return references
}

// HasInitializer checks whether the passed name is part of the pending initializers.
func HasInitializer(initializers *metav1.Initializers, name string) bool {
	if initializers == nil {
		return false
	}
	for _, initializer := range initializers.Pending {
		if initializer.Name == name {
			return true
		}
	}
	return false
}

// ReadLeaderElectionRecord returns the leader election record for a given lock type and a namespace/name combination.
func ReadLeaderElectionRecord(k8sClient kubernetes.Interface, lock, namespace, name string) (*resourcelock.LeaderElectionRecord, error) {
	var (
		leaderElectionRecord resourcelock.LeaderElectionRecord
		annotations          map[string]string
	)

	switch lock {
	case resourcelock.EndpointsResourceLock:
		endpoint, err := k8sClient.Kubernetes().CoreV1().Endpoints(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		annotations = endpoint.Annotations
	case resourcelock.ConfigMapsResourceLock:
		configmap, err := k8sClient.Kubernetes().CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		annotations = configmap.Annotations
	default:
		return nil, fmt.Errorf("Unknown lock type: %s", lock)
	}

	leaderElection, ok := annotations[resourcelock.LeaderElectionRecordAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("Could not find key %s in annotations", resourcelock.LeaderElectionRecordAnnotationKey)
	}

	if err := json.Unmarshal([]byte(leaderElection), &leaderElectionRecord); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal leader election record: %+v", err)
	}

	return &leaderElectionRecord, nil
}

// GardenerDeletionGracePeriod is the default grace period for Gardener's force deletion methods.
var GardenerDeletionGracePeriod = 5 * time.Minute

// ShouldObjectBeRemoved determines whether the given object should be gone now.
// This is calculated by first checking the deletion timestamp of an object: If the deletion timestamp
// is unset, the object should not be removed - i.e. this returns false.
// Otherwise, it is checked whether the deletionTimestamp is before the current time minus the
// grace period.
func ShouldObjectBeRemoved(obj metav1.Object, gracePeriod time.Duration) bool {
	deletionTimestamp := obj.GetDeletionTimestamp()
	if deletionTimestamp == nil {
		return false
	}

	return deletionTimestamp.Time.Before(time.Now().Add(-gracePeriod))
}

// DeleteLoggingStack deletes all resource of the EFK logging stack in the given namespace.
func DeleteLoggingStack(k8sClient kubernetes.Interface, namespace string) error {
	if k8sClient == nil {
		return fmt.Errorf("require kubernetes client")
	}

	var (
		services     = []string{"kibana-logging", "elasticsearch-logging", "fluentd-es"}
		configmaps   = []string{"kibana-object-registration", "kibana-saved-objects", "curator-hourly-config", "curator-daily-config", "fluent-bit-config", "fluentd-es-config", "es-configmap"}
		statefulsets = []string{"elasticsearch-logging", "fluentd-es"}
		cronjobs     = []string{"hourly-curator", "daily-curator"}
	)

	if err := k8sClient.DeleteDeployment(namespace, "kibana-logging"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := k8sClient.DeleteDaemonSet(namespace, "fluent-bit"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	for _, name := range statefulsets {
		if err := k8sClient.DeleteStatefulSet(namespace, name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, name := range cronjobs {
		if err := k8sClient.DeleteCronJob(namespace, name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	if err := k8sClient.DeleteIngress(namespace, "kibana"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := k8sClient.DeleteSecret(namespace, "kibana-basic-auth"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := k8sClient.DeleteClusterRoleBinding("fluent-bit-read"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := k8sClient.DeleteClusterRole("fluent-bit-read"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := k8sClient.DeleteServiceAccount(namespace, "fluent-bit"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := k8sClient.DeleteHorizontalPodAutoscaler(namespace, "fluentd-es"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	for _, name := range services {
		if err := k8sClient.DeleteService(namespace, name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, name := range configmaps {
		if err := k8sClient.DeleteConfigMap(namespace, name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// DeleteAlertmanager deletes all resources of the Alertmanager in a given namespace.
func DeleteAlertmanager(k8sClient kubernetes.Interface, namespace string) error {
	var (
		services = []string{"alertmanager-client", "alertmanager"}
		secrets  = []string{"alertmanager-basic-auth", "alertmanager-tls", "alertmanager-config"}
	)

	if err := k8sClient.DeleteStatefulSet(namespace, AlertManagerStatefulSetName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := k8sClient.DeleteIngress(namespace, "alertmanager"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	for _, svc := range services {
		if err := k8sClient.DeleteService(namespace, svc); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, secret := range secrets {
		if err := k8sClient.DeleteSecret(namespace, secret); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
