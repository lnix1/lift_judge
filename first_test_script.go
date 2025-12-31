package main

import (
        "encoding/binary"
        "fmt"
        "log"
        "net/http"
        "os"
        "os/exec"
        "syscall"
        "time"
)

const (
        NumSlots   = 10
        SlotSize   = 512 * 1024
        HeaderSize = 128
        TotalSize  = HeaderSize + (NumSlots * (HeaderSize + SlotSize))
        ShmPath    = "/dev/shm/camera_ring_buffer"
	
	StatusEmpty      = 0
	StatusRaw        = 1
	StatusProcessing = 2
	StatusReady      = 3
)

type RingBufferWriter struct {
        data       []byte
        tempBuf    []byte
        writeIndex int
}

func (w *RingBufferWriter) Write(p []byte) (n int, err error) {
        for _, b := range p {
                w.tempBuf = append(w.tempBuf, b)

                // Detect Start of Image (SOI)
                if len(w.tempBuf) >= 2 && w.tempBuf[len(w.tempBuf)-2] == 0xFF && w.tempBuf[len(w.tempBuf)-1] == 0xD8 {
                        if len(w.tempBuf) > 2 {
                                w.tempBuf = []byte{0xFF, 0xD8} // Reset to sync if we missed an EOI
                        }
                }

                // Detect End of Image (EOI)
                if len(w.tempBuf) >= 2 && w.tempBuf[len(w.tempBuf)-2] == 0xFF && w.tempBuf[len(w.tempBuf)-1] == 0xD9 {
                        if len(w.tempBuf) <= SlotSize {
				blockStart := HeaderSize + (w.writeIndex * (HeaderSize + SlotSize))

				copy(w.data[blockStart+HeaderSize:], w.tempBuf)
				
				// Write the status and length values for the slot
				w.data[blockStart] = StatusRaw
				binary.LittleEndian.PutUint32(w.data[blockStart+4:], uint32(len(w.tempBuf)))

				// Clear the 40 bytes of joint data (starts at byte 8 of the block)
				for i := 0; i < 40; i++ {
					w.data[blockStart+8+i] = 0
				}

                                // Update current write index (Header byte 0)
                                w.data[0] = byte(w.writeIndex)

                                w.writeIndex = (w.writeIndex + 1) % NumSlots
                        }
                        w.tempBuf = w.tempBuf[:0]
                }
        }
        return len(p), nil
}

func startServer(mmap []byte) {
        http.HandleFunc("/video_feed", func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
                w.Header().Set("Cache-Control", "no-cache")
                w.Header().Set("Connection", "keep-alive")

                lastSentIndex := -1

                for {
                        currentIndex := int(mmap[0])

			blockStart := HeaderSize + (currentIndex * (HeaderSize + SlotSize))

			status := mmap[blockStart]

                        if currentIndex != lastSentIndex && (status == StatusReady || status == StatusRaw) {
				frameLen := binary.LittleEndian.Uint32(mmap[blockStart+4 : blockStart+8])

				jpegStart := blockStart + HeaderSize
				actualJPEG := mmap[jpegStart : jpegStart+int(frameLen)]

                                // Stream to browser
                                fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", frameLen)
                                w.Write(actualJPEG)
                                fmt.Fprintf(w, "\r\n")

                                lastSentIndex = currentIndex
                        }
                        time.Sleep(10 * time.Millisecond)
                }
        })


        log.Println("HTTP Server starting on http://0.0.0.0:8080/video_feed")
        http.ListenAndServe(":8080", nil)
}

func main() {
        f, err := os.OpenFile(ShmPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
        if err != nil {
                log.Fatal(err)
        }
        if err := f.Truncate(int64(TotalSize)); err != nil {
                log.Fatal(err)
        }

        mmap, err := syscall.Mmap(int(f.Fd()), 0, TotalSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
        if err != nil {
                log.Fatal(err)
        }

        go startServer(mmap)

	go func() {
        	log.Println("Starting C++ Annotator...")
        	cmdAnnotator := exec.Command("./annotator")
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
                "--framerate", "30",
                "--nopreview",
                "-o", "-",
        )

        writer := &RingBufferWriter{
                data:    mmap,
                tempBuf: make([]byte, 0, SlotSize),
        }
        cmd.Stdout = writer
        cmd.Stderr = os.Stderr

        log.Println("Starting camera process...")
        if err := cmd.Run(); err != nil {
                log.Fatal(err)
        }
}
