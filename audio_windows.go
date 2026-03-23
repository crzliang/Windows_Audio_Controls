//go:build windows
// +build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"os/exec"
	"encoding/json"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"
)

const (
	VK_MEDIA_NEXT_TRACK byte = 0xB0
	VK_MEDIA_PREV_TRACK byte = 0xB1
	VK_MEDIA_STOP       byte = 0xB2
	VK_MEDIA_PLAY_PAUSE byte = 0xB3
)

var (
	moduser32       = windows.NewLazySystemDLL("user32.dll")
	procKeybdEvent  = moduser32.NewProc("keybd_event")
)

type AudioSession struct {
	PID    uint32  `json:"pid"`
	Name   string  `json:"name"`
	Volume float32 `json:"volume"`
	Mute   bool    `json:"mute"`
}

var (
	modpsapi              = windows.NewLazySystemDLL("psapi.dll")
	procGetModuleBaseName = modpsapi.NewProc("GetModuleBaseNameW")
)

func init() {
	ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
}

func getProcessName(pid uint32) string {
	if pid == 0 {
		return "System Sounds"
	}

	hProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return fmt.Sprintf("Unknown Process (%d)", pid)
	}
	defer windows.CloseHandle(hProcess)

	var name [windows.MAX_PATH]uint16
	ret, _, _ := procGetModuleBaseName.Call(uintptr(hProcess), 0, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if ret == 0 {
		return fmt.Sprintf("Unknown Process (%d)", pid)
	}

	return strings.TrimRight(windows.UTF16ToString(name[:]), "\x00")
}

func getMasterAudio() (float32, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return 0, false, err
	}
	defer mmde.Release()

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		return 0, false, err
	}
	defer mmd.Release()

	var aev *wca.IAudioEndpointVolume
	if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		return 0, false, err
	}
	defer aev.Release()

	var level float32
	var mute bool

	aev.GetMasterVolumeLevelScalar(&level)
	aev.GetMute(&mute)

	return level, mute, nil
}

func setMasterAudio(level float32, mute *bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return err
	}
	defer mmde.Release()

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		return err
	}
	defer mmd.Release()

	var aev *wca.IAudioEndpointVolume
	if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		return err
	}
	defer aev.Release()

	if mute != nil {
		aev.SetMute(*mute, nil)
	}

	if level >= 0 && level <= 1 {
		aev.SetMasterVolumeLevelScalar(level, nil)
	}

	return nil
}

func getAudioSessions() ([]AudioSession, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return nil, err
	}
	defer mmde.Release()

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		return nil, err
	}
	defer mmd.Release()

	var pAudioSessionManager2 *wca.IAudioSessionManager2
	if err := mmd.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &pAudioSessionManager2); err != nil {
		return nil, err
	}
	defer pAudioSessionManager2.Release()

	var pSessionEnumerator *wca.IAudioSessionEnumerator
	if err := pAudioSessionManager2.GetSessionEnumerator(&pSessionEnumerator); err != nil {
		return nil, err
	}
	defer pSessionEnumerator.Release()

	var sessionCount int
	if err := pSessionEnumerator.GetCount(&sessionCount); err != nil {
		return nil, err
	}

	var sessions []AudioSession

	for i := 0; i < sessionCount; i++ {
		var pSessionControl *wca.IAudioSessionControl
		if err := pSessionEnumerator.GetSession(i, &pSessionControl); err != nil {
			continue
		}

		dispatch, err := pSessionControl.QueryInterface(wca.IID_IAudioSessionControl2)
		if err != nil {
			pSessionControl.Release()
			continue
		}
		pSessionControl2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(dispatch))

		var state uint32
		pSessionControl.GetState(&state)
		// wca.AudioSessionStateActive = 1
		if state != 1 {
			pSessionControl2.Release()
			pSessionControl.Release()
			continue
		}

		var pid uint32
		pSessionControl2.GetProcessId(&pid)

		dispatch2, err := pSessionControl.QueryInterface(wca.IID_ISimpleAudioVolume)
		if err != nil {
			pSessionControl2.Release()
			pSessionControl.Release()
			continue
		}
		pSimpleAudioVolume := (*wca.ISimpleAudioVolume)(unsafe.Pointer(dispatch2))

		var vol float32
		var mute bool
		pSimpleAudioVolume.GetMasterVolume(&vol)
		pSimpleAudioVolume.GetMute(&mute)

		name := getProcessName(pid)

		sessions = append(sessions, AudioSession{
			PID:    pid,
			Name:   name,
			Volume: vol,
			Mute:   mute,
		})

		pSimpleAudioVolume.Release()
		pSessionControl2.Release()
		pSessionControl.Release()
	}

	return sessions, nil
}

