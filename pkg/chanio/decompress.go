package chanio

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
)

func Decompress(b []byte) ([]byte, string, error) {
	if len(b) == 0 {
		return nil, "", io.EOF
	}

	if (b[0]&0x0F) == 8 || (b[0]&0x0F) == 15 {
		if zr, err := zlib.NewReader(bytes.NewReader(b)); err == nil {
			defer zr.Close()
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, zr); err != nil {
				return nil, "zlib", err
			}
			return buf.Bytes(), "zlib", nil
		}
	} else if b[0] == 0x01 {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, brotli.NewReader(bytes.NewReader(b[1:]))); err == nil {
			return buf.Bytes(), "brotli(v1)", nil
		}
	} else {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, brotli.NewReader(bytes.NewReader(b))); err == nil {
			return buf.Bytes(), "brotli", nil
		}
	}

	return nil, "unknown", fmt.Errorf("unknown compression (first=0x%02x)", b[0])
}
