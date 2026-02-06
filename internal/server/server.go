package server

import (
	"net/http"
	"log"
	
	video "github.com/lnix1/lift_judge/internal/video_feed"
	"github.com/lnix1/lift_judge/internal/database"
)

type ApiCfg struct {
	Db             	*database.Queries
	Platform       	string
	Secret		string
	WriterCfg	*video.RingBufferWriter
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
	mux.HandleFunc("GET /api/view_tmp/result", apiCfg.handlerGetTmpRecordingResult)
	mux.HandleFunc("POST /api/save_tmp", apiCfg.handlerSaveVideo)
	
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)
	mux.HandleFunc("PUT /api/users", apiCfg.handlerUpdateEmailPassword)
	mux.HandleFunc("POST /api/login", apiCfg.handlerLogin)
	mux.HandleFunc("GET /api/user/videos", apiCfg.handlerGetSingleUserVideos)
	mux.HandleFunc("GET /api/user/video/{videoID}", apiCfg.handlerGetSingleUserSingleVideo)
	mux.HandleFunc("GET /api/user/video/{videoID}/result", apiCfg.handlerGetSingleUserSingleVideoResult)
	mux.HandleFunc("/view/", func(w http.ResponseWriter, r *http.Request) {http.ServeFile(w, r, "./static/view.html")})
	mux.HandleFunc("POST /api/user/video/delete/{videoID}", apiCfg.handlerDeleteSingleUserSingleVideo)

	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

        log.Println("HTTP Server starting on http://raspberrypi.local:8080/")
	log.Fatal(srv.ListenAndServe())
}
