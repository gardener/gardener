package nodeproblemdetector_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestNodeProblemDetector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Botanist Component NodeProblemDetector Suite")
}
