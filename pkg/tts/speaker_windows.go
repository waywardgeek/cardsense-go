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
	// Convert WPM to SAPI rate (-10 to +10)
	// SAPI rate scale: -10 (slow) to +10 (fast)
	// Map 150 WPM -> -10, 450 WPM -> +10 (linear mapping)
	sapiRate := (s.rate - 300) / 15
	if sapiRate < -10 {
		sapiRate = -10
	} else if sapiRate > 10 {
		sapiRate = 10
	}

	// Build PowerShell script with proper rate and optional voice
	var psScript string
	if s.voice != "" {
		// Set specific voice if provided
		psScript = fmt.Sprintf(`Add-Type -AssemblyName System.Speech; $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer; $synth.SelectVoice('%s'); $synth.Rate = %d; $synth.Speak('%s')`, s.voice, sapiRate, text)
	} else {
		// Use default voice
		psScript = fmt.Sprintf(`Add-Type -AssemblyName System.Speech; $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer; $synth.Rate = %d; $synth.Speak('%s')`, sapiRate, text)
	}

	// TODO: Replace with proper SAPI via cgo for better performance (eliminates ~200-500ms PowerShell overhead)
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
// Maps WPM to SAPI rate: 150 WPM -> -10, 300 WPM -> 0, 450 WPM -> +10
func (s *WindowsSpeaker) SetRate(wpm int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rate = wpm
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
