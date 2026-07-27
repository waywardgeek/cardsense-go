package detector

import (
	"fmt"
	"image"
	"log"
	"strings"
	"time"

	"gocv.io/x/gocv"

	"github.com/waywardgeek/cardsense-go/pkg/capture"
	"github.com/waywardgeek/cardsense-go/pkg/hash"
	"github.com/waywardgeek/cardsense-go/pkg/tts"
)

const (
	// NO_CARD_FRAMES is the number of consecutive empty frames before resetting lastCard
	NO_CARD_FRAMES = 10
	
	// Frame interval in seconds (~20 fps)
	FRAME_INTERVAL = 50 * time.Millisecond
)

// StatusCallback is called with status updates for the GUI
type StatusCallback func(string)

// Detector runs the main card detection loop
type Detector struct {
	speaker     tts.Speaker
	onStatus    StatusCallback
	debug       bool
	running     bool
	idx         *hash.CardIndex
	interval    time.Duration
	dataDir     string
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// New creates a new Detector
func New(speaker tts.Speaker, dataDir string, onStatus StatusCallback, debug bool) *Detector {
	return &Detector{
		speaker:     speaker,
		onStatus:    onStatus,
		debug:       debug,
		dataDir:     dataDir,
		interval:    FRAME_INTERVAL,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}
}

// Start begins the detection loop in a background goroutine
func (d *Detector) Start() {
	if d.running {
		return
	}
	d.running = true
	
	// Recreate channels for restart support
	d.stopChan = make(chan struct{})
	d.stoppedChan = make(chan struct{})
	
	go d.loop()
}

// Stop signals the detection loop to stop
func (d *Detector) Stop() {
	if !d.running {
		return
	}
	d.running = false
	close(d.stopChan)
	d.speaker.Cancel()
	<-d.stoppedChan // Wait for loop to finish
}

// setStatus calls the status callback if set
func (d *Detector) setStatus(text string) {
	if d.onStatus != nil {
		d.onStatus(text)
	}
}

// loop wraps loopInner with exception recovery
func (d *Detector) loop() {
	defer close(d.stoppedChan)
	defer func() {
		if r := recover(); r != nil {
			errorMsg := fmt.Sprintf("❌ CRASH: %v", r)
			log.Printf("[ERROR] Detector crashed: %v", r)
			d.setStatus(errorMsg)
		}
	}()
	
	d.loopInner()
}

// loopInner is the main detection loop (port of Python's _loop_inner)
func (d *Detector) loopInner() {
	// Load card index if not already loaded
	if d.idx == nil {
		// TODO: Auto-update check (deferred for now)
		d.setStatus("📚 Loading card index...")
		
		var err error
		d.idx, err = hash.LoadCardIndex(d.dataDir)
		if err != nil {
			d.setStatus(fmt.Sprintf("❌ ERROR loading index: %v", err))
			log.Printf("[ERROR] Failed to load CardIndex: %v", err)
			return
		}
		
		d.setStatus(fmt.Sprintf("✅ Loaded %d cards", d.idx.NumCards()))
		time.Sleep(1 * time.Second) // Show success briefly
	}
	
	// Get screen dimensions
	bounds := capture.GetPrimaryDisplayBounds()
	W, H := bounds.Dx(), bounds.Dy()
	
	// Test screen capture permission
	d.setStatus("🔐 Testing screen recording permission...")
	testShot, err := capture.CaptureScreen(0)
	if err != nil {
		d.setStatus(fmt.Sprintf("❌ Screen capture failed: %v", err))
		log.Printf("[ERROR] Screen capture test failed: %v", err)
		return
	}
	defer testShot.Close()
	
	// Check for all-black capture (permission denied)
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(testShot, &gray, gocv.ColorBGRToGray)
	_, maxVal, _, _ := gocv.MinMaxLoc(gray)
	
	if maxVal < 10 {
		d.setStatus("❌ PERMISSION DENIED: Enable Screen Recording in System Settings → Privacy & Security")
		log.Printf("[ERROR] Screen recording permission denied (all-black capture)")
		log.Printf("[FIX] System Settings → Privacy & Security → Screen Recording → Enable 'cardsense'")
		return
	}
	
	// Load calibration (saved or default, auto-scaled to current screen)
	box, calibrated := capture.LoadCalibration(d.dataDir, W, H)
	cardBox := image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H)
	
	d.setStatus("👀 Watching... right-click a card to identify")
	d.speaker.Speak("CardSense ready")
	
	var lastName string
	noCardCount := 0
	frameNum := 0
	
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	
	for d.running {
		select {
		case <-d.stopChan:
			d.setStatus("⏹️ Stopped")
			return
		case <-ticker.C:
			// Capture screen
			shot, err := capture.CaptureScreen(0)
			if err != nil {
				log.Printf("[ERROR] Screen capture failed: %v", err)
				continue
			}
			frameNum++
			
			// Crop to card box
			region := shot.Region(cardBox)
			crop := gocv.NewMat()
			region.CopyTo(&crop)
			region.Close()
			
			// Convert to grayscale for pHash
			cropGray := gocv.NewMat()
			gocv.CvtColor(crop, &cropGray, gocv.ColorBGRToGray)
			crop.Close()
			
			// Identify card (with OCR fallback enabled)
			hit := d.idx.Identify(cropGray, true, 280, 20, nil, true)
			
			// Save debug crop BEFORE closing cropGray
			var debugCrop gocv.Mat
			if d.debug {
				debugCrop = cropGray.Clone()
			}
			
			// Now safe to close cropGray
			cropGray.Close()
			shot.Close()
			
			if hit != nil {
				name := hit.Meta.Name
				dist := hit.Dist
				margin := hit.Margin
				
				// Twiddle to refine the box on first successful match
				if !calibrated {
					log.Printf("[CALIBRATE] First match: %s (dist=%d, margin=%d), auto-calibrating...", name, dist, margin)
					d.setStatus("🎯 Auto-calibrating detection area...")
					d.speaker.Speak("Calibrating")
					time.Sleep(300 * time.Millisecond) // Let "Calibrating" start before twiddle
					
					// Debug: save box visualization (optional)
					if d.debug {
						// TODO: Save debug images
					}
					
					// Run twiddle refinement
					shot2, err := capture.CaptureScreen(0)
					if err != nil {
						log.Printf("[ERROR] Screen capture for twiddle failed: %v", err)
					} else {
						newBox, newDist, newMargin := d.twiddle(shot2, cardBox)
						shot2.Close()
						
						cardBox = newBox
						calibrated = true
						
						// Save calibration for future sessions
						box := capture.Box{
							X: cardBox.Min.X,
							Y: cardBox.Min.Y,
							W: cardBox.Dx(),
							H: cardBox.Dy(),
						}
						capture.SaveCalibration(d.dataDir, W, H, box)
						
						// Re-identify with refined box
						shot3, err := capture.CaptureScreen(0)
						if err == nil {
							region := shot3.Region(cardBox)
							crop := gocv.NewMat()
							region.CopyTo(&crop)
							region.Close()
							
							cropGray := gocv.NewMat()
							gocv.CvtColor(crop, &cropGray, gocv.ColorBGRToGray)
							crop.Close()
							
							hit2 := d.idx.Identify(cropGray, true, 280, 20, nil, false)
							cropGray.Close()
							shot3.Close()
							
							if hit2 != nil {
								name = hit2.Meta.Name
								dist = newDist
								margin = newMargin
							}
						}
					}
				}
				
				noCardCount = 0
				if name != lastName {
					lastName = name
					text := describe(hit.Meta)
					d.setStatus(fmt.Sprintf("🃏 %s  (d=%d m=%d)", name, dist, margin))
					d.speaker.Speak(text)
					
					// Debug: save crop when card changes
					if d.debug && !debugCrop.Empty() {
						safeName := strings.ReplaceAll(name, " ", "_")
						safeName = strings.ReplaceAll(safeName, "/", "_")
						safeName = strings.ReplaceAll(safeName, ",", "")
						safeName = strings.ReplaceAll(safeName, "'", "")
						
						ocrFlag := ""
						if hit.Meta.OCRFallback {
							ocrFlag = "_OCR"
						}
						
						debugPath := fmt.Sprintf("/tmp/cardsense_match_%d_%s_d%d_m%d%s.png", 
							frameNum, safeName, dist, margin, ocrFlag)
						gocv.IMWrite(debugPath, debugCrop)
						
						if hit.Meta.OCRFallback {
							log.Printf("[DEBUG] OCR Match: %s from '%s' → %s", 
								name, hit.Meta.OCRText, debugPath)
						} else {
							log.Printf("[DEBUG] pHash Match: %s d=%d m=%d → %s", 
								name, dist, margin, debugPath)
						}
					}
				}
			} else {
				// Debug: save non-matches (only first 3 per card session)
				if d.debug && !debugCrop.Empty() && noCardCount < 3 {
					debugPath := fmt.Sprintf("/tmp/cardsense_nomatch_%d.png", frameNum)
					gocv.IMWrite(debugPath, debugCrop)
					log.Printf("[DEBUG] Match failure: saved crop to %s", debugPath)
				}
				
				noCardCount++
				if noCardCount >= NO_CARD_FRAMES && lastName != "" {
					lastName = ""
					d.setStatus("👀 Watching... right-click a card to identify")
				}
			}
			
			// Always close debug crop if we have one
			if d.debug && !debugCrop.Empty() {
				debugCrop.Close()
			}
		}
	}
	
	d.setStatus("⏹️ Stopped")
}

