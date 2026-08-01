package api

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Signer struct {
	Prefix string
	Key    string
	Now    func() time.Time
	Nonce  func() string
}

func NewSigner(prefix, key string) Signer {
	return Signer{
		Prefix: prefix,
		Key:    key,
		Now:    time.Now,
		Nonce:  func() string { return fmt.Sprintf("%06d", rand.IntN(900000)+100000) },
	}
}

func (s Signer) Header(method, pathname string, query url.Values, body []byte, multipart bool) string {
	nonce := s.Nonce()
	timestamp := fmt.Sprintf("%d", s.Now().UnixMilli())
	payload := body
	if strings.EqualFold(method, "GET") {
		encoded := canonicalQuery(query)
		decoded, err := url.QueryUnescape(encoded)
		if err != nil {
			decoded = encoded
		}
		payload = []byte(decoded)
	} else if multipart {
		// The official browser signer evaluates JSON.stringify(FormData), which is "{}".
		payload = []byte("{}")
	}
	payloadHash := md5Hex(payload)
	source := strings.Join([]string{s.Prefix, pathname, nonce, timestamp, payloadHash, s.Key}, "_")
	return fmt.Sprintf("nonce=%s&timestamp=%s&sign=%s", nonce, timestamp, md5Hex([]byte(source)))
}

func canonicalQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		for _, value := range values[key] {
			parts = append(parts, queryEscape(key)+"="+queryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func queryEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func md5Hex(value []byte) string {
	digest := md5.Sum(value)
	return hex.EncodeToString(digest[:])
}
