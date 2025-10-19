package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Joon2000/opstack_blob_decoder/pkg/blob"
	"github.com/Joon2000/opstack_blob_decoder/pkg/frame"
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

	// 1) 여러 blob → ToData() → 하나의 연속 스트림으로 합치기
	var stream []byte
	for _, p := range flag.Args() {
		raw, err := os.ReadFile(p)
		if err != nil {
			fatalf("read %s: %v", p, err)
		}
		d, err := blob.Blob(raw).ToData()
		if err != nil {
			fatalf("decode blob %s: %v", p, err)
		}
		stream = append(stream, d...)
	}
	fmt.Printf("[✓] stream built: %d bytes from %d blob(s)\n", len(stream), flag.NArg())

	// (옵션) 스트림 저장
	if *saveStream != "" {
		if err := os.WriteFile(*saveStream, stream, 0o644); err != nil {
			fatalf("write stream: %v", err)
		}
		fmt.Printf("[✓] saved stream to %s\n", *saveStream)
	}

	// 스트림 헤드 몇 바이트 덤프 (버전 바이트 확인용)
	head := 32
	if len(stream) < head {
		head = len(stream)
	}
	if head > 0 {
		fmt.Printf("head[%d] hex: %s\n", head, hex.EncodeToString(stream[:head]))
		if len(stream) > 0 {
			fmt.Printf("derivation version byte: 0x%02x (expect 0x00)\n", stream[0])
		}
	}

	// 2) 프레임 파싱 (op-node의 ParseFrames 스타일)
	frames, err := frame.ParseFrames(stream)
	if err != nil {
		fatalf("ParseFrames: %v", err)
	}
	fmt.Printf("[✓] frames parsed: %d\n", len(frames))

	// 3) 앞쪽 N개 프레임 요약 출력
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

	// 4) 표준 출력으로 아무것도 내보내지 않음 (파이프라인에 맞춰 필요한 경우만 사용)
	_ = io.Discard
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[fatal] "+format+"\n", args...)
	os.Exit(1)
}
