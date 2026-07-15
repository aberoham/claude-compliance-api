package contentstore

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Local struct {
	root string
}

func NewLocal(root string) (*Local, error) {
	if root == "" {
		return nil, fmt.Errorf("content store root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving content store root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("creating content store: %w", err)
	}
	return &Local{root: abs}, nil
}

func (s *Local) Put(ctx context.Context, source io.Reader, opts PutOptions) (_ *Object, resultErr error) {
	tmp, err := os.CreateTemp(s.root, ".download-*")
	if err != nil {
		return nil, fmt.Errorf("creating temporary content object: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if resultErr != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return nil, err
	}

	sha := sha256.New()
	md := md5.New()
	reader := &contextReader{ctx: ctx, reader: source}
	var written int64
	if opts.MaxBytes > 0 {
		limited := &io.LimitedReader{R: reader, N: opts.MaxBytes + 1}
		written, err = io.Copy(io.MultiWriter(tmp, sha, md), limited)
		if err == nil && written > opts.MaxBytes {
			return nil, fmt.Errorf("download exceeds maximum size of %d bytes", opts.MaxBytes)
		}
	} else {
		written, err = io.Copy(io.MultiWriter(tmp, sha, md), reader)
	}
	if err != nil {
		return nil, fmt.Errorf("streaming content object: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	shaHex := hex.EncodeToString(sha.Sum(nil))
	mdBytes := md.Sum(nil)
	mdHex := hex.EncodeToString(mdBytes)
	verified := false
	if opts.ExpectedMD5 != "" {
		expected, err := decodeMD5(opts.ExpectedMD5)
		if err != nil {
			return nil, err
		}
		if string(expected) != string(mdBytes) {
			return nil, fmt.Errorf("content MD5 mismatch: expected %x, got %s", expected, mdHex)
		}
		verified = true
	}

	objectKey := filepath.Join(shaHex[:2], shaHex[2:4], shaHex)
	finalPath := filepath.Join(s.root, objectKey)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Remove(tmpName); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	} else if err := os.Rename(tmpName, finalPath); err != nil {
		return nil, fmt.Errorf("committing content object: %w", err)
	}

	return &Object{
		SHA256: shaHex, MD5Hex: mdHex, SizeBytes: written, MIMEType: opts.MIMEType,
		Backend: "local", ObjectKey: filepath.ToSlash(objectKey), LocalPath: finalPath,
		VerifiedMD5: verified,
	}, nil
}

func decodeMD5(value string) ([]byte, error) {
	value = strings.Trim(strings.TrimSpace(value), `"`)
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == md5.Size {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == md5.Size {
		return decoded, nil
	}
	return nil, fmt.Errorf("invalid MD5 checksum %q", value)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}
