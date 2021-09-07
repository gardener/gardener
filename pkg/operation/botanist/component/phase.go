// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package component

// Phase is the phase of a component.
type Phase int

const (
	// PhaseUnknown is in an unknown component phase.
	PhaseUnknown Phase = iota
	// PhaseEnabled is when a component was enabled before and it's still active.
	PhaseEnabled
	// PhaseDisabled is when a component was disabled before and it's still disabled.
	PhaseDisabled
	// PhaseEnabling is when a component was disabled before, but it's being activated.
	PhaseEnabling
	// PhaseDisabling is when a component was enabled before, but it's being disabled.
	PhaseDisabling
)

// Done returns a completed phase. e.g.
// Enabling -> Enabled
// Disabling -> Disabled
// otherwise returns the same phase.
func (s Phase) Done() Phase {
	switch s {
	case PhaseEnabling:
		return PhaseEnabled
	case PhaseDisabling:
		return PhaseDisabled
	case PhaseEnabled, PhaseDisabled, PhaseUnknown:
		return s
	default:
		return PhaseUnknown
	}
}
