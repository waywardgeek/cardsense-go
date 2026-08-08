package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/waywardgeek/cardsense-go/pkg/detector"
	"github.com/waywardgeek/cardsense-go/pkg/tts"
)

const VERSION = "0.3.0"

// MacOS voices matching Python version with their actual identifiers
var VOICES = []string{
	"Samantha",
	"Albert",
	"Daniel",
	"Eddy (English (US))",
	"Flo (English (US))",
	"Fred",
	"Junior",
	"Kathy",
	"Reed (English (US))",
	"Rocko (English (US))",
	"Sandy (English (US))",
	"Shelley (English (US))",
}

// Map display names to actual macOS voice identifiers
var VOICE_MAP = map[string]string{
	"Samantha":            "com.apple.speech.synthesis.voice.samantha",
	"Albert":              "com.apple.speech.synthesis.voice.albert",
	"Daniel":              "com.apple.speech.synthesis.voice.daniel",
	"Eddy (English (US))": "com.apple.speech.synthesis.voice.eddy",
	"Flo (English (US))":  "com.apple.speech.synthesis.voice.flo",
	"Fred":                "com.apple.speech.synthesis.voice.fred",
	"Junior":              "com.apple.speech.synthesis.voice.junior",
	"Kathy":               "com.apple.speech.synthesis.voice.kathy",
	"Reed (English (US))": "com.apple.eloquence.en-US.Reed",
	"Rocko (English (US))":"com.apple.eloquence.en-US.Rocko",
	"Sandy (English (US))":"com.apple.eloquence.en-US.Sandy",
	"Shelley (English (US))":"com.apple.eloquence.en-US.Shelley",
}

// BuildGUI creates and shows the CardSense GUI
func BuildGUI(dataDir string, debug bool) {
	myApp := app.NewWithID("com.coderhapsody.cardsense-go")
	myWindow := myApp.NewWindow(fmt.Sprintf("CardSense Go v%s", VERSION))

	// Create speaker
	speaker, err := tts.NewSpeaker()
	if err != nil {
		panic(fmt.Sprintf("Failed to create speaker: %v", err))
	}

	// Status label
	statusLabel := widget.NewLabel("⏸️  Press Start to begin watching")
	statusLabel.Wrapping = fyne.TextWrapWord

	// Create detector with status callback
	var det *detector.Detector
	
	// Status updates from detector goroutine need to be queued for main thread
	statusChan := make(chan string, 10)
	go func() {
		for text := range statusChan {
			text := text // Capture for closure
			fyne.Do(func() {
				statusLabel.SetText(text)
			})
		}
	}()
	
	setStatus := func(text string) {
		select {
		case statusChan <- text:
		default:
			// Channel full, skip update
		}
	}
	det = detector.New(speaker, dataDir, setStatus, debug)

	// Speed slider (150-900 WPM, default 550)
	speedLabel := widget.NewLabel("550")
	speedSlider := widget.NewSlider(150, 900)
	speedSlider.Value = 550
	speedSlider.Step = 10
	speedSlider.OnChanged = func(value float64) {
		wpm := int(value)
		speedLabel.SetText(fmt.Sprintf("%d", wpm))
		speaker.SetRate(wpm)
	}

	// Use BorderLayout: label on left, value on right, slider fills center
	speedFrame := container.NewBorder(nil, nil,
		widget.NewLabel("Speed (WPM):"),
		speedLabel,
		speedSlider,
	)

	// Voice picker
	voiceSelect := widget.NewSelect(VOICES, func(value string) {
		fmt.Printf("[GUI] Voice changed to: %s\n", value)
		// Look up the actual voice identifier
		if voiceID, ok := VOICE_MAP[value]; ok {
			fmt.Printf("[GUI] Using voice ID: %s\n", voiceID)
			speaker.SetVoice(voiceID)
		}
	})
	voiceSelect.SetSelected("Reed (English (US))")
	
	// Set initial voice FIRST
	fmt.Printf("[GUI] Setting initial voice: Reed (English (US))\n")
	if voiceID, ok := VOICE_MAP["Reed (English (US))"]; ok {
		fmt.Printf("[GUI] Using voice ID: %s\n", voiceID)
		speaker.SetVoice(voiceID)
	}
	
	// THEN set initial rate (after voice, so voice doesn't reset it)
	fmt.Printf("[GUI] Setting initial rate: 550 WPM\n")
	speaker.SetRate(550)

	// Test button
	testButton := widget.NewButton("Test", func() {
		speaker.Speak("Llanowar Elves. Creature, Elf Druid. Tap: Add one green mana.")
	})

	voiceFrame := container.NewHBox(
		widget.NewLabel("Voice:"),
		voiceSelect,
		testButton,
	)

	// Start/Stop buttons
	var startButton, stopButton *widget.Button

	startButton = widget.NewButton("▶  Start", func() {
		det.Start()
		startButton.Disable()
		stopButton.Enable()
	})

	stopButton = widget.NewButton("■  Stop", func() {
		det.Stop()
		startButton.Enable()
		stopButton.Disable()
		setStatus("⏹️  Stopped - Press Start to resume")
	})
	stopButton.Disable()

	// Recalibrate: discard the current calibration and search again from the
	// reference box scaled to the current display. Needed when moving between
	// displays (e.g. 4K external -> laptop panel), where the saved box is
	// specific to the resolution it was measured on.
	recalibrateButton := widget.NewButton("🎯  Recalibrate", func() {
		det.RequestRecalibrate()
		setStatus("🎯 Recalibrating - hover a card...")
	})

	buttonFrame := container.NewHBox(
		layout.NewSpacer(),
		startButton,
		stopButton,
		recalibrateButton,
		layout.NewSpacer(),
	)

	// Main content
	content := container.NewVBox(
		statusLabel,
		widget.NewSeparator(),
		speedFrame,
		voiceFrame,
		widget.NewSeparator(),
		buttonFrame,
	)

	// Window close handler
	myWindow.SetOnClosed(func() {
		det.Stop()
		speaker.Close()
	})

	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(480, 280))
	myWindow.SetFixedSize(true)
	myWindow.ShowAndRun()
}
