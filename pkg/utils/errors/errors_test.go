package errors

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestErrors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Errors Suite")
}

var _ = Describe("Errors", func() {
	var (
		err1, err2 error
	)
	BeforeEach(func() {
		err1 = fmt.Errorf("error 1")
		err2 = fmt.Errorf("error 2")
	})

	Describe("#WithSuppressed", func() {
		It("should return nil if the error is nil", func() {
			Expect(WithSuppressed(nil, err2)).To(BeNil())
		})

		It("should return the error if the suppressed error is nil", func() {
			Expect(WithSuppressed(err1, nil)).To(BeIdenticalTo(err1))
		})

		It("should return an error with cause and suppressed equal to the given errors", func() {
			err := WithSuppressed(err1, err2)

			Expect(errors.Cause(err)).To(BeIdenticalTo(err1))
			Expect(Suppressed(err)).To(BeIdenticalTo(err2))
		})
	})

	Describe("#Suppressed", func() {
		It("should retrieve the suppressed error", func() {
			Expect(Suppressed(WithSuppressed(err1, err2))).To(BeIdenticalTo(err2))
		})

		It("should retrieve nil if the error doesn't have a suppressed error", func() {
			Expect(Suppressed(err1)).To(BeNil())
		})
	})

	Context("withSuppressed", func() {
		Describe("#Error", func() {
			It("should return an error message consisting of the two errors", func() {
				Expect(WithSuppressed(err1, err2).Error()).To(Equal("error 1, suppressed: error 2"))
			})
		})

		Describe("#Format", func() {
			It("should correctly format the error in verbose mode", func() {
				Expect(fmt.Sprintf("%+v", WithSuppressed(err1, err2))).
					To(Equal("error 1\nsuppressed: error 2"))
			})
		})
	})
})
