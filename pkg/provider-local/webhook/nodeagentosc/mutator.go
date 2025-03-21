// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentosc

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
)

type mutator struct{}

func (m *mutator) Mutate(_ context.Context, newObj, _ client.Object) error {
	operatingSystemConfig, ok := newObj.(*extensionsv1alpha1.OperatingSystemConfig)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *extensionsv1alpha1.OperatingSystemConfig", newObj)
	}

	if err := modifyFile(operatingSystemConfig.Spec.Files, nodeinit.PathInitScript, func(oldContent string) string {
		return strings.ReplaceAll(oldContent,
			`cp -f "$tmp_dir/gardener-node-agent"`,
			`cp -f "$tmp_dir/ko-app/gardener-node-agent"`,
		)
	}); err != nil {
		return fmt.Errorf("failed modifying file %q: %w", nodeinit.PathInitScript, err)
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
