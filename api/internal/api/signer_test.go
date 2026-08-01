package api

import (
	"net/url"
	"testing"
	"time"
)

func TestCanonicalQueryAndSignature(t *testing.T) {
	values := url.Values{
		"q":    {"hello world"},
		"page": {"1"},
	}
	if got, want := canonicalQuery(values), "page=1&q=hello%20world"; got != want {
		t.Fatalf("canonicalQuery() = %q, want %q", got, want)
	}

	signer := Signer{
		Prefix: "prefix",
		Key:    "key",
		Now:    func() time.Time { return time.UnixMilli(1700000000000) },
		Nonce:  func() string { return "123456" },
	}
	got := signer.Header("GET", "/music/api/v1/search/track", values, nil, false)
	want := "nonce=123456&timestamp=1700000000000&sign=a32d34bb733dfb367391b7fd7e0ce2d7"
	if got != want {
		t.Fatalf("Header() = %q, want %q", got, want)
	}
}

func TestMultipartSignatureUsesBrowserFormDataPayload(t *testing.T) {
	signer := Signer{
		Prefix: "prefix",
		Key:    "key",
		Now:    func() time.Time { return time.UnixMilli(1700000000000) },
		Nonce:  func() string { return "123456" },
	}
	got := signer.Header("POST", "/music/api/v1/static/cover/track", nil, []byte("binary"), true)
	want := "nonce=123456&timestamp=1700000000000&sign=74ff9fc73bebf5276024c6bb60fbfae0"
	if got != want {
		t.Fatalf("Header() = %q, want %q", got, want)
	}
}
