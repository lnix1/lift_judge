package main

import (
        "log"
        "os"
        "os/exec"
        "syscall"

	constants "github.com/lnix1/lift_judge/internal/constants"
	server "github.com/lnix1/lift_judge/internal/server"
	video "github.com/lnix1/lift_judge/internal/video_feed"
)

func main() {
        f, err := os.OpenFile(constants.ShmPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
        if err != nil {
                log.Fatal(err)
        }
        if err := f.Truncate(int64(constants.TotalSize)); err != nil {
                log.Fatal(err)
        }

        mmap, err := syscall.Mmap(int(f.Fd()), 0, constants.TotalSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
        if err != nil {
                log.Fatal(err)
        }

        go server.StartServer(mmap)

	go func() {
        	log.Println("Starting C++ Annotator...")
        	//cmdAnnotator := exec.Command("./internal/annotators/yolov8/annotator")
        	cmdAnnotator := exec.Command("./internal/annotators/media_pipe/annotator")
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
                "--framerate", "15",
                "--nopreview",
                "-o", "-",
        )

        writer := &video.RingBufferWriter{
                Data:    mmap,
                TempBuf: make([]byte, 0, constants.SlotSize),
        }
        cmd.Stdout = writer
        cmd.Stderr = os.Stderr

        log.Println("Starting camera process...")
        if err := cmd.Run(); err != nil {
                log.Fatal(err)
        }
}
