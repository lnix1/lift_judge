package server

import (
	"net/http"
        "encoding/binary"
        "os/exec"
	"os"
	"log"
	"time"
	"math"
	"fmt"
	
	constants "github.com/lnix1/lift_judge/internal/constants"
	resp "github.com/lnix1/lift_judge/internal/responses"
	"github.com/lnix1/lift_judge/internal/auth"
)

func (apiCfg *ApiCfg) handlerVideoFeed(w http.ResponseWriter, r *http.Request) {
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		bearer = r.URL.Query().Get("access_token")
    	}
	if bearer == "" {
		resp.RespondWithError(w, http.StatusUnauthorized, "Authentication token missing", err)
		return
	}
	
	_, err = auth.ValidateJWT(bearer, apiCfg.Secret)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Invalid authentication token", err)
		return
	}

        w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        lastSentIndex := -1

        for {
                currentIndex := int(apiCfg.WriterCfg.Data[0])
		if currentIndex == 0 {
			currentIndex = constants.NumSlots - 2
		} else if currentIndex == 1 {
			currentIndex = constants.NumSlots - 1
		} else {
			currentIndex = currentIndex - 2
		}

		blockStart := constants.HeaderSize + (currentIndex * (constants.HeaderSize + constants.SlotSize))

		status := apiCfg.WriterCfg.Data[blockStart]

                if currentIndex != lastSentIndex && (status == constants.StatusReady || status == constants.StatusRaw) {
			frameLen := binary.LittleEndian.Uint32(apiCfg.WriterCfg.Data[blockStart+4 : blockStart+8])

			jpegStart := blockStart + constants.HeaderSize
			actualJPEG := apiCfg.WriterCfg.Data[jpegStart : jpegStart+int(frameLen)]

                        // Stream to browser
                        fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", frameLen)
                        w.Write(actualJPEG)
                        fmt.Fprintf(w, "\r\n")

                        lastSentIndex = currentIndex
                }
                time.Sleep(10 * time.Millisecond)
        }
}

func (apiCfg *ApiCfg) handlerStartRecording(w http.ResponseWriter, r *http.Request) {
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Authentication token missing", err)
		return
	}
	
	userID, err := auth.ValidateJWT(bearer, apiCfg.Secret)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Invalid authentication token", err)
		return
	}

	if apiCfg.WriterCfg.RecordFlag == true && apiCfg.WriterCfg.RecordingUser != userID {
		resp.RespondWithError(w, http.StatusConflict, "Another User is currently recording", err)
		return
	}

	clear(apiCfg.WriterCfg.RecordedData)
	apiCfg.WriterCfg.RecordWriteIndex = 0
	apiCfg.WriterCfg.RecordFlag = true
	apiCfg.WriterCfg.RecordingUser = userID
	log.Println("Video recording started...")

	resp.RespondWithJSON(w, http.StatusNoContent, nil)
	return
}

func (apiCfg *ApiCfg) handlerStopRecording(w http.ResponseWriter, r *http.Request) {
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Authentication token missing", err)
		return
	}
	
	userID, err := auth.ValidateJWT(bearer, apiCfg.Secret)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Invalid authentication token", err)
		return
	}
	
	if apiCfg.WriterCfg.RecordFlag == true && apiCfg.WriterCfg.RecordingUser != userID {
		resp.RespondWithError(w, http.StatusConflict, "Another User is currently recording", err)
		return
	}
	
	if apiCfg.WriterCfg.RecordFlag == false{
		resp.RespondWithError(w, http.StatusConflict, "No recording in progress", err)
		return
	}

	msPerTenFrames := (1000.0 / float64(constants.FramesPerSecond)) * 10.0
	roundedMs := math.Ceil(msPerTenFrames)
	time.Sleep(time.Duration(roundedMs) * time.Millisecond)
	apiCfg.WriterCfg.RecordFlag = false

        log.Println("Starting C++ Recording Annotator...")
        cmdAnnotator := exec.Command("./internal/annotators/media_pipe/annotator", 
		fmt.Sprintf("--headersize=%d", constants.HeaderSize), 
		fmt.Sprintf("--slotsize=%d", constants.SlotSize), 
		fmt.Sprintf("--numslots=%d", apiCfg.WriterCfg.RecordWriteIndex+1), 
		fmt.Sprintf("--detectionconfidence=%f", constants.DetectionConfidence), 
		fmt.Sprintf("--isring=%t", false), 
		fmt.Sprintf("--shmpath=%s", constants.ShmPathRecordingCpp), 
	)
        cmdAnnotator.Stdout = os.Stdout
        cmdAnnotator.Stderr = os.Stderr
        if err := cmdAnnotator.Run(); err != nil {
            	log.Printf("Annotator error: %v", err)
        }

	_ = apiCfg.WriterCfg.WriteRecordingToDisk()

	resp.RespondWithJSON(w, http.StatusNoContent, nil)
	return
}
