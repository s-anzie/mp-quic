package crypto

import (
	"fmt"
	"hash/fnv"

	lru "github.com/hashicorp/golang-lru"
	"github.com/s-anzie/mp-quic/internal/protocol"
)

var (
	compressedCertsCache *lru.Cache
)

func getCompressedCert(chain [][]byte, pCommonSetHashes, pCachedHashes []byte) ([]byte, error) {
	// Hash all inputs
	hasher := fnv.New64a()
	for _, v := range chain {
		hasher.Write(v)
	}
	hasher.Write(pCommonSetHashes)
	hasher.Write(pCachedHashes)
	hash := hasher.Sum64()

	var result []byte

	resultI, isCached := compressedCertsCache.Get(hash)
	if isCached {
		result = resultI.([]byte)
	} else {
		var err error
		result, err = compressChain(chain, pCommonSetHashes, pCachedHashes)
		if err != nil {
			return nil, err
		}
		compressedCertsCache.Add(hash, result)
	}

	return result, nil
}

func init() {
	var err error
	compressedCertsCache, err = lru.New(protocol.NumCachedCertificates)
	if err != nil {
		panic(fmt.Sprintf("fatal error in quic-go: could not create lru cache: %s", err.Error()))
	}
}