// twiddle refines the crop box to minimize pHash distance (port of Python's _twiddle)
func (d *Detector) twiddle(shot gocv.Mat, box image.Rectangle) (image.Rectangle, int, int) {
	x := box.Min.X
	y := box.Min.Y
	w := box.Dx()
	h := box.Dy()
	
	H_max := shot.Rows()
	W_max := shot.Cols()
	
	// Score function: computes min hamming distance for a box
	score := func(bx, by, bw, bh int) int {
		if bx < 0 || by < 0 || bw < 20 || bh < 20 {
			return 9999
		}
		if bx+bw > W_max || by+bh > H_max {
			return 9999
		}
		
		rect := image.Rect(bx, by, bx+bw, by+bh)
		region := shot.Region(rect)
		crop := gocv.NewMat()
		region.CopyTo(&crop)
		region.Close()
		
		cropGray := gocv.NewMat()
		gocv.CvtColor(crop, &cropGray, gocv.ColorBGRToGray)
		crop.Close()
		
		hashes := hash.DualPHash(cropGray, nil)
		cropGray.Close()
		
		distances := hash.HammingScan(hashes, d.idx.Bits)
		minDist := distances[0]
		for _, dist := range distances[1:] {
			if dist < minDist {
				minDist = dist
			}
		}
		return minDist
	}
	
	bestScore := score(x, y, w, h)
	
	// Two-pass hill climbing: 1% then 0.1%
	for _, pct := range []float64{0.01, 0.001} {
		improved := true
		rounds := 0
		
		for improved && rounds < 50 {
			improved = false
			rounds++
			
			// Try adjusting each dimension (x, y, w, h)
			for dim := 0; dim < 4; dim++ {
				for _, sign := range []int{+1, -1} {
					trial := []int{x, y, w, h}
					step := int(float64(trial[dim]) * pct)
					if step < 1 {
						step = 1
					}
					trial[dim] += sign * step
					
					s := score(trial[0], trial[1], trial[2], trial[3])
					if s <= bestScore {
						x, y, w, h = trial[0], trial[1], trial[2], trial[3]
						bestScore = s
						improved = true
					}
				}
			}
		}
	}
	
	log.Printf("[TWIDDLE] done box=(%d,%d,%d,%d) dist=%d", x, y, w, h, bestScore)
	
	// Get margin for the final box
	rect := image.Rect(x, y, x+w, y+h)
	region := shot.Region(rect)
	crop := gocv.NewMat()
	region.CopyTo(&crop)
	region.Close()
	
	cropGray := gocv.NewMat()
	gocv.CvtColor(crop, &cropGray, gocv.ColorBGRToGray)
	crop.Close()
	
	// Get hit with no distance/margin thresholds to compute margin
	hit := d.idx.Identify(cropGray, true, 9999, 0, nil, false)
	cropGray.Close()
	
	margin := 0
	if hit != nil {
		margin = hit.Margin
	}
	
	return rect, bestScore, margin
}

// describe formats card metadata for TTS (port of Python's describe function)
func describe(meta hash.CardMeta) string {
	parts := []string{meta.Name}
	if meta.TypeLine != "" {
		parts = append(parts, meta.TypeLine)
	}
	if meta.OracleText != "" {
		parts = append(parts, meta.OracleText)
	}
	
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ". "
		}
		result += part
	}
	return result
}