func setAudioSessionVolume(pid uint32, level float32, mute *bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return err
	}
	defer mmde.Release()

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		return err
	}
	defer mmd.Release()

	var pAudioSessionManager2 *wca.IAudioSessionManager2
	if err := mmd.Activate(wca.IID_IAudioSessionManager2, wca.CLSCTX_ALL, nil, &pAudioSessionManager2); err != nil {
		return err
	}
	defer pAudioSessionManager2.Release()

	var pSessionEnumerator *wca.IAudioSessionEnumerator
	if err := pAudioSessionManager2.GetSessionEnumerator(&pSessionEnumerator); err != nil {
		return err
	}
	defer pSessionEnumerator.Release()

	var sessionCount int
	if err := pSessionEnumerator.GetCount(&sessionCount); err != nil {
		return err
	}

	for i := 0; i < sessionCount; i++ {
		var pSessionControl *wca.IAudioSessionControl
		if err := pSessionEnumerator.GetSession(i, &pSessionControl); err != nil {
			continue
		}

		dispatch, err := pSessionControl.QueryInterface(wca.IID_IAudioSessionControl2)
		if err != nil {
			pSessionControl.Release()
			continue
		}
		pSessionControl2 := (*wca.IAudioSessionControl2)(unsafe.Pointer(dispatch))

		var sessionPID uint32
		pSessionControl2.GetProcessId(&sessionPID)

		if sessionPID == pid {
			dispatch2, err := pSessionControl.QueryInterface(wca.IID_ISimpleAudioVolume)
			if err == nil {
				pSimpleAudioVolume := (*wca.ISimpleAudioVolume)(unsafe.Pointer(dispatch2))
				if mute != nil {
					pSimpleAudioVolume.SetMute(*mute, nil)
				}
				if level >= 0 && level <= 1 {
					pSimpleAudioVolume.SetMasterVolume(level, nil)
				}
				pSimpleAudioVolume.Release()
			}
			pSessionControl2.Release()
			pSessionControl.Release()
			return nil
		}

		pSessionControl2.Release()
		pSessionControl.Release()
	}

	return fmt.Errorf("session not found")
}

func sendMediaKey(key byte) {
	const KEYEVENTF_KEYUP = 0x0002
	procKeybdEvent.Call(uintptr(key), 0, 0, 0)
	procKeybdEvent.Call(uintptr(key), 0, KEYEVENTF_KEYUP, 0)
}

func getMediaInfo() *MediaInfo {
	psScript := `
	$ErrorActionPreference = 'SilentlyContinue';
	Add-Type -AssemblyName System.Runtime.WindowsRuntime;
	$mgr = [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager, Windows.Media.Control, ContentType=WindowsRuntime]::RequestAsync().GetResults();
	$session = $mgr.GetCurrentSession();
	if ($session) {
		try {
			$playback = $session.GetPlaybackInfo();
			$res = @{
				'status' = $playback.PlaybackStatus;
			};
			$res | ConvertTo-Json;
		} catch {}
	}
	`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	if len(strings.TrimSpace(string(out))) == 0 {
		return nil
	}

	var info MediaInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil
	}

	return &info
}
