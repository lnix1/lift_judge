package main

import _ "github.com/lib/pq"

import (
        "log"
        "os"
        "os/exec"
        "syscall"
	"strconv"
	"fmt"
	"database/sql"

	"github.com/lnix1/lift_judge/internal/database"
	constants "github.com/lnix1/lift_judge/internal/constants"
	server "github.com/lnix1/lift_judge/internal/server"
	video "github.com/lnix1/lift_judge/internal/video_feed"

	"github.com/joho/godotenv"
)

func openSharedSystemMemory(size int, shmPath string) (mmap []byte, err error) {
        f, err := os.OpenFile(shmPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
        if err != nil {
                log.Fatal(err)
        }

        if err := f.Truncate(int64(size)); err != nil {
                log.Fatal(err)
        }

        return syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
}

func main() {
	if constants.NumSlots > 255 {
		log.Fatal("Too many slots in ring buffer, must be <= 255")
	}

        ringBuffer, err := openSharedSystemMemory(constants.TotalSize, constants.ShmPath)
        if err != nil {
                log.Fatal(err)
        }
        
	recordingBuffer, err := openSharedSystemMemory(constants.MaxRecordingSize, constants.ShmPathRecording)
        if err != nil {
                log.Fatal(err)
        }

	annotatorTrigger, err := video.NewSemaphore(constants.AnnotationTriggerSem)
        if err != nil {
                log.Fatal(err)
        }
	defer annotatorTrigger.Close()
        
	writerCfg := &video.RingBufferWriter{
                Data:    ringBuffer,
                TempBuf: make([]byte, 0, constants.SlotSize),
		RecordedData: recordingBuffer,
		AnnotatorTrigger: annotatorTrigger,
        }
	
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")

	if dbURL == "" {
		log.Fatal("DB_URL must be set")
	}
	if secret == "" {
		log.Fatal("Secret must be set")
	}

	dbConn, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %s", err)
	}

	dbQueries := database.New(dbConn)

	apiCfg := server.ApiCfg{
		Db:             dbQueries,
		Platform: 	platform,
		Secret: 	secret,
		WriterCfg: 	writerCfg,
	}

        go apiCfg.StartServer()

	go func() {
        	log.Println("Starting C++ Annotator...")
        	//cmdAnnotator := exec.Command("./internal/annotators/yolov8/annotator")
        	cmdAnnotator := exec.Command("./internal/annotators/media_pipe/annotator", 
			fmt.Sprintf("--headersize=%d", constants.HeaderSize), 
			fmt.Sprintf("--slotsize=%d", constants.SlotSize), 
			fmt.Sprintf("--numslots=%d", constants.NumSlots), 
			fmt.Sprintf("--detectionconfidence=%f", constants.DetectionConfidence), 
			fmt.Sprintf("--isring=%t", true), 
			fmt.Sprintf("--shmpath=%s", constants.ShmPathCpp), 
			fmt.Sprintf("--sempath=%s", constants.AnnotationTriggerSem), 
		)
        	cmdAnnotator.Stdout = os.Stdout
        	//cmdAnnotator.Stderr = os.Stderr
        	if err := cmdAnnotator.Run(); err != nil {
            		log.Printf("Annotator error: %v", err)
        	}
    	}()

        cmd := exec.Command("rpicam-vid",
                "-t", "0",
                "--codec", "mjpeg",
                "--width", "640",
                "--height", "640",
                "--framerate", strconv.Itoa(constants.FramesPerSecond),
                "--nopreview",
                "-o", "-",
        )

        cmd.Stdout = apiCfg.WriterCfg
        //cmd.Stderr = os.Stderr

        log.Println("Starting camera process...")
        if err := cmd.Run(); err != nil {
                log.Fatal(err)
        }
}
