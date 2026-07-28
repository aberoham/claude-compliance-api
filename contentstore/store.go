// Package contentstore stores immutable Compliance API downloads outside the
// relational cache. Metadata and resource links remain in SQLite.
package contentstore

import (
	"context"
	"io"
)

type PutOptions struct {
	ExpectedMD5 string
	MIMEType    string
	MaxBytes    int64
}

type Object struct {
	SHA256      string
	MD5Hex      string
	SizeBytes   int64
	MIMEType    string
	Backend     string
	ObjectKey   string
	LocalPath   string
	VerifiedMD5 bool
}

// Store is deliberately backend-neutral. A future GCS implementation can use
// ObjectKey for its blob name while SQLite continues to hold only metadata.
type Store interface {
	Put(context.Context, io.Reader, PutOptions) (*Object, error)
}
