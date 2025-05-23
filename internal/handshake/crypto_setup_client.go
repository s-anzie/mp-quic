package handshake

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"

	"github.com/s-anzie/mp-quic/internal/crypto"
	"github.com/s-anzie/mp-quic/internal/protocol"
	"github.com/s-anzie/mp-quic/internal/utils"
	"github.com/s-anzie/mp-quic/qerr"
)

type cryptoSetupClient struct {
	mutex sync.RWMutex

	hostname           string
	connID             protocol.ConnectionID
	version            protocol.VersionNumber
	negotiatedVersions []protocol.VersionNumber

	cryptoStream io.ReadWriter

	serverConfig *serverConfigClient

	stk              []byte
	sno              []byte
	nonc             []byte
	proof            []byte
	chloForSignature []byte
	lastSentCHLO     []byte
	certManager      crypto.CertManager

	// Easier to cache then
	certData []byte
	scfgData []byte

	divNonceChan         chan []byte
	diversificationNonce []byte

	clientHelloCounter int
	serverVerified     bool // has the certificate chain and the proof already been verified
	keyDerivation      QuicCryptoKeyDerivationFunction
	keyExchange        KeyExchangeFunction

	receivedSecurePacket bool
	nullAEAD             crypto.AEAD
	secureAEAD           crypto.AEAD
	forwardSecureAEAD    crypto.AEAD
	aeadChanged          chan<- protocol.EncryptionLevel

	params               *TransportParameters
	connectionParameters ConnectionParametersManager
}

var _ CryptoSetup = &cryptoSetupClient{}

var (
	errNoObitForClientNonce             = errors.New("CryptoSetup BUG: No OBIT for client nonce available")
	errClientNonceAlreadyExists         = errors.New("CryptoSetup BUG: A client nonce was already generated")
	errConflictingDiversificationNonces = errors.New("Received two different diversification nonces")
)

// NewCryptoSetupClient creates a new CryptoSetup instance for a client
func NewCryptoSetupClient(
	hostname string,
	connID protocol.ConnectionID,
	version protocol.VersionNumber,
	cryptoStream io.ReadWriter,
	tlsConfig *tls.Config,
	connectionParameters ConnectionParametersManager,
	aeadChanged chan<- protocol.EncryptionLevel,
	params *TransportParameters,
	negotiatedVersions []protocol.VersionNumber,
) (CryptoSetup, error) {
	return &cryptoSetupClient{
		hostname:             hostname,
		connID:               connID,
		version:              version,
		cryptoStream:         cryptoStream,
		certManager:          crypto.NewCertManager(tlsConfig),
		connectionParameters: connectionParameters,
		keyDerivation:        crypto.DeriveQuicCryptoAESKeys,
		keyExchange:          getEphermalKEX,
		nullAEAD:             crypto.NewNullAEAD(protocol.PerspectiveClient, version),
		aeadChanged:          aeadChanged,
		negotiatedVersions:   negotiatedVersions,
		divNonceChan:         make(chan []byte),
		params:               params,
	}, nil
}

