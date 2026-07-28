package contentstore

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func TestLocalPutDeduplicatesAndVerifiesMD5(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	value := "immutable compliance content"
	sum := md5.Sum([]byte(value))
	opts := PutOptions{ExpectedMD5: base64.StdEncoding.EncodeToString(sum[:]), MIMEType: "text/plain"}
	first, err := store.Put(context.Background(), strings.NewReader(value), opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(context.Background(), strings.NewReader(value), opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != second.SHA256 || !first.VerifiedMD5 {
		t.Fatalf("unexpected objects: %#v %#v", first, second)
	}
	bytes, err := os.ReadFile(first.LocalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes) != value {
		t.Fatalf("content = %q", bytes)
	}
}

func TestLocalPutRejectsSizeAndChecksum(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), strings.NewReader("large"), PutOptions{MaxBytes: 2}); err == nil {
		t.Fatal("expected size error")
	}
	if _, err := store.Put(context.Background(), strings.NewReader("value"), PutOptions{ExpectedMD5: "00000000000000000000000000000000"}); err == nil {
		t.Fatal("expected checksum error")
	}
}
