package virtualcluster_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVirtualcluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Virtualcluster Suite")
}
