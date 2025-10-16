package main

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/ethereum/go-ethereum/rlp"
)

// ---- Frame/Channel 스키마 (Bedrock 파이프라인 단순화 버전) ----
// 네트워크 리비전별 세부 스키마 차이가 있을 수 있어요.
// 필요한 경우 길이나 필드를 약간 조정하세요.

type Frame struct {
	// 채널 식별자 길이는 네트워크 구현에 따라 16~32바이트 등 차이가 날 수 있음.
	// Base/OP Stack 최신 기준 16 또는 32를 흔히 봅니다.
	ChannelID   []byte // fixed-length 권장 (필요시 validate)
	FrameNumber uint64
	IsLast      bool
	Data        []byte
}

// RLP 디코딩 훅 (필요하면 커스텀 sedes로 고도화)
func (f *Frame) DecodeRLP(s *rlp.Stream) error {
	if _, err := s.List(); err != nil {
		return err
	}
	var raw [][]byte
	for {
		b, err := s.Raw()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		raw = append(raw, b)
	}
	if len(raw) < 4 {
		return fmt.Errorf("bad frame length: %d", len(raw))
	}

	if err := rlp.DecodeBytes(raw[0], &f.ChannelID); err != nil {
		return err
	}
	if err := rlp.DecodeBytes(raw[1], &f.FrameNumber); err != nil {
		return err
	}
	if err := rlp.DecodeBytes(raw[2], &f.IsLast); err != nil {
		return err
	}
	if err := rlp.DecodeBytes(raw[3], &f.Data); err != nil {
		return err
	}

	// 필요 시 ChannelID 길이 validate (예: 16 또는 32)
	// if len(f.ChannelID) != 16 { return fmt.Errorf("unexpected channel id len: %d", len(f.ChannelID)) }
	return nil
}

func mustReadAll(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

// decoded.bin 안에는 Frame들이 연속으로 붙어있다고 가정.
// RLP는 self-delimiting이라 앞에서부터 한 개씩 빼먹는 방식으로 파싱 가능.
func readFrames(stream []byte) ([]*Frame, error) {
	var out []*Frame
	rest := stream
	for len(rest) > 0 {
		f := new(Frame)
		s := rlp.NewStream(bytes.NewReader(rest), 0)
		if err := f.DecodeRLP(s); err != nil {
			return nil, fmt.Errorf("frame decode error near offset=%d: %w", len(stream)-len(rest), err)
		}
		// s can tell how many bytes consumed by tracking decoder; 간단히 다음 위치 추정:
		consumed := len(rest) - s.Remainder()
		if consumed <= 0 || consumed > len(rest) {
			return nil, fmt.Errorf("bad rlp consumption: %d", consumed)
		}
		out = append(out, f)
		rest = rest[consumed:]
	}
	return out, nil
}

type part struct {
	n int
	d []byte
}

func reassembleChannels(frames []*Frame) (order [][]byte, payloads map[string][]byte) {
	order = make([][]byte, 0, 8)
	tmp := map[string][]part{}
	done := map[string]bool{}

	key := func(id []byte) string { return hex.EncodeToString(id) }

	for _, f := range frames {
		k := key(f.ChannelID)
		if _, ok := tmp[k]; !ok {
			tmp[k] = []part{}
			order = append(order, append([]byte(nil), f.ChannelID...))
		}
		tmp[k] = append(tmp[k], part{n: int(f.FrameNumber), d: f.Data})
		if f.IsLast {
			done[k] = true
		}
	}
	payloads = make(map[string][]byte, len(tmp))
	for _, id := range order {
		k := key(id)
		parts := tmp[k]
		sort.Slice(parts, func(i, j int) bool { return parts[i].n < parts[j].n })
		var buf bytes.Buffer
		for _, p := range parts {
			buf.Write(p.d)
		}
		// 완료된 채널만 payloads에 넣자(원하면 미완료도 넣어서 실험 가능)
		if done[k] {
			payloads[k] = buf.Bytes()
		}
	}
	return order, payloads
}

func tryZlib(b []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: opstack-decoder decoded.bin")
		os.Exit(1)
	}
	stream := mustReadAll(os.Args[1])

	fmt.Println("[-] parsing frames...")
	frames, err := readFrames(stream)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[✓] frames parsed: %d\n", len(frames))

	fmt.Println("[-] reassembling channels...")
	order, chans := reassembleChannels(frames)
	fmt.Printf("[✓] complete channels: %d\n", len(chans))

	// 각 채널 payload를 zlib 해제 시도
	for _, id := range order {
		k := hex.EncodeToString(id)
		payload, ok := chans[k]
		if !ok {
			continue
		} // 미완료 채널 skip
		raw, err := tryZlib(payload)
		if err != nil {
			fmt.Printf("[!] channel %s: zlib decompress failed: %v\n", k[:8], err)
			continue
		}
		// 여기서 raw가 "배치 스트림" (L2 블록 경계 + 트랜잭션들)
		// 간단 출력: 앞 부분 헥사, 길이
		sample := raw
		if len(sample) > 256 {
			sample = sample[:256]
		}
		fmt.Printf("\n[✓] channel %s decompressed: %d bytes\n", k[:8], len(raw))
		fmt.Printf("    head(hex): %s\n", hex.EncodeToString(sample))

		// 다음 단계(선택):
		// - 배치 포맷 파서 추가 (RLP/바이너리) → L2 블록/트랜잭션 리스트 복원
		// - 여기선 간단히 zlib까지 확인하는 걸로 마무리
	}
}
