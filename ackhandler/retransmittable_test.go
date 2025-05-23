package ackhandler

import (
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/s-anzie/mp-quic/internal/wire"
)

var _ = Describe("retransmittable frames", func() {
	for fl, el := range map[wire.Frame]bool{
		&wire.AckFrame{}:             false,
		&wire.StopWaitingFrame{}:     false,
		&wire.BlockedFrame{}:         true,
		&wire.ConnectionCloseFrame{}: true,
		&wire.GoawayFrame{}:          true,
		&wire.PingFrame{}:            true,
		&wire.RstStreamFrame{}:       true,
		&wire.StreamFrame{}:          true,
		&wire.WindowUpdateFrame{}:    true,
	} {
		f := fl
		e := el
		fName := reflect.ValueOf(f).Elem().Type().Name()

		It("works for "+fName, func() {
			Expect(IsFrameRetransmittable(f)).To(Equal(e))
		})

		It("stripping non-retransmittable frames works for "+fName, func() {
			s := []wire.Frame{f}
			if e {
				Expect(stripNonRetransmittableFrames(s)).To(Equal([]wire.Frame{f}))
			} else {
				Expect(stripNonRetransmittableFrames(s)).To(BeEmpty())
			}
		})

		It("HasRetransmittableFrames works for "+fName, func() {
			Expect(HasRetransmittableFrames([]wire.Frame{f})).To(Equal(e))
		})
	}
})
