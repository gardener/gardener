// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies

// RuleBuilder is a builder for easy construction of Source.
type RuleBuilder struct {
	rule Rule
}

// NewSource creates a new instance of RuleBuilder.
func NewSource(pi *SourcePod) *RuleBuilder {
	return &RuleBuilder{rule: Rule{SourcePod: pi}}
}

// AllowHost adds `allowedHosts` as allowed Targets.
func (s *RuleBuilder) AllowHost(allowedHosts ...*Host) *RuleBuilder {
	return s.conditionalHost(true, allowedHosts...)
}

// AllowPod adds `allowedSources` as allowed Targets.
func (s *RuleBuilder) AllowPod(allowedSources ...*SourcePod) *RuleBuilder {
	allowedTargets := []*TargetPod{}
	for _, ap := range allowedSources {
		allowedTargets = append(allowedTargets, ap.AsTargetPods()...)
	}
	return s.conditionalPod(true, allowedTargets...)
}

// AllowTargetPod adds `allowTargetPods` as allowed Targets.
func (s *RuleBuilder) AllowTargetPod(allowTargetPods ...*TargetPod) *RuleBuilder {
	return s.conditionalPod(true, allowTargetPods...)
}

// DenyHost adds `deniedHosts` as denied Targets.
func (s *RuleBuilder) DenyHost(deniedHosts ...*Host) *RuleBuilder {
	return s.conditionalHost(false, deniedHosts...)
}

// DenyPod adds `deniedPods` as denied Targets.
func (s *RuleBuilder) DenyPod(deniedPods ...*SourcePod) *RuleBuilder {
	deniedTargets := []*TargetPod{}
	for _, ap := range deniedPods {
		deniedTargets = append(deniedTargets, ap.AsTargetPods()...)
	}
	return s.conditionalPod(false, deniedTargets...)
}

// DenyTargetPod adds `deniedTargets` as denied Targets.
func (s *RuleBuilder) DenyTargetPod(deniedTargets ...*TargetPod) *RuleBuilder {
	return s.conditionalPod(false, deniedTargets...)
}

// Build returns the completed Source instance.
func (s *RuleBuilder) Build() Rule {
	return s.rule
}

func (s *RuleBuilder) conditionalPod(allowed bool, pods ...*TargetPod) *RuleBuilder {
	for _, pod := range pods {
		if s.rule.SourcePod.Name == pod.Pod.Name {
			// same target and source pods are alwayds allowed to talk to eachother.
			continue
		}
		found := false
		for i, existingTarget := range s.rule.TargetPods {
			if pod.Pod.Name == existingTarget.TargetPod.Pod.Name && pod.Port.Port == existingTarget.TargetPod.Port.Port {
				s.rule.TargetPods[i].Allowed = allowed
				found = true
				break
			}
		}
		if !found {
			s.rule.TargetPods = append(s.rule.TargetPods, PodRule{TargetPod: *pod, Allowed: allowed})
		}
	}
	return s
}

func (s *RuleBuilder) conditionalHost(allowed bool, hosts ...*Host) *RuleBuilder {
	for _, host := range hosts {
		found := false
		for i, existingTarget := range s.rule.TargetHosts {

			if host.HostName == existingTarget.Host.HostName && host.Port == existingTarget.Host.Port {
				s.rule.TargetHosts[i].Allowed = allowed
				found = true
				break
			}
		}
		if !found {
			s.rule.TargetHosts = append(s.rule.TargetHosts, HostRule{Host: *host, Allowed: allowed})
		}
	}
	return s
}
