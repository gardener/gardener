// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodeagentosc

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
)

const gnaBinaryPathBuiltViaKo = "/ko-app/gardener-node-agent"

type mutator struct{}

func (m *mutator) Mutate(_ context.Context, newObj, _ client.Object) error {
	operatingSystemConfig, ok := newObj.(*extensionsv1alpha1.OperatingSystemConfig)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *extensionsv1alpha1.OperatingSystemConfig", newObj)
	}

	if err := modifyFile(operatingSystemConfig.Spec.Files, nodeinit.PathInitScript, func(oldContent string) string {
		return strings.ReplaceAll(oldContent,
			`cp -f "$tmp_dir/gardener-node-agent"`,
			`cp -f "$tmp_dir`+gnaBinaryPathBuiltViaKo+`"`,
		)
	}); err != nil {
		return fmt.Errorf("failed modifying file %q: %w", nodeinit.PathInitScript, err)
	}

	if file := extensionswebhook.FileWithPath(operatingSystemConfig.Spec.Files, nodeagent.PathBinary); file != nil && file.Content.ImageRef != nil {
		file.Content.ImageRef.FilePathInImage = gnaBinaryPathBuiltViaKo
	}

	return nil
}

var fciCodec = oscutils.NewFileContentInlineCodec()

func modifyFile(files []extensionsv1alpha1.File, path string, mutateFunc func(string) string) error {
	if file := extensionswebhook.FileWithPath(files, path); file != nil {
		oldContent, err := fciCodec.Decode(file.Content.Inline)
		if err != nil {
			return fmt.Errorf("failed to decode file %q: %w", path, err)
		}

		newContent := mutateFunc(string(oldContent))

		var newFileContent *extensionsv1alpha1.FileContentInline
		if newFileContent, err = fciCodec.Encode([]byte(newContent), file.Content.Inline.Encoding); err != nil {
			return fmt.Errorf("could not encode file: %q: %w", path, err)
		}
		*file.Content.Inline = *newFileContent
	}

	return nil
}
