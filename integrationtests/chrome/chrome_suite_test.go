package chrome_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/s-anzie/mp-quic/integrationtests/tools/testserver"
	"github.com/s-anzie/mp-quic/internal/protocol"
	"github.com/s-anzie/mp-quic/internal/utils"

	_ "github.com/s-anzie/mp-quic/integrationtests/tools/testlog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const (
	nChromeRetries = 8

	dataLen     = 500 * 1024       // 500 KB
	dataLongLen = 50 * 1024 * 1024 // 50 MB
)

var (
	nFilesUploaded     int32 // should be used atomically
	testEndpointCalled utils.AtomicBool
	doneCalled         utils.AtomicBool
	version            protocol.VersionNumber
)

func TestChrome(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chrome Suite")
}

func init() {
	// Requires the len & num GET parameters, e.g. /uploadtest?len=100&num=1
	http.HandleFunc("/uploadtest", func(w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()
		response := uploadHTML
		response = strings.Replace(response, "LENGTH", r.URL.Query().Get("len"), -1)
		response = strings.Replace(response, "NUM", r.URL.Query().Get("num"), -1)
		_, err := io.WriteString(w, response)
		Expect(err).NotTo(HaveOccurred())
		testEndpointCalled.Set(true)
	})

	// Requires the len & num GET parameters, e.g. /downloadtest?len=100&num=1
	http.HandleFunc("/downloadtest", func(w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()
		response := downloadHTML
		response = strings.Replace(response, "LENGTH", r.URL.Query().Get("len"), -1)
		response = strings.Replace(response, "NUM", r.URL.Query().Get("num"), -1)
		_, err := io.WriteString(w, response)
		Expect(err).NotTo(HaveOccurred())
		testEndpointCalled.Set(true)
	})

	http.HandleFunc("/uploadhandler", func(w http.ResponseWriter, r *http.Request) {
		defer GinkgoRecover()

		l, err := strconv.Atoi(r.URL.Query().Get("len"))
		Expect(err).NotTo(HaveOccurred())

		defer r.Body.Close()
		actual, err := ioutil.ReadAll(r.Body)
		Expect(err).NotTo(HaveOccurred())

		Expect(bytes.Equal(actual, testserver.GeneratePRData(l))).To(BeTrue())

		atomic.AddInt32(&nFilesUploaded, 1)
	})

	http.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) {
		doneCalled.Set(true)
	})
}

var _ = JustBeforeEach(func() {
	testserver.StartQuicServer([]protocol.VersionNumber{version})
})

var _ = AfterEach(func() {
	testserver.StopQuicServer()

	atomic.StoreInt32(&nFilesUploaded, 0)
	doneCalled.Set(false)
	testEndpointCalled.Set(false)
})

func getChromePath() string {
	if runtime.GOOS == "darwin" {
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	if path, err := exec.LookPath("google-chrome"); err == nil {
		return path
	}
	if path, err := exec.LookPath("chromium-browser"); err == nil {
		return path
	}
	Fail("No Chrome executable found.")
	return ""
}

func chromeTest(version protocol.VersionNumber, url string, blockUntilDone func()) {
	// Chrome sometimes starts but doesn't send any HTTP requests for no apparent reason.
	// Retry starting it a couple of times.
	for i := 0; i < nChromeRetries; i++ {
		if chromeTestImpl(version, url, blockUntilDone) {
			return
		}
	}
	Fail("Chrome didn't hit the testing endpoints")
}

func chromeTestImpl(version protocol.VersionNumber, url string, blockUntilDone func()) bool {
	userDataDir, err := ioutil.TempDir("", "quic-go-test-chrome-dir")
	Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(userDataDir)
	path := getChromePath()
	args := []string{
		"--disable-gpu",
		"--no-first-run=true",
		"--no-default-browser-check=true",
		"--user-data-dir=" + userDataDir,
		"--enable-quic=true",
		"--no-proxy-server=true",
		"--origin-to-force-quic-on=quic.clemente.io:443",
		fmt.Sprintf(`--host-resolver-rules=MAP quic.clemente.io:443 localhost:%s`, testserver.Port()),
		fmt.Sprintf("--quic-version=QUIC_VERSION_%d", version),
		url,
	}
	utils.Infof("Running chrome: %s '%s'", getChromePath(), strings.Join(args, "' '"))
	command := exec.Command(path, args...)
	session, err := gexec.Start(command, nil, nil)
	Expect(err).NotTo(HaveOccurred())
	defer session.Kill()
	const pollInterval = 100 * time.Millisecond
	const pollDuration = 10 * time.Second
	for i := 0; i < int(pollDuration/pollInterval); i++ {
		time.Sleep(pollInterval)
		if testEndpointCalled.Get() {
			break
		}
	}
	if !testEndpointCalled.Get() {
		return false
	}
	blockUntilDone()
	return true
}

func waitForDone() {
	Eventually(func() bool { return doneCalled.Get() }, 60).Should(BeTrue())
}

func waitForNUploaded(expected int) func() {
	return func() {
		Eventually(func() int32 {
			return atomic.LoadInt32(&nFilesUploaded)
		}, 60).Should(BeEquivalentTo(expected))
	}
}

const commonJS = `
var buf = new ArrayBuffer(LENGTH);
var prng = new Uint8Array(buf);
var seed = 1;
for (var i = 0; i < LENGTH; i++) {
	// https://en.wikipedia.org/wiki/Lehmer_random_number_generator
	seed = seed * 48271 % 2147483647;
	prng[i] = seed;
}
`

const uploadHTML = `
<html>
<body>
<script>
	console.log("Running DL test...");

  ` + commonJS + `
	for (var i = 0; i < NUM; i++) {
		var req = new XMLHttpRequest();
		req.open("POST", "/uploadhandler?len=" + LENGTH, true);
		req.send(buf);
	}
</script>
</body>
</html>
`

const downloadHTML = `
<html>
<body>
<script>
	console.log("Running DL test...");
	` + commonJS + `

	function verify(data) {
		if (data.length !== LENGTH) return false;
		for (var i = 0; i < LENGTH; i++) {
			if (data[i] !== prng[i]) return false;
		}
		return true;
	}

	var nOK = 0;
	for (var i = 0; i < NUM; i++) {
		let req = new XMLHttpRequest();
		req.responseType = "arraybuffer";
		req.open("POST", "/prdata?len=" + LENGTH, true);
		req.onreadystatechange = function () {
			if (req.readyState === XMLHttpRequest.DONE && req.status === 200) {
				if (verify(new Uint8Array(req.response))) {
					nOK++;
					if (nOK === NUM) {
						console.log("Done :)");
						var reqDone = new XMLHttpRequest();
						reqDone.open("GET", "/done");
						reqDone.send();
					}
				}
			}
		};
		req.send();
	}
</script>
</body>
</html>
`
