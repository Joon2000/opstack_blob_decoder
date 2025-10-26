package derive

import (
	"encoding/hex"
	"sort"

	"github.com/Joon2000/opstack_blob_decoder/pkg/frame"
)

type ChanStat struct {
	FramesTotal   int
	FramesUsed    int
	FirstNumber   uint64
	LastNumber    uint64
	Complete      bool
	HasGaps       bool
	HasDuplicate  bool
	BytesConcated int
	Err           string
}

type Channel struct {
	ID      []byte
	Payload []byte
	Stat    ChanStat
}

func ReassembleChannels(frames []frame.Frame, maxBytesPerChannel uint64) (order [][]byte, channels map[string]*Channel) {
	order = make([][]byte, 0, 8)
	channels = make(map[string]*Channel)

	key := func(id []byte) string { return hex.EncodeToString(id) }

	tmp := make(map[string][]frame.Frame)
	seen := make(map[string]bool)

	for _, f := range frames {
		k := key(f.ID[:])
		if !seen[k] {
			seen[k] = true
			cp := make([]byte, len(f.ID))
			copy(cp, f.ID[:])
			order = append(order, cp)
			channels[k] = &Channel{ID: cp, Stat: ChanStat{}}
		}
		tmp[k] = append(tmp[k], f)

		st := channels[k].Stat
		st.FramesTotal++
		if st.FramesTotal == 1 {
			st.FirstNumber = uint64(f.FrameNumber)
			st.LastNumber = uint64(f.FrameNumber)
		} else {
			if uint64(f.FrameNumber) < st.FirstNumber {
				st.FirstNumber = uint64(f.FrameNumber)
			}
			if uint64(f.FrameNumber) > st.LastNumber {
				st.LastNumber = uint64(f.FrameNumber)
			}
			if uint64(f.FrameNumber) > st.LastNumber {
				st.LastNumber = uint64(f.FrameNumber)
			}
		}
		if f.IsLast {
			st.Complete = true
		}
		channels[k].Stat = st
	}

	for _, id := range order {
		k := key(id)
		parts := tmp[k]
		if len(parts) == 0 {
			continue
		}
		sort.Slice(parts, func(i, j int) bool { return parts[i].FrameNumber < parts[j].FrameNumber })

		var (
			out       []byte
			prev      *uint64
			hasGap    bool
			hasDup    bool
			used      int
			complete  bool
			sizeError error
		)

		for _, p := range parts {
			if prev != nil {
				if uint64(p.FrameNumber) == *prev {
					hasDup = true
					continue
				}
				if uint64(p.FrameNumber) != *prev+1 {
					hasGap = true
				}
			}

			out = append(out, p.Data...)
			used++
			if p.IsLast {
				complete = true
			}
			v := uint64(p.FrameNumber)
			prev = &v
		}
		st := channels[k].Stat
		st.FramesUsed = used
		st.HasGaps = st.HasGaps || hasGap
		st.HasDuplicate = st.HasDuplicate || hasDup
		st.BytesConcated = len(out)
		if sizeError != nil {
			st.Err = sizeError.Error()
		}

		if complete && sizeError == nil {
			channels[k].Payload = out
			st.Complete = true
		} else {
			st.Complete = false
			channels[k].Payload = nil
		}
		channels[k].Stat = st
	}

	return order, channels
}
