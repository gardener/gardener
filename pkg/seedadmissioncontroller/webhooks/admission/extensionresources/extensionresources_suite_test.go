package extensionresources_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestExtensionResources(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SeedAdmissionController Admission ExtensionResources Suite")
}
