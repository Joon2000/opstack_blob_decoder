package chanio

import (
	"bytes"
	"io"
	"math/big"

	"encoding/hex"

	"github.com/ethereum/go-ethereum/rlp"
)

func PeekFirstRLP(raw []byte) (int, error) {
	s := rlp.NewStream(bytes.NewReader(raw), 0)
	if _, _, err := s.Kind(); err != nil {
		return 0, err
	}
	b, err := s.Raw()
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func DumpRLPJSON(raw []byte) (any, error) {
	s := rlp.NewStream(bytes.NewReader(raw), 0)
	var items []any
	for {
		if _, _, err := s.Kind(); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		v, err := readRLPValue(s)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, nil
}

func readRLPValue(s *rlp.Stream) (any, error) {
	kind, _, err := s.Kind()
	if err != nil {
		return nil, err
	}
	if kind == rlp.List {
		if _, err := s.List(); err != nil {
			return nil, err
		}
		var arr []any
		for {
			if _, _, err := s.Kind(); err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			v, err := readRLPValue(s)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	}

	b, err := s.Bytes()
	if err != nil {
		return nil, err
	}
	if len(b) <= 8 {
		n := new(big.Int).SetBytes(b)
		if n.BitLen() <= 63 {
			return n.Uint64(), nil
		}
	}
	return "0x" + hex.EncodeToString(b), nil
}
