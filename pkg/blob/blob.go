package blob

import (
	"fmt"
)

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
	// check the version
	if b[VersionOffset] != EncodingVersion {
		return nil, fmt.Errorf(
			"invalid encoding version: expected %d, got %d", EncodingVersion, b[VersionOffset])
	}

	// check the size
	if len(b) != BlobSize {
		return nil, fmt.Errorf("invalid length: got %d, want %d", len(b), BlobSize)
	}

	// round 0 is special cased to copy only the remaining 27 bytes of the first field element into
	// the output due to version/length encoding already occupying its first 5 bytes.
	outputLen := uint32(b[2])<<16 | uint32(b[3])<<8 | uint32(b[4])
	if outputLen > MaxBlobDataSize {
		return nil, fmt.Errorf("invalid output length: got %d, want %d", outputLen, MaxBlobDataSize)
	}

	output := make(Data, MaxBlobDataSize)
	copy(output[0:27], b[5:])

	opos := 28 // current position into output buffer
	ipos := 32 // current position into the input blob

	encodedByte := make([]byte, 4)
	encodedByte[0] = b[0]

	var err error
	for i := 1; i < 4; i++ {
		encodedByte[i], opos, ipos, err = b.decodeFieldElement(opos, ipos, output)
		if err != nil {
			return nil, err
		}
	}

	opos = reassembleBytes(opos, encodedByte[:], output)

	// in each remaining round we decode 4 field elements (128 bytes) of the input into 127 bytes
	// of output
	for i := 1; i < Rounds && opos < int(outputLen); i++ {
		for j := 0; j < 4; j++ {
			encodedByte[j], opos, ipos, err = b.decodeFieldElement(opos, ipos, output)
			if err != nil {
				return nil, err
			}
		}
		opos = reassembleBytes(opos, encodedByte[:], output)
	}

	for i := int(outputLen); i < len(output); i++ {
		if output[i] != 0 {
			return nil, fmt.Errorf("fe=%d: extraneous data in output past declared length", i/32)
		}
	}

	output = output[:outputLen]

	for ; ipos < BlobSize; ipos++ {
		if b[ipos] != 0 {
			return nil, fmt.Errorf("pos=%d: extraneous data in blob tail", ipos)
		}
	}

	return output, nil
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

// reassembleBytes takes the 4x6-bit chunks from encodedByte, reassembles them into 3 bytes of
// output, and places them in their appropriate output positions.
func reassembleBytes(opos int, encodedByte []byte, output []byte) int {
	opos-- // account for fact that we don't output a 128th byte
	x := (encodedByte[0] & 0b0011_1111) | ((encodedByte[1] & 0b0011_0000) << 2)
	y := (encodedByte[1] & 0b0000_1111) | ((encodedByte[3] & 0b0000_1111) << 4)
	z := (encodedByte[2] & 0b0011_1111) | ((encodedByte[3] & 0b0011_0000) << 2)
	// put the re-assembled bytes in their appropriate output locations
	output[opos-32] = z
	output[opos-(32*2)] = y
	output[opos-(32*3)] = x
	return opos
}
