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

package utils

import (
	"net"
	"sort"
	"time"

	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/apimachinery/pkg/util/wait"
)

// LookupDNS performs a DNS lookup for the given <domain>. In case of success, it will return the list
// of records. If the domain is not resolvable, it will return nil.
func LookupDNS(domain string) []string {
	nsRecords, err := net.LookupHost(domain)
	if err == nil {
		sort.Strings(nsRecords)
		return nsRecords
	}
	return nil
}

// WaitUntilDNSNameResolvable is a helper function which takes a <domain> and waits for a maximum of five
// minutes that the domain name is resolvable by a DNS. It returns the first record of the resolution, or
// an error if it was not successful.
func WaitUntilDNSNameResolvable(domain string) (string, error) {
	var nsRecord string
	err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		nsRecords := LookupDNS(domain)
		if nsRecords != nil {
			nsRecord = nsRecords[0]
			return true, nil
		}
		logger.Logger.Info("Waiting for DNS name " + domain + " to be resolvable...")
		return false, nil
	})
	return nsRecord, err
}