func writeBinaryFile(filename string, data []byte) {
	// If previous cache present, let it as it
	if _, err := os.Stat(filename); err == nil {
		return
	}

	f, err := os.Create(filename)
	if err != nil {
		utils.Infof("Cannot write cache to %s", filename)
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	// First is SNI
	binary.Write(w, binary.LittleEndian, data)
	w.Flush()
}

func readBinaryFile(filename string) ([]byte, error) {
	// If no cache, skip this
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, err
	}

	f, err := os.Open(filename)
	if err != nil {
		utils.Infof("error when opening handshake cache %s: %s", filename, err)
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (h *cryptoSetupClient) cacheHandshake() {
	// First is SNI
	cacheSNIFilename := "cache_sni_" + h.hostname
	writeBinaryFile(cacheSNIFilename, []byte(h.hostname))
	// STK
	cacheSTKFilename := "cache_stk_" + h.hostname
	writeBinaryFile(cacheSTKFilename, []byte(h.stk))
	// Collect certData
	cacheCERTFilename := "cache_cert_" + h.hostname
	writeBinaryFile(cacheCERTFilename, []byte(h.certData))
	// Server Config
	cacheSCFGFilename := "cache_scfg_" + h.hostname
	writeBinaryFile(cacheSCFGFilename, []byte(h.scfgData))
}

func (h *cryptoSetupClient) useHandshakeCache() {
	// First is SNI
	cacheSNIFilename := "cache_sni_" + h.hostname
	sni, err := readBinaryFile(cacheSNIFilename)
	if err != nil {
		utils.Infof("%s", err)
		return
	}
	hname := string(sni)
	if h.hostname != hname {
		utils.Infof("error when comparing SNI: %s != %s", h.hostname, hname)
		return
	}

	// STK
	cacheSTKFilename := "cache_stk_" + h.hostname
	stk, err := readBinaryFile(cacheSTKFilename)
	if err != nil {
		utils.Infof("%s", err)
		return
	}
	h.stk = stk

	// Collect certData
	cacheCERTFilename := "cache_cert_" + h.hostname
	certData, err := readBinaryFile(cacheCERTFilename)
	if err != nil {
		utils.Infof("%s", err)
		return
	}
	h.certData = certData
	err = h.certManager.SetData(h.certData)
	if err != nil {
		utils.Infof("error when parsing certData: %s", err)
		return
	}
	// Server Config
	cacheSCFGFilename := "cache_scfg_" + h.hostname
	scfgData, err := readBinaryFile(cacheSCFGFilename)
	if err != nil {
		utils.Infof("%s", err)
		return
	}

	h.scfgData = scfgData
	h.serverConfig, err = parseServerConfig(h.scfgData)
	if err != nil {
		utils.Infof("error when parsing server config: %s", err)
		return
	}

	// TODO Check if server config is expired

	// Generate client nonce
	err = h.generateClientNonce()
	if err != nil {
		utils.Infof("error when generating client nonce: %s", err)
		return
	}

	// If everything went well, the server could be considered as verified
	h.serverVerified = true
}

func (h *cryptoSetupClient) HandleCryptoStream() error {
	messageChan := make(chan HandshakeMessage)
	errorChan := make(chan error)

	if h.params.CacheHandshake {
		h.useHandshakeCache()
	}

	go func() {
		for {
			message, err := ParseHandshakeMessage(h.cryptoStream)
			if err != nil {
				errorChan <- qerr.Error(qerr.HandshakeFailed, err.Error())
				return
			}
			messageChan <- message
		}
	}()

	for {
		err := h.maybeUpgradeCrypto()
		if err != nil {
			return err
		}

		h.mutex.RLock()
		sendCHLO := h.secureAEAD == nil
		h.mutex.RUnlock()

		if sendCHLO {
			err = h.sendCHLO()
			if err != nil {
				return err
			}
		}

		var message HandshakeMessage
		select {
		case divNonce := <-h.divNonceChan:
			if len(h.diversificationNonce) != 0 && !bytes.Equal(h.diversificationNonce, divNonce) {
				return errConflictingDiversificationNonces
			}
			h.diversificationNonce = divNonce
			// there's no message to process, but we should try upgrading the crypto again
			continue
		case message = <-messageChan:
		case err = <-errorChan:
			return err
		}

		utils.Debugf("Got %s", message)
		switch message.Tag {
		case TagREJ:
			err = h.handleREJMessage(message.Data)
		case TagSHLO:
			err = h.handleSHLOMessage(message.Data)
			if h.params.CacheHandshake && err == nil {
				// It worked, cache the data
				h.cacheHandshake()
			}
		default:
			return qerr.InvalidCryptoMessageType
		}
		if err != nil {
			return err
		}
	}
}

func (h *cryptoSetupClient) handleREJMessage(cryptoData map[Tag][]byte) error {
	var err error

	if stk, ok := cryptoData[TagSTK]; ok {
		h.stk = stk
	}

	if sno, ok := cryptoData[TagSNO]; ok {
		h.sno = sno
	}

	// TODO: what happens if the server sends a different server config in two packets?
	if scfg, ok := cryptoData[TagSCFG]; ok {
		h.serverConfig, err = parseServerConfig(scfg)
		if err != nil {
			return err
		}

		if h.serverConfig.IsExpired() {
			return qerr.CryptoServerConfigExpired
		}

		h.scfgData = scfg

		// now that we have a server config, we can use its OBIT value to generate a client nonce
		if len(h.nonc) == 0 {
			err = h.generateClientNonce()
			if err != nil {
				return err
			}
		}
	}

	if proof, ok := cryptoData[TagPROF]; ok {
		h.proof = proof
		h.chloForSignature = h.lastSentCHLO
	}

	if crt, ok := cryptoData[TagCERT]; ok {
		err := h.certManager.SetData(crt)
		if err != nil {
			return qerr.Error(qerr.InvalidCryptoMessageParameter, "Certificate data invalid")
		}
		h.certData = crt

		err = h.certManager.Verify("quic.clemente.io") // h.hostname)
		if err != nil {
			utils.Infof("Certificate validation failed: %s", err.Error())
			return qerr.ProofInvalid
		}
	}

	if h.serverConfig != nil && len(h.proof) != 0 && h.certManager.GetLeafCert() != nil {
		validProof := h.certManager.VerifyServerProof(h.proof, h.chloForSignature, h.serverConfig.Get())
		if !validProof {
			utils.Infof("Server proof verification failed")
			return qerr.ProofInvalid
		}

		h.serverVerified = true
	}

	return nil
}

func (h *cryptoSetupClient) handleSHLOMessage(cryptoData map[Tag][]byte) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if !h.receivedSecurePacket {
		return qerr.Error(qerr.CryptoEncryptionLevelIncorrect, "unencrypted SHLO message")
	}

	if sno, ok := cryptoData[TagSNO]; ok {
		h.sno = sno
	}

	serverPubs, ok := cryptoData[TagPUBS]
	if !ok {
		return qerr.Error(qerr.CryptoMessageParameterNotFound, "PUBS")
	}

	verTag, ok := cryptoData[TagVER]
	if !ok {
		return qerr.Error(qerr.InvalidCryptoMessageParameter, "server hello missing version list")
	}
	if !h.validateVersionList(verTag) {
		return qerr.Error(qerr.VersionNegotiationMismatch, "Downgrade attack detected")
	}

	nonce := append(h.nonc, h.sno...)

	ephermalSharedSecret, err := h.serverConfig.kex.CalculateSharedKey(serverPubs)
	if err != nil {
		return err
	}

	leafCert := h.certManager.GetLeafCert()

	h.forwardSecureAEAD, err = h.keyDerivation(
		true,
		ephermalSharedSecret,
		nonce,
		h.connID,
		h.lastSentCHLO,
		h.serverConfig.Get(),
		leafCert,
		nil,
		protocol.PerspectiveClient,
	)
	if err != nil {
		return err
	}

	err = h.connectionParameters.SetFromMap(cryptoData)
	if err != nil {
		return qerr.InvalidCryptoMessageParameter
	}

	h.aeadChanged <- protocol.EncryptionForwardSecure
	close(h.aeadChanged)

	return nil
}

func (h *cryptoSetupClient) validateVersionList(verTags []byte) bool {
	if len(h.negotiatedVersions) == 0 {
		return true
	}
	if len(verTags)%4 != 0 || len(verTags)/4 != len(h.negotiatedVersions) {
		return false
	}

	b := bytes.NewReader(verTags)
	for _, negotiatedVersion := range h.negotiatedVersions {
		verTag, err := utils.LittleEndian.ReadUint32(b)
		if err != nil { // should never occur, since the length was already checked
			return false
		}
		ver := protocol.VersionTagToNumber(verTag)
		if !protocol.IsSupportedVersion(protocol.SupportedVersions, ver) {
			ver = protocol.VersionUnsupported
		}
		if ver != negotiatedVersion {
			return false
		}
	}
	return true
}

func (h *cryptoSetupClient) Open(dst, src []byte, packetNumber protocol.PacketNumber, associatedData []byte) ([]byte, protocol.EncryptionLevel, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if h.forwardSecureAEAD != nil {
		data, err := h.forwardSecureAEAD.Open(dst, src, packetNumber, associatedData)
		if err == nil {
			return data, protocol.EncryptionForwardSecure, nil
		}
		return nil, protocol.EncryptionUnspecified, err
	}

	if h.secureAEAD != nil {
		data, err := h.secureAEAD.Open(dst, src, packetNumber, associatedData)
		if err == nil {
			h.receivedSecurePacket = true
			return data, protocol.EncryptionSecure, nil
		}
		if h.receivedSecurePacket {
			return nil, protocol.EncryptionUnspecified, err
		}
	}
	res, err := h.nullAEAD.Open(dst, src, packetNumber, associatedData)
	if err != nil {
		return nil, protocol.EncryptionUnspecified, err
	}
	return res, protocol.EncryptionUnencrypted, nil
}

func (h *cryptoSetupClient) GetSealer() (protocol.EncryptionLevel, Sealer) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	if h.forwardSecureAEAD != nil {
		return protocol.EncryptionForwardSecure, h.forwardSecureAEAD
	} else if h.secureAEAD != nil {
		return protocol.EncryptionSecure, h.secureAEAD
	} else {
		return protocol.EncryptionUnencrypted, h.nullAEAD
	}
}

