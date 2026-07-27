package tts

// Speaker is the cross-platform TTS interface
type Speaker interface {
	// Speak speaks the given text (cancels any current speech)
	Speak(text string)
	
	// Cancel stops any current speech
	Cancel()
	
	// SetRate sets the speech rate in words per minute
	SetRate(wpm int)
	
	// SetVoice sets the voice by name
	SetVoice(name string)
	
	// IsSpeaking returns true if currently speaking
	IsSpeaking() bool
	
	// Close cleans up resources
	Close() error
}

// NewSpeaker creates a platform-specific Speaker instance
func NewSpeaker() (Speaker, error) {
	return newPlatformSpeaker()
}
