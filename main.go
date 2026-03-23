package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"

	"github.com/getlantern/systray"
)

//go:embed web/* assets/icon.ico
var embeddedFiles embed.FS

type StatusResponse struct {
	MasterVolume float32        `json:"masterVolume"`
	MasterMute   bool           `json:"masterMute"`
	Sessions     []AudioSession `json:"sessions"`
}

func main() {
	systray.Run(onReady, onExit)
}

func onExit() {
	// cleanup here
}

func onReady() {
	iconData, err := embeddedFiles.ReadFile("assets/icon.ico")
	if err == nil {
		systray.SetIcon(iconData)
	}
	systray.SetTitle("Windows Audio Controller")
	systray.SetTooltip("Audio Web API Running (Port 8080)")

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	// Start web server in background
	go func() {
		// API routes
		http.HandleFunc("/api/status", handleStatus)
		http.HandleFunc("/api/master/volume", handleMasterVolume)
		http.HandleFunc("/api/sessions/volume", handleSessionVolume)

		// Serve Static Files
		webFS, err := fs.Sub(embeddedFiles, "web")
		if err != nil {
			log.Fatal(err)
		}
		http.Handle("/", http.FileServer(http.FS(webFS)))

		port := 8080
		fmt.Printf("Starting Windows Audio Controller on http://0.0.0.0:%d\n", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Fatal(err)
		}
	}()
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	masterLevel, masterMute, err := getMasterAudio()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessions, err := getAudioSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if sessions == nil {
		sessions = []AudioSession{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{
		MasterVolume: masterLevel,
		MasterMute:   masterMute,
		Sessions:     sessions,
	})
}

func handleMasterVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var level float32 = -1
	var mute *bool

	levelStr := r.URL.Query().Get("level")
	if levelStr != "" {
		l, err := strconv.ParseFloat(levelStr, 32)
		if err == nil {
			level = float32(l)
		}
	}

	muteStr := r.URL.Query().Get("mute")
	if muteStr != "" {
		m, err := strconv.ParseBool(muteStr)
		if err == nil {
			mValue := m // create copy for pointer
			mute = &mValue
		}
	}

	if err := setMasterAudio(level, mute); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleSessionVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pidStr := r.URL.Query().Get("pid")
	pid, err := strconv.ParseUint(pidStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	var level float32 = -1
	var mute *bool

	levelStr := r.URL.Query().Get("level")
	if levelStr != "" {
		l, err := strconv.ParseFloat(levelStr, 32)
		if err == nil {
			level = float32(l)
		}
	}

	muteStr := r.URL.Query().Get("mute")
	if muteStr != "" {
		m, err := strconv.ParseBool(muteStr)
		if err == nil {
			mValue := m
			mute = &mValue
		}
	}

	if err := setAudioSessionVolume(uint32(pid), level, mute); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
