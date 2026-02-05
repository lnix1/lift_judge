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
	"path/filepath"
	"encoding/json"
	"io"
	
	constants "github.com/lnix1/lift_judge/internal/constants"
	resp "github.com/lnix1/lift_judge/internal/responses"
	"github.com/lnix1/lift_judge/internal/auth"
	"github.com/lnix1/lift_judge/internal/database"
	
	"github.com/google/uuid"
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
        //cmdAnnotator.Stderr = os.Stderr
        if err := cmdAnnotator.Run(); err != nil {
            	log.Printf("Annotator error: %v", err)
        }
        
	type parameters struct {
		LiftType	string	`json:"lifttype"`
        }
        
	decoder := json.NewDecoder(r.Body)
        params := parameters{}
        err = decoder.Decode(&params)
        if err != nil {
                resp.RespondWithError(w, http.StatusInternalServerError, "Could not decode parameters", err)
                return
        }

	if params.LiftType == "deadlift" {
		apiCfg.WriterCfg.RecordedLiftResult = apiCfg.judgeDeadlift()
	}
	if params.LiftType == "squat" {
		apiCfg.WriterCfg.RecordedLiftResult = apiCfg.judgeSquat()
	}

	apiCfg.WriterCfg.RecordedLiftType = params.LiftType

	_ = apiCfg.WriterCfg.WriteRecordingToDisk()

	resp.RespondWithJSON(w, http.StatusNoContent, nil)
	return
}

func (apiCfg *ApiCfg) handlerViewTmpRecording(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("RefreshToken")
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}

	dbBearer, err := apiCfg.Db.GetRefreshToken(r.Context(), cookie.Value)
	if err != nil || dbBearer.ExpiredBool == false || dbBearer.RevokeCheck == false {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}
	
	if apiCfg.WriterCfg.RecordingUser != dbBearer.UserID {
		resp.RespondWithError(w, http.StatusConflict, "Another User is currently recording", err)
		return
	}

	videoPath := filepath.Join(".", "videos", "tmp.mp4")
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		http.Error(w, "No recent recording found", http.StatusNotFound)
		return
	}

	w.Header().Set("X-Video-Lift-Result", fmt.Sprintf("%t", apiCfg.WriterCfg.RecordedLiftResult))
	http.ServeFile(w, r, videoPath)
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	bytesCopied, err := io.Copy(destination, source)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully copied %d bytes from %s to %s\n", bytesCopied, src, dst)
	return nil
}


func (apiCfg *ApiCfg) handlerSaveVideo(w http.ResponseWriter, r *http.Request) {
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

	uploadParams := database.CreateVideoParams{apiCfg.WriterCfg.RecordedLiftType, userID, fmt.Sprintf("%t", apiCfg.WriterCfg.RecordedLiftResult),}

	videoRecord, err := apiCfg.Db.CreateVideo(r.Context(), uploadParams)
        if err != nil {
                resp.RespondWithError(w, http.StatusInternalServerError, "Could not save video", err)
                return
        }

	videoPathTmp := filepath.Join(".", "videos", "tmp.mp4")
	videoPath := filepath.Join(".", "videos", videoRecord.ID.String() + ".mp4")
	copyFile(videoPathTmp, videoPath)

        resp.RespondWithJSON(w, http.StatusNoContent, nil)
        return
}

func (apiCfg *ApiCfg) handlerGetSingleUserVideos(w http.ResponseWriter, r *http.Request) {
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

	videos, err := apiCfg.Db.GetSingleUserVideos(r.Context(), userID)
        if err != nil {
                resp.RespondWithError(w, http.StatusBadRequest, "Failed to get user videos", err)
                return
        }
        
	type returnVal struct {
                Id              uuid.UUID       `json:"id"`
                Date      	time.Time       `json:"date"`
                Label           string          `json:"label"`
        }

	returnValList := []returnVal{}
	for _, video := range videos {
		returnValList = append(returnValList, returnVal{video.ID, video.CreatedAt, video.LiftType})
	}

        resp.RespondWithJSON(w, http.StatusOK, returnValList)
        return
}

func (apiCfg *ApiCfg) handlerGetSingleUserSingleVideo(w http.ResponseWriter, r *http.Request) {
	videoID, err := uuid.Parse(r.PathValue("videoID"))
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Not a proper video ID", err)
		return
	}

	cookie, err := r.Cookie("RefreshToken")
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}

	dbBearer, err := apiCfg.Db.GetRefreshToken(r.Context(), cookie.Value)
	if err != nil || dbBearer.ExpiredBool == false || dbBearer.RevokeCheck == false {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}
	
	videoPath := filepath.Join(".", "videos", videoID.String() + ".mp4")
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		http.Error(w, "No video found with this ID", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, videoPath)
	return
}

func (apiCfg *ApiCfg) handlerDeleteSingleUserSingleVideo(w http.ResponseWriter, r *http.Request) {
	videoID, err := uuid.Parse(r.PathValue("videoID"))
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Not a proper video ID", err)
		return
	}

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
	
	video, err := apiCfg.Db.GetVideo(r.Context(), videoID)
	if err != nil || video.UserID != userID {
		resp.RespondWithError(w, http.StatusUnauthorized, "No video with this ID or user not authorized", err)
		return
	}
	
	err = apiCfg.Db.DeleteVideoById(r.Context(), videoID)
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Error deleting file from database", err)
		return
	}

	videoPath := filepath.Join(".", "videos", videoID.String() + ".mp4")
	err = os.Remove(videoPath)
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Error deleting file from disk", err)
		fmt.Printf("Error deleting file: %v\n", err)
		return
	}
	return
}

func (apiCfg *ApiCfg) handlerGetTmpRecordingResult(w http.ResponseWriter, r *http.Request) {
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

	type returnVal struct {
                Result	bool	`json:"result"`
        }
        
	resp.RespondWithJSON(w, http.StatusOK, returnVal{Result: apiCfg.WriterCfg.RecordedLiftResult})
	return
}

func (apiCfg *ApiCfg) handlerGetSingleUserSingleVideoResult(w http.ResponseWriter, r *http.Request) {
	videoID, err := uuid.Parse(r.PathValue("videoID"))
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Not a proper video ID", err)
		return
	}

	cookie, err := r.Cookie("RefreshToken")
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}

	dbBearer, err := apiCfg.Db.GetRefreshToken(r.Context(), cookie.Value)
	if err != nil || dbBearer.ExpiredBool == false || dbBearer.RevokeCheck == false {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}
	
	video, err := apiCfg.Db.GetVideo(r.Context(), videoID)
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Error getting file from database", err)
		return
	}
	
	type returnVal struct {
                Result	string	`json:"result"`
        }
        
	resp.RespondWithJSON(w, http.StatusOK, returnVal{Result: video.LiftResult})
	return
}
