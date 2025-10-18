package frame

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const ChannelIDLength = 16
const MaxFrameLen = 1_000_000

type ChannelID [ChannelIDLength]byte

type Frame struct {
	ID          ChannelID
	FrameNumber uint64
	IsLast      bool
	Data        []byte
}

type ByteReader interface {
	io.Reader
	io.ByteReader
}

const DerivationVersion0 = 0

func ParseFrames(data []byte) ([]Frame, error) {
	if len(data) == 0 {
		return nil, errors.New("data array must not be empty")
	}
	if data[0] != DerivationVersion0 {
		return nil, fmt.Errorf("invalid derivation format byte: got %d", data[0])
	}

	r := bytes.NewBuffer(data[1:])

	var frames []Frame

	for r.Len() > 0 {
		var f Frame
		if err := f.UnmarshalBinary(r); err != nil {
			return nil, fmt.Errorf("parsing frame %d: %w", len(frames), err)
		}
		frames = append(frames, f)
	}

	if len(frames) == 0 {
		return nil, errors.New("was not able to find any frames")
	}

	return frames, nil
}

func (f *Frame) UnmarshalBinary(r ByteReader) error {
	if _, err := io.ReadFull(r, f.ID[:]); err != nil {
		return fmt.Errorf("reading channel_id: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &f.FrameNumber); err != nil {
		return fmt.Errorf("reading frame_number: %w", eofAsUnexpectedMissing(err))
	}

	var frameLength uint32
	if err := binary.Read(r, binary.BigEndian, &frameLength); err != nil {
		return fmt.Errorf("reading frame_data_length: %w", eofAsUnexpectedMissing(err))
	}
	if frameLength > MaxFrameLen {
		return fmt.Errorf("frame_data_length is too large: %d", frameLength)
	}

	f.Data = make([]byte, frameLength)
	if _, err := io.ReadFull(r, f.Data); err != nil {
		return fmt.Errorf("reading frame_data: %w", eofAsUnexpectedMissing(err))
	}

	isLastByte, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("reading final byte (is_last): %w", eofAsUnexpectedMissing(err))
	}

	switch isLastByte {
	case 0:
		f.IsLast = false
	case 1:
		f.IsLast = true
	default:
		return errors.New("invalid byte as is_last")
	}

	return nil
}

// MarshalBinary writes the frame to `w`.
// It returns any errors encountered while writing, but
// generally expects the writer very rarely fail.
func (f *Frame) MarshalBinary(w io.Writer) error {
	_, err := w.Write(f.ID[:])
	if err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, f.FrameNumber); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(f.Data))); err != nil {
		return err
	}
	_, err = w.Write(f.Data)
	if err != nil {
		return err
	}
	if f.IsLast {
		if _, err = w.Write([]byte{1}); err != nil {
			return err
		}
	} else {
		if _, err = w.Write([]byte{0}); err != nil {
			return err
		}
	}
	return nil
}

func eofAsUnexpectedMissing(err error) error {
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("fully missing: %w", io.ErrUnexpectedEOF)
	}
	return err
}

func (f *Frame) String() string {
	return fmt.Sprintf("Frame{ch=%x..., n=%d, last=%v, data=%dB}", f.ID[:4], f.FrameNumber, f.IsLast, len(f.Data))
}
