//go:build !windows
// +build !windows

package main

import "fmt"

type AudioSession struct {
	PID    uint32  `json:"pid"`
	Name   string  `json:"name"`
	Volume float32 `json:"volume"`
	Mute   bool    `json:"mute"`
}

func getMasterAudio() (float32, bool, error) {
	return 0.5, false, nil
}

func setMasterAudio(level float32, mute *bool) error {
	fmt.Printf("STUB: setMasterAudio level=%v mute=%v\n", level, mute)
	return nil
}

func getAudioSessions() ([]AudioSession, error) {
	return []AudioSession{
		{PID: 1234, Name: "StubApp.exe", Volume: 0.8, Mute: false},
	}, nil
}

func setAudioSessionVolume(pid uint32, level float32, mute *bool) error {
	fmt.Printf("STUB: setAudioSessionVolume pid=%d level=%v mute=%v\n", pid, level, mute)
	return nil
}
