package server

import (
	"net/http"
	"log"
	"fmt"
	"time"
        "encoding/binary"
	
	constants "github.com/lnix1/lift_judge/internal/constants"
)

func StartServer(mmap []byte) {
        http.HandleFunc("/video_feed", func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
                w.Header().Set("Cache-Control", "no-cache")
                w.Header().Set("Connection", "keep-alive")

                lastSentIndex := -1

                for {
                        currentIndex := int(mmap[0])

			blockStart := constants.HeaderSize + (currentIndex * (constants.HeaderSize + constants.SlotSize))

			status := mmap[blockStart]

                        if currentIndex != lastSentIndex && (status == constants.StatusReady || status == constants.StatusRaw) {
				frameLen := binary.LittleEndian.Uint32(mmap[blockStart+4 : blockStart+8])

				jpegStart := blockStart + constants.HeaderSize
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
