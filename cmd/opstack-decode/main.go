package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Joon2000/opstack_blob_decoder/pkg/blob"
	"github.com/Joon2000/opstack_blob_decoder/pkg/frame"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func main() {
	var (
		saveStream = flag.String("save-stream", "", "복원한 연속 스트림을 이 경로로 저장 (옵션)")
		printN     = flag.Int("n", 5, "요약 출력할 프레임 개수")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <blob1> [blob2 ...]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	// 1) 여러 blob → ToData() → 조각 단위로 수집 ([]hexutil.Bytes)
	var datas []hexutil.Bytes
	totalBytes := 0
	for _, p := range flag.Args() {
		raw, err := os.ReadFile(p)
		if err != nil {
			fatalf("read %s: %v", p, err)
		}
		d, err := blob.Blob(raw).ToData() // d: []byte
		if err != nil {
			fatalf("decode blob %s: %v", p, err)
		}
		datas = append(datas, hexutil.Bytes(d))
		totalBytes += len(d)
	}
	fmt.Printf("[✓] collected %d piece(s), %d bytes total\n", len(datas), totalBytes)

	// 2) (옵션) 연속 스트림으로 합쳐서 저장
	//    저장 여부와 무관하게 헤더 덤프/버전 확인을 위해 stream은 한 번 만들어 둡니다.
	var stream hexutil.Bytes
	for _, piece := range datas {
		stream = append(stream, piece...)
	}

	if *saveStream != "" {
		if err := os.WriteFile(*saveStream, []byte(stream), 0o644); err != nil {
			fatalf("write stream: %v", err)
		}
		fmt.Printf("[✓] saved stream to %s\n", *saveStream)
	}

	// 3) 스트림 헤드 덤프 (버전 바이트 확인)
	head := 32
	if len(stream) < head {
		head = len(stream)
	}
	if head > 0 {
		fmt.Printf("head[%d] hex: %s\n", head, hex.EncodeToString([]byte(stream[:head])))
		fmt.Printf("derivation version byte: 0x%02x (expect 0x00)\n", stream[0])
	}

	// 4) 프레임 파싱: 각 조각(piece) 별로 ParseFrames 실행
	var frames []frame.Frame
	var firstErr error
	for idx, piece := range datas {
		fs, err := frame.ParseFrames(piece)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("piece %d: %w", idx, err)
			}
			// 계속 진행은 하되 오류는 기억
			fmt.Fprintf(os.Stderr, "[warn] ParseFrames failed for piece %d: %v\n", idx, err)
			continue
		}
		frames = append(frames, fs...)
	}
	if firstErr != nil && len(frames) == 0 {
		// 전부 실패했다면 fatal 처리
		fatalf("ParseFrames: %v", firstErr)
	}
	fmt.Printf("[✓] frames parsed: %d (with possible per-piece warnings)\n", len(frames))

	// 5) 앞쪽 N개 프레임 요약 출력
	n := *printN
	if n > len(frames) {
		n = len(frames)
	}
	for i := 0; i < n; i++ {
		f := frames[i]
		chID := hex.EncodeToString(f.ID[:])
		if len(chID) > 8 {
			chID = chID[:8]
		}
		fmt.Printf("  #%d: ch=%s.. n=%d last=%v data=%dB\n",
			i, chID, f.FrameNumber, f.IsLast, len(f.Data))
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[fatal] "+format+"\n", args...)
	os.Exit(1)
}
