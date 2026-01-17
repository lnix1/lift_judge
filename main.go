package main

import (
        "log"
        "os"
        "os/exec"
        "syscall"
	"strconv"
	"fmt"

	constants "github.com/lnix1/lift_judge/internal/constants"
	server "github.com/lnix1/lift_judge/internal/server"
	video "github.com/lnix1/lift_judge/internal/video_feed"
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
        ringBuffer, err := openSharedSystemMemory(constants.TotalSize, constants.ShmPath)
        if err != nil {
                log.Fatal(err)
        }
        
	recordingBuffer, err := openSharedSystemMemory(constants.MaxRecordingSize, constants.ShmPathRecording)
        if err != nil {
                log.Fatal(err)
        }
        
	writer := &video.RingBufferWriter{
                Data:    ringBuffer,
                TempBuf: make([]byte, 0, constants.SlotSize),
		RecordedData: recordingBuffer,
        }

        go server.StartServer(writer)

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
		)
        	cmdAnnotator.Stdout = os.Stdout
        	cmdAnnotator.Stderr = os.Stderr
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

        cmd.Stdout = writer
        cmd.Stderr = os.Stderr

        log.Println("Starting camera process...")
        if err := cmd.Run(); err != nil {
                log.Fatal(err)
        }
}
