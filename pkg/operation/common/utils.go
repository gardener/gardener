// Copyright 2018 The Gardener Authors.
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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// ApplyChart takes a Kubernetes client <k8sClient>, chartRender <renderer>, path to a chart <chartPath>, name of the release <name>,
// release's namespace <namespace> and two maps <defaultValues>, <additionalValues>, and renders the template
// based on the merged result of both value maps. The resulting manifest will be applied to the cluster the
// Kubernetes client has been created for.
func ApplyChart(k8sClient kubernetes.Client, renderer chartrenderer.ChartRenderer, chartPath, name, namespace string, defaultValues, additionalValues map[string]interface{}) error {
	release, err := renderer.Render(chartPath, name, namespace, utils.MergeMaps(defaultValues, additionalValues))
	if err != nil {
		return err
	}
	return k8sClient.Apply(release.Manifest())
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
func DistributeOverZones(zoneIndex, size, zoneSize int) string {
	first := size / zoneSize
	second := 0
	if zoneIndex < (size % zoneSize) {
		second = 1
	}
	return strconv.Itoa(first + second)
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
// returns it as string, i.e. "10" will be returned.
func DiskSize(size string) string {
	regex, _ := regexp.Compile("^(\\d+)")
	return regex.FindString(size)
}

// ComputeNonMasqueradeCIDR computes the CIDR range which should be non-masqueraded (this is passed as
// command-line flag to kubelet during its start). This range is the whole service/pod network range.
func ComputeNonMasqueradeCIDR(cidr gardenv1beta1.CIDR) string {
	cidrSplit := strings.Split(string(cidr), "/")
	cidrSplit[1] = "10"
	return strings.Join(cidrSplit, "/")
}

// GenerateAddonConfig returns the provided <values> in case <isEnabled> is a boolean value which
// is true. Otherwise, nil is returned.
func GenerateAddonConfig(values map[string]interface{}, isEnabled interface{}) map[string]interface{} {
	enabled, ok := isEnabled.(bool)
	if !ok {
		enabled = false
	}
	v := make(map[string]interface{})
	if enabled {
		for key, value := range values {
			v[key] = value
		}
	}
	v["enabled"] = enabled
	return v
}

// GetLoadBalancerIngress takes a K8SClient, a namespace and a service name. It queries for a load balancer's technical name
// (ip address or hostname). It returns the value of the technical name whereby it always prefers the IP address (if given)
// over the hostname. It also returns the list of all load balancer ingresses.
func GetLoadBalancerIngress(client kubernetes.Client, namespace, name string) (string, []corev1.LoadBalancerIngress, error) {
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
		return "", nil, errors.New("`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created")
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

// GenerateTerraformVariablesEnvironment takes a <secret> and a <keyValueMap> and builds an environment which
// can be injected into the Terraformer job/pod manifest. The keys of the <keyValueMap> will be prefixed with
// 'TF_VAR_' and the value will be used to extract the respective data from the <secret>.
func GenerateTerraformVariablesEnvironment(secret *corev1.Secret, keyValueMap map[string]string) []map[string]interface{} {
	m := []map[string]interface{}{}
	for key, value := range keyValueMap {
		m = append(m, map[string]interface{}{
			"name":  fmt.Sprintf("TF_VAR_%s", key),
			"value": strings.TrimSpace(string(secret.Data[value])),
		})
	}
	return m
}

// EnsureImagePullSecrets takes a Kubernetes client <k8sClient> and a <namespace> and creates the
// image pull secrets stored in the Garden namespace and having the respective role label. After
// that it patches the default service account in that namespace by appending the names of the just
// created secrets to its .imagePullSecrets[] list.
func EnsureImagePullSecrets(k8sClient kubernetes.Client, namespace string, secrets map[string]*corev1.Secret, createSecrets bool, log *logrus.Entry) error {
	var (
		imagePullKeys       = GetSecretKeysWithPrefix(GardenRoleImagePull, secrets)
		serviceAccountName  = "default"
		serviceAccountPatch = corev1.ServiceAccount{
			ImagePullSecrets: []corev1.LocalObjectReference{},
		}
	)
	if len(imagePullKeys) == 0 {
		return nil
	}

	err := wait.PollImmediate(5*time.Second, 60*time.Second, func() (bool, error) {
		_, err := k8sClient.GetServiceAccount(namespace, serviceAccountName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("Waiting for ServiceAccount '%s' to be created in namespace '%s'...", serviceAccountName, namespace)
				if log != nil {
					log.Info(msg)
				} else {
					logger.Logger.Info(msg)
				}
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	for _, key := range imagePullKeys {
		secret := secrets[key]
		if createSecrets {
			_, err := k8sClient.CreateSecret(namespace, secret.Name, corev1.SecretTypeDockercfg, secret.Data, true)
			if err != nil {
				return err
			}
		}
		serviceAccountPatch.ImagePullSecrets = append(serviceAccountPatch.ImagePullSecrets, corev1.LocalObjectReference{
			Name: secret.Name,
		})
	}

	patch, err := json.Marshal(serviceAccountPatch)
	if err != nil {
		return err
	}
	_, err = k8sClient.PatchServiceAccount(namespace, serviceAccountName, patch)
	return err
}
