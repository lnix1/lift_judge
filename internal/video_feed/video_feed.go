package video_feed

import (
        "encoding/binary"
        "os/exec"
	"fmt"
	"log"

	constants "github.com/lnix1/lift_judge/internal/constants"
	
	"github.com/google/uuid"
)

type RingBufferWriter struct {
        Data       	 []byte
        TempBuf    	 []byte
        WriteIndex 	 int
	RecordFlag 	 bool
	RecordWriteIndex int
	RecordedData	 []byte
	AnnotatorTrigger *Semaphore
	RecordingUser 	 uuid.UUID
}

func (writerCfg *RingBufferWriter) Write(p []byte) (n int, err error) {
        for _, b := range p {
                writerCfg.TempBuf = append(writerCfg.TempBuf, b)

                // Detect Start of Image (SOI)
                if len(writerCfg.TempBuf) >= 2 && writerCfg.TempBuf[len(writerCfg.TempBuf)-2] == 0xFF && writerCfg.TempBuf[len(writerCfg.TempBuf)-1] == 0xD8 {
                        if len(writerCfg.TempBuf) > 2 {
                                writerCfg.TempBuf = []byte{0xFF, 0xD8} // Reset to sync if we missed an EOI
                        }
                }

                // Detect End of Image (EOI)
                if len(writerCfg.TempBuf) >= 2 && writerCfg.TempBuf[len(writerCfg.TempBuf)-2] == 0xFF && writerCfg.TempBuf[len(writerCfg.TempBuf)-1] == 0xD9 {
                        if len(writerCfg.TempBuf) <= constants.SlotSize {
				blockStart := constants.HeaderSize + (writerCfg.WriteIndex * (constants.HeaderSize + constants.SlotSize))
				
				if writerCfg.RecordFlag == true && writerCfg.RecordWriteIndex < constants.MaxRecordedFrames {
					imgLengthBytes := writerCfg.Data[blockStart+4 : blockStart+8]
					imgLength := binary.LittleEndian.Uint32(imgLengthBytes)
					imgBlockEnd := blockStart + constants.HeaderSize + int(imgLength)

					recordedBlockStart := writerCfg.RecordWriteIndex * (constants.HeaderSize + constants.SlotSize)

					copy(writerCfg.RecordedData[recordedBlockStart : ], writerCfg.Data[blockStart : imgBlockEnd])

					writerCfg.RecordWriteIndex = writerCfg.RecordWriteIndex + 1
				}

				copy(writerCfg.Data[blockStart + constants.HeaderSize:], writerCfg.TempBuf)
				
				// Write the status and length values for the slot
				writerCfg.Data[blockStart] = constants.StatusRaw
				binary.LittleEndian.PutUint32(writerCfg.Data[blockStart+4:], uint32(len(writerCfg.TempBuf)))

				// Clear the 40 bytes of joint Data (starts at byte 8 of the block)
				for i := 0; i < 40; i++ {
					writerCfg.Data[blockStart+8+i] = 0
				}

                                // Update current write index (Header byte 0)
                                writerCfg.Data[0] = byte(writerCfg.WriteIndex)

                                writerCfg.WriteIndex = (writerCfg.WriteIndex + 1) % constants.NumSlots
				
				// Let annotator know another frame is ready
				err := writerCfg.AnnotatorTrigger.Post()
				if err != nil {
					return len(p), fmt.Errorf("sem_post error: %v", err)
				}
                        }
                        writerCfg.TempBuf = writerCfg.TempBuf[:0]
                }
        }
        return len(p), nil
}

func (writerCfg *RingBufferWriter) WriteRecordingToDisk() error {
        log.Println("Starting to write recording to disk...")
	cmd := exec.Command("ffmpeg",
		"-y",                 
		"-f", "mjpeg",        
		"-r", fmt.Sprintf("%d", constants.FramesPerSecond),
		"-i", "pipe:0",       
		//"-c:v", "copy",       
		"-c:v", "libx264",        // Use the H.264 encoder
    		"-pix_fmt", "yuv420p",    // Crucial for Windows/Mobile compatibility
    		"-preset", "ultrafast",   // Fast encoding for the Raspberry Pi
    		"-crf", "23",             // Quality level (18-28 is standard; lower is better)
		"videos/tmp.mp4",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	for i := 0; i < writerCfg.RecordWriteIndex; i++ {
		blockStart := i * (constants.HeaderSize + constants.SlotSize)
		imgLengthBytes := writerCfg.RecordedData[blockStart+4 : blockStart+8]
		imgLength := binary.LittleEndian.Uint32(imgLengthBytes)

		start := blockStart + constants.HeaderSize
		end := start + int(imgLength)
		
		_, err := stdin.Write(writerCfg.RecordedData[start:end])
		if err != nil {
			return fmt.Errorf("failed to write frame %d: %v", i, err)
		}
	}

	stdin.Close()
	return cmd.Wait()
}
