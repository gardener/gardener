// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnsrecord

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

const pathEtcHosts = "/etc/hosts"

type actuator struct {
	client client.Client

	lock             sync.Mutex
	writeToHostsFile bool
}

// NewActuator creates a new Actuator that updates the status of the handled DNSRecord resources.
func NewActuator(mgr manager.Manager, writeToHostsFile bool) dnsrecord.Actuator {
	return &actuator{
		client:           mgr.GetClient(),
		writeToHostsFile: writeToHostsFile,
	}
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.reconcile(ctx, log, dnsrecord, cluster, CreateOrUpdateValuesInEtcHostsFile, updateCoreDNSRewriteRule)
}

func (a *actuator) Delete(ctx context.Context, log logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.reconcile(ctx, log, dnsrecord, cluster, DeleteValuesInEtcHostsFile, deleteCoreDNSRewriteRule)
}

func (a *actuator) reconcile(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster, mutateEtcHosts func([]byte, *extensionsv1alpha1.DNSRecord) []byte, mutateCorednsRules func(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, zone *string)) error {
	if err := a.updateCoreDNSRewritingRules(ctx, log, dnsRecord, cluster, mutateCorednsRules); err != nil {
		return err
	}

	if !a.writeToHostsFile {
		return nil
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	fileInfo, err := os.Stat(pathEtcHosts)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(pathEtcHosts, os.O_RDWR, fileInfo.Mode())
	if err != nil {
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Error(err, "Error closing hosts file")
		}
	}()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if err := file.Truncate(0); err != nil {
		return err
	}

	_, err = file.WriteAt(mutateEtcHosts(content, dnsRecord), 0)
	return err
}

func (a *actuator) Migrate(ctx context.Context, log logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, dnsrecord, cluster)
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, dnsrecord, cluster)
}

const (
	beginOfSection = "# Begin of gardener-extension-provider-local section"
	endOfSection   = "# End of gardener-extension-provider-local section"
)

// CreateOrUpdateValuesInEtcHostsFile creates or updates the values of the provided DNSRecord object in the /etc/hosts
// file.
func CreateOrUpdateValuesInEtcHostsFile(etcHostsContent []byte, dnsrecord *extensionsv1alpha1.DNSRecord) []byte {
	return reconcileEtcHostsFile(etcHostsContent, dnsrecord.Spec.Name, dnsrecord.Spec.Values, false)
}

// DeleteValuesInEtcHostsFile deletes the values of the provided DNSRecord object in the /etc/hosts file.
func DeleteValuesInEtcHostsFile(etcHostsContent []byte, dnsrecord *extensionsv1alpha1.DNSRecord) []byte {
	return reconcileEtcHostsFile(etcHostsContent, dnsrecord.Spec.Name, dnsrecord.Spec.Values, true)
}

func reconcileEtcHostsFile(etcHostsContent []byte, name string, values []string, removeEntries bool) []byte {
	var (
		oldContent = string(etcHostsContent)
		newContent = string(oldContent)

		hostnameToIPs = make(map[string][]string)

		sectionBeginIndex = strings.Index(oldContent, beginOfSection)
		sectionEndIndex   = strings.Index(oldContent, endOfSection)
		sectionExists     = sectionBeginIndex >= 0 && sectionEndIndex >= 0
	)

	if sectionExists {
		newContent = oldContent[0 : sectionBeginIndex-1]
		existingSection := oldContent[sectionBeginIndex : sectionEndIndex-1]

		for _, line := range strings.Split(existingSection, "\n") {
			split := strings.Split(line, " ")
			if len(split) != 2 {
				continue
			}

			ip, hostname := split[0], split[1]
			hostnameToIPs[hostname] = append(hostnameToIPs[hostname], ip)
		}
	}

	newContent = strings.TrimSuffix(newContent, "\n")

	if removeEntries {
		delete(hostnameToIPs, name)
	} else {
		hostnameToIPs[name] = values
	}

	var newEntries []string
	for hostname, ips := range hostnameToIPs {
		for _, ip := range ips {
			newEntries = append(newEntries, fmt.Sprintf("%s %s", ip, hostname))
		}
	}

	if len(newEntries) > 0 {
		newContent += fmt.Sprintf("\n%s\n", beginOfSection)
		slices.Sort(newEntries)
		newContent += strings.Join(newEntries, "\n")
		newContent += fmt.Sprintf("\n%s", endOfSection)
	}

	if sectionExists {
		newContent += oldContent[sectionEndIndex+len(endOfSection):]
	}

	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	return []byte(newContent)
}

func (a *actuator) updateCoreDNSRewritingRules(ctx context.Context, log logr.Logger, dnsRecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster, mutateCorednsRules func(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, zone *string)) error {
	// Only handle dns records for kube-apiserver
	if dnsRecord == nil || !strings.HasPrefix(dnsRecord.Spec.Name, "api.") || !strings.HasSuffix(dnsRecord.Spec.Name, ".local.gardener.cloud") {
		return nil
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dnsRecord.Namespace}}
	if err := a.client.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		return err
	}

	var zone *string
	if zones, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok && !strings.Contains(zones, ",") && len(cluster.Seed.Spec.Provider.Zones) > 1 {
		zone = &zones
	}

	corednsConfig := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "coredns-custom", Namespace: "gardener-extension-provider-local-coredns"}}
	if err := a.client.Get(ctx, client.ObjectKeyFromObject(corednsConfig), corednsConfig); err != nil {
		return err
	}

	originalConfig := corednsConfig.DeepCopy()
	mutateCorednsRules(corednsConfig, dnsRecord, zone)

	return a.client.Patch(ctx, corednsConfig, client.MergeFrom(originalConfig))
}

func deleteCoreDNSRewriteRule(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, zone *string) {
	delete(corednsConfig.Data, dnsRecord.Spec.Name+".override")
}

func updateCoreDNSRewriteRule(corednsConfig *corev1.ConfigMap, dnsRecord *extensionsv1alpha1.DNSRecord, zone *string) {
	istioNamespaceSuffix := ""
	if zone != nil {
		istioNamespaceSuffix = "--" + *zone
	}
	corednsConfig.Data[dnsRecord.Spec.Name+".override"] =
		"rewrite stop name regex " + regexp.QuoteMeta(dnsRecord.Spec.Name) + " istio-ingressgateway.istio-ingress" + istioNamespaceSuffix + ".svc.cluster.local answer auto"
}
