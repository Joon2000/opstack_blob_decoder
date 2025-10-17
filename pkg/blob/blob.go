package blob

import "fmt"

type Blob []byte
type Data []byte

const (
	BlobSize        = 4096 * 32
	MaxBlobDataSize = (4*31+3)*1024 - 4
	EncodingVersion = 0
	VersionOffset   = 1    // offset of the version byte in the blob encoding
	Rounds          = 1024 // number of encode/decode rounds
)

func (b Blob) ToData() (Data, error) {
	if len(b) != BlobSize {
		return nil, fmt.Errorf("Invalid blob size: got %d, want %d", len(b), BlobSize)
	}

	if b[VersionOffset] != EncodingVersion {
		return nil, fmt.Errorf("invalid encoding version: expected %d, got %d", EncodingVersion, b[VersionOffset])
	}

	outputLen := uint32(b[2])<<16 | uint32(b[3])<<8 | uint32(b[4])
	if outputLen > MaxBlobDataSize {
		return nil, fmt.Errorf("invalid outpu length: %d (max %d)", outputLen, MaxBlobDataSize)
	}

	output := make(Data, MaxBlobDataSize)

	copy(output[0:27], b[5:])

	opos := 28
	ipos := 32
	_ = ipos
	_ = opos

	return output[:outputLen], nil
}
