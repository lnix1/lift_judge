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
        TotalSize  = HeaderSize + (NumSlots * SlotSize)
        ShmPath    = "/dev/shm/camera_ring_buffer"
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
                                targetSlot := HeaderSize + (w.writeIndex * SlotSize)

                                // Copy frame to shared memory
                                copy(w.data[targetSlot:], w.tempBuf)

                                // Store length in Table of Contents (Starts at byte 4)
                                lenOffset := 4 + (w.writeIndex * 4)
                                binary.LittleEndian.PutUint32(w.data[lenOffset:lenOffset+4], uint32(len(w.tempBuf)))

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

                        if currentIndex != lastSentIndex {
                                // Read length from Table of Contents
                                lenOffset := 4 + (currentIndex * 4)
                                frameLen := binary.LittleEndian.Uint32(mmap[lenOffset : lenOffset+4])

                                targetSlot := HeaderSize + (currentIndex * SlotSize)
                                actualJPEG := mmap[targetSlot : targetSlot+int(frameLen)]

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
        // 1. Setup Shared Memory
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

        // 2. Start HTTP Server in background
        go startServer(mmap)

        // 3. Configure and Start Camera
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
