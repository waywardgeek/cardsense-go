// +build darwin

package tts

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework Foundation
#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>

// Wrapper functions to interface with Objective-C from Go
void* createSpeechSynthesizer(const char* voice) {
    NSString *voiceStr = [NSString stringWithUTF8String:voice];
    NSSpeechSynthesizer *synth = [[NSSpeechSynthesizer alloc] initWithVoice:voiceStr];
    return (void*)CFBridgingRetain(synth);
}

void releaseSpeechSynthesizer(void* synth) {
    if (synth) {
        CFBridgingRelease(synth);
    }
}

void speak(void* synth, const char* text) {
    NSSpeechSynthesizer *synthesizer = (__bridge NSSpeechSynthesizer*)synth;
    NSString *str = [NSString stringWithUTF8String:text];
    [synthesizer startSpeakingString:str];
}

void stopSpeaking(void* synth) {
    NSSpeechSynthesizer *synthesizer = (__bridge NSSpeechSynthesizer*)synth;
    [synthesizer stopSpeaking];
}

void setRate(void* synth, float rate) {
    NSSpeechSynthesizer *synthesizer = (__bridge NSSpeechSynthesizer*)synth;
    [synthesizer setRate:rate];
}

void setVoiceByIdentifier(void* synth, const char* voice) {
    NSSpeechSynthesizer *synthesizer = (__bridge NSSpeechSynthesizer*)synth;
    NSString *voiceStr = [NSString stringWithUTF8String:voice];
    [synthesizer setVoice:voiceStr];
}

int isSpeaking(void* synth) {
    NSSpeechSynthesizer *synthesizer = (__bridge NSSpeechSynthesizer*)synth;
    return [synthesizer isSpeaking] ? 1 : 0;
}

// Run the NSRunLoop for a brief period to pump events
void pumpRunLoop() {
    NSDate *date = [NSDate dateWithTimeIntervalSinceNow:0.05];
    [[NSRunLoop currentRunLoop] runUntilDate:date];
}
*/
import "C"
import (
	"sync"
	"time"
	"unsafe"
)

// MacOSSpeaker implements Speaker using NSSpeechSynthesizer with NSRunLoop pumping
type MacOSSpeaker struct {
	synth    unsafe.Pointer
	rate     int
	voice    string
	mu       sync.Mutex
	stopLoop chan struct{}
	ready    chan struct{}
}

// newPlatformSpeaker creates a macOS speaker using NSSpeechSynthesizer
func newPlatformSpeaker() (Speaker, error) {
	// Use the correct Reed voice identifier
	voice := "com.apple.eloquence.en-US.Reed"
	
	speaker := &MacOSSpeaker{
		rate:     550,
		voice:    voice,
		stopLoop: make(chan struct{}),
		ready:    make(chan struct{}),
	}

	// Start the runloop in a dedicated goroutine
	go speaker.runLoop()
	
	// Wait for initialization
	<-speaker.ready

	return speaker, nil
}

// runLoop runs on a dedicated goroutine and pumps NSRunLoop
func (s *MacOSSpeaker) runLoop() {
	// Create synthesizer on this thread
	cvoice := C.CString(s.voice)
	s.synth = C.createSpeechSynthesizer(cvoice)
	C.free(unsafe.Pointer(cvoice))
	
	if s.synth == nil {
		close(s.ready)
		return
	}
	
	// Set initial rate
	C.setRate(s.synth, C.float(s.rate))
	
	// Signal ready
	close(s.ready)
	
	// Pump the runloop every 50ms (matches Python)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopLoop:
			return
		case <-ticker.C:
			C.pumpRunLoop()
		}
	}
}

// Speak speaks the given text (cancels any current speech)
func (s *MacOSSpeaker) Speak(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop any current speech
	C.stopSpeaking(s.synth)

	// Start new speech
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	C.speak(s.synth, ctext)
}

// Cancel stops any current speech
func (s *MacOSSpeaker) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	C.stopSpeaking(s.synth)
}

// SetRate sets the speech rate in words per minute
func (s *MacOSSpeaker) SetRate(wpm int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rate = wpm
	C.setRate(s.synth, C.float(wpm))
}

// SetVoice sets the voice by identifier (e.g., "com.apple.eloquence.en-US.Reed")
func (s *MacOSSpeaker) SetVoice(identifier string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.voice = identifier
	cvoice := C.CString(identifier)
	defer C.free(unsafe.Pointer(cvoice))
	C.setVoiceByIdentifier(s.synth, cvoice)
}

// IsSpeaking returns true if currently speaking
func (s *MacOSSpeaker) IsSpeaking() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return C.isSpeaking(s.synth) != 0
}

// Close cleans up resources
func (s *MacOSSpeaker) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Stop the runloop
	close(s.stopLoop)
	
	if s.synth != nil {
		C.stopSpeaking(s.synth)
		C.releaseSpeechSynthesizer(s.synth)
		s.synth = nil
	}
	return nil
}
