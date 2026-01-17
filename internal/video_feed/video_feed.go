package video_feed

import (
        "encoding/binary"
        "os/exec"
	"fmt"

	constants "github.com/lnix1/lift_judge/internal/constants"
)

type RingBufferWriter struct {
        Data       	 []byte
        TempBuf    	 []byte
        WriteIndex 	 int
	RecordFlag 	 bool
	RecordWriteIndex int
	RecordedData	 []byte
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
				
				// Sort of hacky way to record frames, but will miss last 10 frames when a users stops the recording.
				// Not a problem if we have a small wait when the record is ended or if record is ended with some space after 
				// the lift
				if w.RecordFlag == true && w.RecordWriteIndex < constants.MaxRecordedFrames {
					imgLengthBytes := w.Data[blockStart+4 : blockStart+8]
					imgLength := binary.LittleEndian.Uint32(imgLengthBytes)
					imgBlockEnd := blockStart + constants.HeaderSize + int(imgLength)

					recordedBlockStart := w.RecordWriteIndex * (constants.HeaderSize + constants.SlotSize)

					copy(w.RecordedData[recordedBlockStart : ], w.Data[blockStart : imgBlockEnd])

					w.RecordWriteIndex = w.RecordWriteIndex + 1
				}

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

func (writer *RingBufferWriter) WriteRecordingToDisk() error {
	cmd := exec.Command("ffmpeg",
		"-y",                 
		"-f", "mjpeg",        
		"-r", fmt.Sprintf("%d", constants.FramesPerSecond),
		"-i", "pipe:0",       
		"-c:v", "copy",       
		"videos/tmp.mp4",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	for i := 0; i <= writer.RecordWriteIndex; i++ {
		blockStart := writer.RecordWriteIndex * (constants.HeaderSize + constants.SlotSize)
		imgLengthBytes := writer.RecordedData[blockStart+4 : blockStart+8]
		imgLength := binary.LittleEndian.Uint32(imgLengthBytes)

		start := blockStart + constants.HeaderSize
		end := start + int(imgLength)
		
		_, err := stdin.Write(writer.RecordedData[start:end])
		if err != nil {
			return fmt.Errorf("failed to write frame %d: %v", i, err)
		}
	}

	stdin.Close()
	return cmd.Wait()
}
