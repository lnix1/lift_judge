package video_feed

import (
        "encoding/binary"

	constants "github.com/lnix1/lift_judge/internal/constants"
)

type RingBufferWriter struct {
        Data       []byte
        TempBuf    []byte
        WriteIndex int
}

func (w *RingBufferWriter) Write(p []byte) (n int, err error) {
        for _, b := range p {
                w.TempBuf = append(w.TempBuf, b)

                // Detect Start of Image (SOI)
                if len(w.TempBuf) >= 2 && w.TempBuf[len(w.TempBuf)-2] == 0xFF && w.TempBuf[len(w.TempBuf)-1] == 0xD8 {
                        if len(w.TempBuf) > 2 {
                                w.TempBuf = []byte{0xFF, 0xD8} // Reset to sync if we missed an EOI
                        }
                }

                // Detect End of Image (EOI)
                if len(w.TempBuf) >= 2 && w.TempBuf[len(w.TempBuf)-2] == 0xFF && w.TempBuf[len(w.TempBuf)-1] == 0xD9 {
                        if len(w.TempBuf) <= constants.SlotSize {
				blockStart := constants.HeaderSize + (w.WriteIndex * (constants.HeaderSize + constants.SlotSize))

				copy(w.Data[blockStart + constants.HeaderSize:], w.TempBuf)
				
				// Write the status and length values for the slot
				w.Data[blockStart] = constants.StatusRaw
				binary.LittleEndian.PutUint32(w.Data[blockStart+4:], uint32(len(w.TempBuf)))

				// Clear the 40 bytes of joint Data (starts at byte 8 of the block)
				for i := 0; i < 40; i++ {
					w.Data[blockStart+8+i] = 0
				}

                                // Update current write index (Header byte 0)
                                w.Data[0] = byte(w.WriteIndex)

                                w.WriteIndex = (w.WriteIndex + 1) % constants.NumSlots
                        }
                        w.TempBuf = w.TempBuf[:0]
                }
        }
        return len(p), nil
}
