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

package azurebotanist

// ApplyCreateHook does currently nothing for Azure.
func (b *AzureBotanist) ApplyCreateHook() error {
	return nil
}

// ApplyDeleteHook does currently nothing for Azure.
func (b *AzureBotanist) ApplyDeleteHook() error {
	return nil
}

// CheckIfClusterGetsScaled does currently nothing for Azure, as ScaleSets aren't supported.
func (b *AzureBotanist) CheckIfClusterGetsScaled() (bool, int, error) {
	return false, 0, nil
}