func (h *cryptoSetupClient) GetSealerForCryptoStream() (protocol.EncryptionLevel, Sealer) {
	return protocol.EncryptionUnencrypted, h.nullAEAD
}

func (h *cryptoSetupClient) GetSealerWithEncryptionLevel(encLevel protocol.EncryptionLevel) (Sealer, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	switch encLevel {
	case protocol.EncryptionUnencrypted:
		return h.nullAEAD, nil
	case protocol.EncryptionSecure:
		if h.secureAEAD == nil {
			return nil, errors.New("CryptoSetupClient: no secureAEAD")
		}
		return h.secureAEAD, nil
	case protocol.EncryptionForwardSecure:
		if h.forwardSecureAEAD == nil {
			return nil, errors.New("CryptoSetupClient: no forwardSecureAEAD")
		}
		return h.forwardSecureAEAD, nil
	}
	return nil, errors.New("CryptoSetupClient: no encryption level specified")
}

func (h *cryptoSetupClient) DiversificationNonce() []byte {
	panic("not needed for cryptoSetupClient")
}

func (h *cryptoSetupClient) SetDiversificationNonce(data []byte) {
	h.divNonceChan <- data
}

func (h *cryptoSetupClient) sendCHLO() error {
	h.clientHelloCounter++
	if h.clientHelloCounter > protocol.MaxClientHellos {
		return qerr.Error(qerr.CryptoTooManyRejects, fmt.Sprintf("More than %d rejects", protocol.MaxClientHellos))
	}

	b := &bytes.Buffer{}

	tags, err := h.getTags()
	if err != nil {
		return err
	}
	h.addPadding(tags)
	message := HandshakeMessage{
		Tag:  TagCHLO,
		Data: tags,
	}

	utils.Debugf("Sending %s", message)
	message.Write(b)

	_, err = h.cryptoStream.Write(b.Bytes())
	if err != nil {
		return err
	}

	h.lastSentCHLO = b.Bytes()

	return nil
}

