package chanio

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
)

type RLPItem struct {
	Raw   []byte
	Index int
}

func IterateRLP(raw []byte, maxItems int) ([]RLPItem, error) {
	s := rlp.NewStream(bytes.NewReader(raw), 0)
	var out []RLPItem
	idx := 0
	for {
		if maxItems > 0 && idx >= maxItems {
			break
		}
		if _, _, err := s.Kind(); err == io.EOF {
			break
		} else if err != nil {
			return out, err
		}
		b, err := s.Raw()
		if err != nil {
			return out, err
		}
		out = append(out, RLPItem{Raw: b, Index: idx})
		idx++
	}
	return out, nil
}

func PrettyDump(item []byte) (any, error) {
	s := rlp.NewStream(bytes.NewReader(item), 0)
	return readRLPValue(s)
}

func PrettyDumpJSON(item []byte) ([]byte, error) {
	v, err := PrettyDump(item)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}
