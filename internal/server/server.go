package server

import (
	"net/http"
	"log"
	"fmt"
	"net"
	
	video "github.com/lnix1/lift_judge/internal/video_feed"
)

func getWlanIP() (string, error) {
	iface, err := net.InterfaceByName("wlan0")
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no IPv4 address found for wlan0")
}

func StartServer(writer *video.RingBufferWriter) {
	const staticFilePath = "./static"
	const port = "8080"

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir(staticFilePath)))
        mux.HandleFunc("GET /video_feed", writer.HandlerVideoFeed)
	mux.HandleFunc("POST /start_recording", writer.HandlerStartRecording)
	mux.HandleFunc("POST /stop_recording", writer.HandlerStopRecording)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	lanAddr, err := getWlanIP()
	if err != nil {
		log.Fatal("error getting address to which users will route API calls")
	}

        log.Println(fmt.Sprintf("HTTP Server starting on http://%s:8080/video_feed", lanAddr))
	log.Fatal(srv.ListenAndServe())
}
