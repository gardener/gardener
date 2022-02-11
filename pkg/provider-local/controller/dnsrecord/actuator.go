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

package dnsrecord

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const pathEtcHosts = "/etc/hosts"

type actuator struct {
	logger logr.Logger
	lock   sync.Mutex
	common.RESTConfigContext
}

// NewActuator creates a new Actuator that updates the status of the handled DNSRecord resources.
func NewActuator() dnsrecord.Actuator {
	return &actuator{
		logger: log.Log.WithName("dnsrecord-actuator"),
	}
}

func (a *actuator) Reconcile(_ context.Context, dnsrecord *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	return a.reconcile(dnsrecord, CreateOrUpdateValuesInEtcHostsFile)
}

func (a *actuator) Delete(_ context.Context, dnsrecord *extensionsv1alpha1.DNSRecord, _ *extensionscontroller.Cluster) error {
	return a.reconcile(dnsrecord, DeleteValuesInEtcHostsFile)
}

func (a *actuator) reconcile(dnsRecord *extensionsv1alpha1.DNSRecord, mutateEtcHosts func([]byte, *extensionsv1alpha1.DNSRecord) []byte) error {
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
			a.logger.Error(err, "Error closing hosts file")
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

func (a *actuator) Migrate(ctx context.Context, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, dnsrecord, cluster)
}

func (a *actuator) Restore(ctx context.Context, dnsrecord *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, dnsrecord, cluster)
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
		sort.Strings(newEntries)
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
