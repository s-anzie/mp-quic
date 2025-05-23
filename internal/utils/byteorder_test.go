package utils

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/s-anzie/mp-quic/internal/protocol"
)

var _ = Describe("Byte Order", func() {
	It("says little Little Endian before QUIC 39", func() {
		Expect(GetByteOrder(protocol.Version37)).To(Equal(LittleEndian))
		Expect(GetByteOrder(protocol.Version38)).To(Equal(LittleEndian))
	})

	It("says little Little Endian for QUIC 39", func() {
		Expect(GetByteOrder(protocol.Version39)).To(Equal(BigEndian))
	})
})
