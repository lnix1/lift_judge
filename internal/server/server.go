package server

import (
	"net/http"
	"log"
	"fmt"
	"net"
	
	video "github.com/lnix1/lift_judge/internal/video_feed"
	"github.com/lnix1/lift_judge/internal/database"
)

type ApiCfg struct {
	Db             	*database.Queries
	Platform       	string
	Secret		string
	WriterCfg	*video.RingBufferWriter
}

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

func (apiCfg *ApiCfg) StartServer() {
	const staticFilePath = "./static"
	const port = "8080"

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir(staticFilePath)))
	mux.HandleFunc("GET /api/video_feed", apiCfg.handlerVideoFeed)
	mux.HandleFunc("POST /api/start_recording", apiCfg.handlerStartRecording)
	mux.HandleFunc("POST /api/stop_recording", apiCfg.handlerStopRecording)
	mux.HandleFunc("GET /api/view_tmp", apiCfg.handlerViewTmpRecording)
	mux.HandleFunc("POST /api/save_tmp", apiCfg.handlerSaveVideo)
	
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)
	mux.HandleFunc("PUT /api/users", apiCfg.handlerUpdateEmailPassword)
	mux.HandleFunc("POST /api/login", apiCfg.handlerLogin)
	mux.HandleFunc("GET /api/user/videos", apiCfg.handlerGetSingleUserVideos)

	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	lanAddr, err := getWlanIP()
	if err != nil {
		log.Fatal("error getting address to which users will route API calls")
	}

        log.Println(fmt.Sprintf("HTTP Server starting on http://%s:8080/", lanAddr))
	log.Fatal(srv.ListenAndServe())
}
