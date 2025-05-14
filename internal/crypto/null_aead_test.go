package crypto

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/s-anzie/mp-quic/internal/protocol"
)

var _ = Describe("NullAEAD", func() {
	It("selects the right FVN variant", func() {
		Expect(NewNullAEAD(protocol.PerspectiveClient, protocol.Version39)).To(Equal(&nullAEADFNV128a{
			perspective: protocol.PerspectiveClient,
		}))
		Expect(NewNullAEAD(protocol.PerspectiveClient, protocol.VersionTLS)).To(Equal(&nullAEADFNV64a{}))
	})
})