func (h *cryptoSetupClient) getTags() (map[Tag][]byte, error) {
	tags, err := h.connectionParameters.GetHelloMap()
	if err != nil {
		return nil, err
	}
	tags[TagSNI] = []byte(h.hostname)
	tags[TagPDMD] = []byte("X509")

	ccs := h.certManager.GetCommonCertificateHashes()
	if len(ccs) > 0 {
		tags[TagCCS] = ccs
	}

	versionTag := make([]byte, 4)
	binary.LittleEndian.PutUint32(versionTag, protocol.VersionNumberToTag(h.version))
	tags[TagVER] = versionTag

	if h.params.RequestConnectionIDTruncation {
		tags[TagTCID] = []byte{0, 0, 0, 0}
	}
	if len(h.stk) > 0 {
		tags[TagSTK] = h.stk
	}
	if len(h.sno) > 0 {
		tags[TagSNO] = h.sno
	}

	if h.serverConfig != nil {
		tags[TagSCID] = h.serverConfig.ID

		leafCert := h.certManager.GetLeafCert()
		if leafCert != nil {
			certHash, _ := h.certManager.GetLeafCertHash()
			xlct := make([]byte, 8)
			binary.LittleEndian.PutUint64(xlct, certHash)

			tags[TagNONC] = h.nonc
			tags[TagXLCT] = xlct
			tags[TagKEXS] = []byte("C255")
			tags[TagAEAD] = []byte("AESG")
			tags[TagPUBS] = h.serverConfig.kex.PublicKey() // TODO: check if 3 bytes need to be prepended
		}
	}

	return tags, nil
}

