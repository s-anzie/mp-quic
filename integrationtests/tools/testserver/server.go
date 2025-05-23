package testserver

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"

	quic "github.com/s-anzie/mp-quic"
	"github.com/s-anzie/mp-quic/h2quic"
	"github.com/s-anzie/mp-quic/internal/protocol"
	"github.com/s-anzie/mp-quic/internal/testdata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	dataLen     = 500 * 1024       // 500 KB
	dataLenLong = 50 * 1024 * 1024 // 50 MB
)

var (
	PRData     = GeneratePRData(dataLen)
	PRDataLong = GeneratePRData(dataLenLong)

	server *h2quic.Server
	port   string
)

func init() {
	http.HandleFunc("/prdata", func(w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()
		sl := r.URL.Query().Get("len")
		if sl != "" {
			var err error
			l, err := strconv.Atoi(sl)
			Expect(err).NotTo(HaveOccurred())
			_, err = w.Write(GeneratePRData(l))
			Expect(err).NotTo(HaveOccurred())
		} else {
			_, err := w.Write(PRData)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	http.HandleFunc("/prdatalong", func(w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()
		_, err := w.Write(PRDataLong)
		Expect(err).NotTo(HaveOccurred())
	})

	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()
		_, err := io.WriteString(w, "Hello, World!\n")
		Expect(err).NotTo(HaveOccurred())
	})

	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()
		body, err := ioutil.ReadAll(r.Body)
		Expect(err).NotTo(HaveOccurred())
		_, err = w.Write(body)
		Expect(err).NotTo(HaveOccurred())
	})
}

// See https://en.wikipedia.org/wiki/Lehmer_random_number_generator
func GeneratePRData(l int) []byte {
	res := make([]byte, l)
	seed := uint64(1)
	for i := 0; i < l; i++ {
		seed = seed * 48271 % 2147483647
		res[i] = byte(seed)
	}
	return res
}

// StartQuicServer starts a h2quic.Server.
// versions is a slice of supported QUIC versions. It may be nil, then all supported versions are used.
func StartQuicServer(versions []protocol.VersionNumber) {
	server = &h2quic.Server{
		Server: &http.Server{
			TLSConfig: testdata.GetTLSConfig(),
		},
		QuicConfig: &quic.Config{
			Versions: versions,
		},
	}

	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	Expect(err).NotTo(HaveOccurred())
	conn, err := net.ListenUDP("udp", addr)
	Expect(err).NotTo(HaveOccurred())
	port = strconv.Itoa(conn.LocalAddr().(*net.UDPAddr).Port)

	go func() {
		defer GinkgoRecover()
		server.Serve(conn)
	}()
}

func StopQuicServer() {
	Expect(server.Close()).NotTo(HaveOccurred())
}

func Port() string {
	return port
}
