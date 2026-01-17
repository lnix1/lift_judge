package video_feed

import (
	"net/http"
        "os/exec"
	"os"
	"log"
	"time"
	"math"
	
	constants "github.com/lnix1/lift_judge/internal/constants"
)

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
        cmdAnnotator := exec.Command("./internal/annotators/media_pipe/annotator_recording")
        cmdAnnotator.Stdout = os.Stdout
        cmdAnnotator.Stderr = os.Stderr
        if err := cmdAnnotator.Run(); err != nil {
            	log.Printf("Annotator error: %v", err)
        }

	_ = writer.WriteRecordingToDisk()
}
