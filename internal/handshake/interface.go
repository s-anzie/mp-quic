package handshake

import (
	"net"

	"github.com/s-anzie/mp-quic/internal/crypto"
	"github.com/s-anzie/mp-quic/internal/protocol"
)

// Sealer seals a packet
type Sealer interface {
	Seal(dst, src []byte, packetNumber protocol.PacketNumber, associatedData []byte) []byte
	Overhead() int
}

// CryptoSetup is a crypto setup
type CryptoSetup interface {
	Open(dst, src []byte, packetNumber protocol.PacketNumber, associatedData []byte) ([]byte, protocol.EncryptionLevel, error)
	HandleCryptoStream() error
	// TODO: clean up this interface
	DiversificationNonce() []byte   // only needed for cryptoSetupServer
	SetDiversificationNonce([]byte) // only needed for cryptoSetupClient

	GetSealer() (protocol.EncryptionLevel, Sealer)
	GetSealerWithEncryptionLevel(protocol.EncryptionLevel) (Sealer, error)
	GetSealerForCryptoStream() (protocol.EncryptionLevel, Sealer)
	SetDerivationKey(otherKey []byte, myKey []byte, otherIV []byte, myIV []byte)
	GetOncesObitID() ([]byte, []byte, []byte)
	SetOncesObitID(diversifi []byte, obit []byte, ID []byte)
	SetRemoteAddr(addr net.Addr)
	GetAEADs() (crypto.AEAD, crypto.AEAD, crypto.AEAD)
}

// TransportParameters are parameters sent to the peer during the handshake
type TransportParameters struct {
	RequestConnectionIDTruncation bool
	CacheHandshake                bool
}