// add a TagPAD to a tagMap, such that the total size will be bigger than the ClientHelloMinimumSize
func (h *cryptoSetupClient) addPadding(tags map[Tag][]byte) {
	var size int
	for _, tag := range tags {
		size += 8 + len(tag) // 4 bytes for the tag + 4 bytes for the offset + the length of the data
	}
	paddingSize := protocol.ClientHelloMinimumSize - size
	if paddingSize > 0 {
		tags[TagPAD] = bytes.Repeat([]byte{0}, paddingSize)
	}
}

func (h *cryptoSetupClient) maybeUpgradeCrypto() error {
	if !h.serverVerified {
		return nil
	}

	h.mutex.Lock()
	defer h.mutex.Unlock()

	leafCert := h.certManager.GetLeafCert()
	if h.secureAEAD == nil && (h.serverConfig != nil && len(h.serverConfig.sharedSecret) > 0 && len(h.nonc) > 0 && len(leafCert) > 0 && len(h.diversificationNonce) > 0 && len(h.lastSentCHLO) > 0) {
		var err error
		var nonce []byte
		if h.sno == nil {
			nonce = h.nonc
		} else {
			nonce = append(h.nonc, h.sno...)
		}

		h.secureAEAD, err = h.keyDerivation(
			false,
			h.serverConfig.sharedSecret,
			nonce,
			h.connID,
			h.lastSentCHLO,
			h.serverConfig.Get(),
			leafCert,
			h.diversificationNonce,
			protocol.PerspectiveClient,
		)
		if err != nil {
			return err
		}

		h.aeadChanged <- protocol.EncryptionSecure
	}

	return nil
}

func (h *cryptoSetupClient) generateClientNonce() error {
	if len(h.nonc) > 0 {
		return errClientNonceAlreadyExists
	}

	nonc := make([]byte, 32)
	binary.BigEndian.PutUint32(nonc, uint32(time.Now().Unix()))

	if len(h.serverConfig.obit) != 8 {
		return errNoObitForClientNonce
	}

	copy(nonc[4:12], h.serverConfig.obit)

	_, err := rand.Read(nonc[12:])
	if err != nil {
		return err
	}

	h.nonc = nonc
	return nil
}
func (h *cryptoSetupClient) SetDerivationKey(otherKey []byte, myKey []byte, otherIV []byte, myIV []byte) {

}

func (h *cryptoSetupClient) GetOncesObitID() ([]byte, []byte, []byte) {
	return h.diversificationNonce, nil, nil
}
func (h *cryptoSetupClient) SetOncesObitID(diversifi []byte, obit []byte, ID []byte) {
	h.diversificationNonce = diversifi
}
func (h *cryptoSetupClient) SetRemoteAddr(addr net.Addr) {

}
func (h *cryptoSetupClient) GetAEADs() (crypto.AEAD, crypto.AEAD, crypto.AEAD) {
	return h.forwardSecureAEAD, h.secureAEAD, h.nullAEAD
}
