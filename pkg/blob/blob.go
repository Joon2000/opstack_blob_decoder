package blob

import "fmt"

type Blob [BlobSize]byte
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

	// round 0 is special cased to copy only the remaining 27 bytes of the first field element into
	// the output due to version/length encoding already occupying its first 5 bytes.
	outputLen := uint32(b[2])<<16 | uint32(b[3])<<8 | uint32(b[4])
	if outputLen > MaxBlobDataSize {
		return nil, fmt.Errorf("invalid outpu length: %d (max %d)", outputLen, MaxBlobDataSize)
	}

	output := make(Data, MaxBlobDataSize)
	copy(output[0:27], b[5:])

	// now process remaining 3 field elements to complete round 0
	base := 28
	ipos := 32

	encodedByte := [4]byte{}
	encodedByte[0] = b[0]

	o := base
	var err error
	for i := 1; i < 4; i++ {
		encodedByte[i], o, ipos, err = b.decodeFieldElement(o, ipos, encodedByte[:i])
		if err != nil {
			return nil, err
		}
	}

	reassembleBytes(base, encodedByte[:], output)

	// in each remaining round we decode 4 field elements (128 bytes) of the input into 127 bytes
	// of output
	for i := 1; i < Rounds && o < int(outputLen); i++ {
		base = 0
		for j := 0; j < 4; j++ {
			encodedByte[j], o, ipos, err = b.decodeFieldElement(o, ipos, output)
			if err != nil {
				return nil, err
			}
		}
		reassembleBytes(base, encodedByte[:], output)

		o = base + 127
	}

	for i := int(outputLen); i < len(output); i++ {
		if output[i] != 0 {
			return nil, fmt.Errorf("fe=%d: extraneous data in output past declared length", i/32)
		}
	}

	for ; ipos < BlobSize; ipos++ {
		if b[ipos] != 0 {
			return nil, fmt.Errorf("pos=%d: extraneous data in blob tail", ipos)
		}
	}

	return output[:outputLen], nil
}

// decodeFieldElement는 다음 필드요소(FE)의 tail(31바이트)을
// 출력에 복사(32 stride로 1바이트 갭 유지)하고,
// FE의 첫 바이트(상위2비트=0이어야 함)를 반환한다.
func (b *Blob) decodeFieldElement(opos, ipos int, output []byte) (byte, int, int, error) {
	// two highest order bits of the first byte of each field element should always be 0
	if b[ipos]&0b1100_0000 != 0 {
		return 0, 0, 0, fmt.Errorf("invalid field element: high bits set at fe-offset=%d", ipos)
	}
	copy(output[opos:], b[ipos+1:ipos+32])
	return b[ipos], opos + 32, ipos + 32, nil
}

// reassembleBytes는 4개의 첫바이트 하위 6비트(=24비트)를 3바이트로 합쳐
// 현재 라운드의 3개 갭(base+31, base+63, base+95)에 채워 넣는다.
func reassembleBytes(base int, eb []byte, out []byte) {
	// eb[0..3]의 하위 6비트만 취해 24비트로 결합
	v := (uint32(eb[0]&0x3F) << 18) |
		(uint32(eb[1]&0x3F) << 12) |
		(uint32(eb[2]&0x3F) << 6) |
		uint32(eb[3]&0x3F)

	out[base+31] = byte(v >> 16) // 상위 8비트
	out[base+63] = byte(v >> 8)  // 중간 8비트
	out[base+95] = byte(v)       // 하위 8비트
}
