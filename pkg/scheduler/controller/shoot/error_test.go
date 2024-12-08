package shoot

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DetermineSeedError", func() {
	It("should return an DetermineSeedError", func() {
		dsErr := newDetermineSeedError(Unknown, "the %dst error message", 1)
		Expect(dsErr).To(HaveOccurred())
		Expect(dsErr).To(BeAssignableToTypeOf(&DetermineSeedError{}))
		Expect(dsErr.message).To(Equal(fmt.Sprintf("the 1st error message %s", Unknown.suffix())))
	})

	It("Should convert string to DetermineSeedError", func() {
		errMsg := fmt.Sprintf("the 1st error message %s", Unknown.suffix())
		dsErr, err := DetermineSeedErrorFromString(errMsg)
		Expect(err).NotTo(HaveOccurred())
		Expect(dsErr).NotTo(BeNil())
		Expect(dsErr).To(BeAssignableToTypeOf(&DetermineSeedError{}))
		Expect(dsErr.reason).To(Equal(Unknown))
	})

	It("Should fail if string does not match with DetermineSeedError", func() {
		dsErr, err := DetermineSeedErrorFromString("any kind of string")
		Expect(dsErr).To(BeNil())
		Expect(err).To(HaveOccurred())
	})
})
