package video_feed

import (
	"net/http"
        "encoding/binary"
        "os/exec"
	"os"
	"log"
	"time"
	"math"
	"fmt"
	"strconv"
	
	constants "github.com/lnix1/lift_judge/internal/constants"
)

func (writer *RingBufferWriter) HandlerVideoFeed(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        lastSentIndex := -1

        for {
                currentIndex := int(writer.Data[0])

		blockStart := constants.HeaderSize + (currentIndex * (constants.HeaderSize + constants.SlotSize))

		status := writer.Data[blockStart]

                if currentIndex != lastSentIndex && (status == constants.StatusReady || status == constants.StatusRaw) {
			frameLen := binary.LittleEndian.Uint32(writer.Data[blockStart+4 : blockStart+8])

			jpegStart := blockStart + constants.HeaderSize
			actualJPEG := writer.Data[jpegStart : jpegStart+int(frameLen)]

                        // Stream to browser
                        fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", frameLen)
                        w.Write(actualJPEG)
                        fmt.Fprintf(w, "\r\n")

                        lastSentIndex = currentIndex
                }
                time.Sleep(10 * time.Millisecond)
        }
}

func (writer *RingBufferWriter) HandlerStartRecording(w http.ResponseWriter, r *http.Request) {
	clear(writer.RecordedData)
	writer.RecordWriteIndex = 0
	writer.RecordFlag = true
}

func (writer *RingBufferWriter) HandlerStopRecording(w http.ResponseWriter, r *http.Request) {
	clear(writer.RecordedData)
	writer.RecordWriteIndex = 0
	writer.RecordFlag = true

	msPerTenFrames := (1000.0 / float64(constants.FramesPerSecond)) * 10.0
	roundedMs := math.Ceil(msPerTenFrames)
	time.Sleep(time.Duration(roundedMs) * time.Millisecond)


        log.Println("Starting C++ Recording Annotator...")
        cmdAnnotator := exec.Command("./internal/annotators/media_pipe/annotator", strconv.Itoa(constants.MaxRecordedFrames), "0")
        cmdAnnotator.Stdout = os.Stdout
        cmdAnnotator.Stderr = os.Stderr
        if err := cmdAnnotator.Run(); err != nil {
            	log.Printf("Annotator error: %v", err)
        }

	_ = writer.WriteRecordingToDisk()
}
