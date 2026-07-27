// +build windows

package tts

import (
	"fmt"
	"os/exec"
	"sync"
)

// WindowsSpeaker implements Speaker using Windows SAPI
// TODO: Replace with proper SAPI via cgo for better performance
type WindowsSpeaker struct {
	rate    int
	voice   string
	mu      sync.Mutex
	cmd     *exec.Cmd
	cancel  chan struct{}
}

// newPlatformSpeaker creates a Windows speaker
func newPlatformSpeaker() (Speaker, error) {
	return &WindowsSpeaker{
		rate:   450, // Windows SAPI max ~450 WPM
		voice:  "",  // Use default
		cancel: make(chan struct{}),
	}, nil
}

// Speak speaks the given text (cancels any current speech)
func (s *WindowsSpeaker) Speak(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel any current speech
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}

	// Start new speech in background
	go s.speakAsync(text)
}

func (s *WindowsSpeaker) speakAsync(text string) {
	s.mu.Lock()
	// TODO: Replace with proper SAPI via cgo
	// For now, use PowerShell as a fallback (slower but works)
	psScript := fmt.Sprintf(`Add-Type -AssemblyName System.Speech; $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer; $synth.Rate = 10; $synth.Speak('%s')`, text)
	s.cmd = exec.Command("powershell", "-Command", psScript)
	s.mu.Unlock()

	s.cmd.Run()
}

// Cancel stops any current speech
func (s *WindowsSpeaker) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd = nil
	}
}

// SetRate sets the speech rate in words per minute
func (s *WindowsSpeaker) SetRate(wpm int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// SAPI rate range is -10 to +10, map from WPM
	// 550 WPM -> +10
	s.rate = min(450, wpm)
}

// SetVoice sets the voice by name
func (s *WindowsSpeaker) SetVoice(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.voice = name
}

// IsSpeaking returns true if currently speaking
func (s *WindowsSpeaker) IsSpeaking() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cmd != nil && s.cmd.Process != nil
}

// Close cleans up resources
func (s *WindowsSpeaker) Close() error {
	s.Cancel()
	close(s.cancel)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
