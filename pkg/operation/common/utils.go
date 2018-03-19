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
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func DistributeOverZones(zoneIndex, size, zoneSize int) int {
	first := size / zoneSize
	second := 0
	if zoneIndex < (size % zoneSize) {
		second = 1
	}
	return first + second
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

// CheckConfirmationDeletionTimestampValid checks whether an annotation with the key of the constant <ConfirmationDeletionTimestamp>
// variable exists on the provided <shoot> object and if yes, whether its value is equal to the Shoot's
// '.metadata.deletionTimestamp' value. In that case, it returns true, otherwise false.
func CheckConfirmationDeletionTimestampValid(objectMeta metav1.ObjectMeta) bool {
	deletionTimestamp := objectMeta.DeletionTimestamp
	if !metav1.HasAnnotation(objectMeta, ConfirmationDeletionTimestamp) || deletionTimestamp == nil {
		return false
	}
	timestamp, err := time.Parse(time.RFC3339, objectMeta.Annotations[ConfirmationDeletionTimestamp])
	if err != nil {
		return false
	}
	confirmationDeletionTimestamp := metav1.NewTime(timestamp)
	return confirmationDeletionTimestamp.Equal(deletionTimestamp)
}
