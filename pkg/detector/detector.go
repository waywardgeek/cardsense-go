package detector

import (
	"fmt"
	"image"
	"log"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gocv.io/x/gocv"

	"github.com/waywardgeek/cardsense-go/pkg/capture"
	"github.com/waywardgeek/cardsense-go/pkg/hash"
	"github.com/waywardgeek/cardsense-go/pkg/tts"
)

const (
	// NO_CARD_FRAMES is the number of consecutive empty frames before resetting lastCard
	NO_CARD_FRAMES = 5
	
	// Frame interval in milliseconds (~5 fps for reduced CPU usage)
	FRAME_INTERVAL = 200 * time.Millisecond

	// MATCH_MAX_DIST is the maximum pHash distance accepted as a real match.
	//
	// Was 280, which admitted outright false positives: a board view at
	// dist=172 was accepted as "Golgari Thug" and would have been spoken.
	// Measured populations on a calibrated box:
	//   real card : dist 48-100  (Badgermole Cub matched at 52)
	//   noise     : dist 170-202
	// 150 sits in the gap.
	MATCH_MAX_DIST = 150

	// MATCH_MIN_MARGIN is the minimum lead over the best differently-named card.
	//
	// Lowered from 20 to 15. A KNOWN-GOOD match (Badgermole Cub) measured
	// dist=52 margin=18: a real card can legitimately have a small margin when
	// similar art exists in the index, so a threshold of 20 would reject it and
	// push it down the OCR path -- exactly the path that invents wrong names.
	// MATCH_MAX_DIST is the discriminator doing the real work, since it already
	// excludes the entire noise band (170+) regardless of margin.
	MATCH_MIN_MARGIN = 15
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

	// recalRequested is set by RequestRecalibrate (GUI thread) and consumed by
	// the detection loop. int32 + atomics because the two run on different
	// goroutines.
	recalRequested int32
}

// RequestRecalibrate asks the detection loop to discard its current calibration
// and start over from the reference box scaled to the current display.
//
// Needed when moving between displays: the saved calibration is specific to the
// resolution it was measured on, and while it is rescaled automatically, an
// explicit reset is the reliable escape hatch if the rescaled box does not
// converge (or if a previous session persisted a bad box).
//
// Safe to call whether or not the detector is running.
func (d *Detector) RequestRecalibrate() {
	atomic.StoreInt32(&d.recalRequested, 1)
}

// New creates a new Detector
func New(speaker tts.Speaker, dataDir string, onStatus StatusCallback, debug bool) *Detector {
	// Resolve the data directory ONCE, here, rather than letting each consumer
	// decide what "" means. LoadCardIndex silently substitutes hash.DataDir()
	// for an empty string, but LoadCalibration/SaveCalibration did not -- they
	// built a RELATIVE path ("calibration.json"), so every save failed
	// (os.MkdirAll("") returns an error) and every load missed. The detector
	// therefore re-calibrated from scratch on every launch while logging
	// "Saved calibration", because the error return was discarded.
	if dataDir == "" {
		dataDir = hash.DataDir()
	}
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
			// Honour an explicit recalibration request from the GUI.
			if atomic.SwapInt32(&d.recalRequested, 0) == 1 {
				if err := capture.ClearCalibration(d.dataDir); err != nil {
					log.Printf("[ERROR] Could not clear saved calibration: %v", err)
				}
				def := capture.DefaultBox(W, H)
				cardBox = image.Rect(def.X, def.Y, def.X+def.W, def.Y+def.H)
				calibrated = false
				lastName = "" // so the same card re-announces after recalibrating
				log.Printf("[CALIBRATE] Manual recalibration requested: reset to default box (%d,%d,%d,%d) for %dx%d",
					def.X, def.Y, def.W, def.H, W, H)
				d.setStatus("🎯 Recalibrating - hover a card...")
				d.speaker.Speak("Recalibrating. Hover a card.")
			}

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
			
			// Check for calibration opportunity BEFORE full identification
			// If uncalibrated and there's strong visual evidence of a card
			// trigger calibration even without a confident match
			if !calibrated {
				h := cropGray.Rows()
				w := cropGray.Cols()
				aspect := float64(w) / float64(h)
				
				// Do a quick pHash check to see if this looks card-like
				hashes := hash.DualPHash(cropGray, nil)
				distances := hash.HammingScan(hashes, d.idx.Bits)
				
				// Find min distance AND margin (distance to next different card)
				type match struct {
					idx  int
					dist int
				}
				matches := make([]match, len(distances))
				for i, dist := range distances {
					matches[i] = match{i, dist}
				}
				sort.Slice(matches, func(i, j int) bool {
					return matches[i].dist < matches[j].dist
				})
				
				minDist := matches[0].dist
				topName := d.idx.Names[matches[0].idx]
				
				// Find margin
				margin := 1000
				for i := 1; i < len(matches); i++ {
					if d.idx.Names[matches[i].idx] != topName {
						margin = matches[i].dist - minDist
						break
					}
				}
				
				// Calibrate only on STRONG evidence of a real card.
				//
				// Measured populations on a correctly-shaped box (2026-08-07):
				//   real card : dist 78 pre-twiddle -> 54 post, margin 88
				//   non-card  : dist 190-198,                   margin 0-4
				//
				// The old gate was `minDist < 200`, which accepts literally
				// everything including empty board views, so the detector would
				// calibrate on noise and then persist it -- overwriting a good
				// dist=54 box with a dist=190 one. Since calibration is saved to
				// disk and becomes the scaling source for later runs, a single
				// bad frame poisoned every future session.
				//
				// 150 sits in the empty gap between the two populations.
				const calTriggerDist = 150
				if minDist < calTriggerDist && aspect >= 0.53 && aspect <= 0.85 {
					log.Printf("[CALIBRATE] Visual evidence of card (dist=%d, margin=%d, aspect=%.3f), triggering calibration...", minDist, margin, aspect)
					d.speaker.Speak("Calibrating")
					time.Sleep(300 * time.Millisecond)
					
					shot2, err := capture.CaptureScreen(0)
					if err != nil {
						log.Printf("[ERROR] Screen capture for twiddle failed: %v", err)
					} else {
						newBox, newDist, _ := d.twiddle(shot2, cardBox)
						shot2.Close()
						
						// Only accept + persist a calibration that actually
						// converged onto a card. Otherwise leave the box alone
						// and stay uncalibrated so we can retry on a later frame.
						const calAcceptDist = 100
						if newDist < calAcceptDist {
							cardBox = newBox
							calibrated = true
							
							box := capture.Box{
								X: cardBox.Min.X,
								Y: cardBox.Min.Y,
								W: cardBox.Dx(),
								H: cardBox.Dy(),
							}
							if err := capture.SaveCalibration(d.dataDir, W, H, box); err != nil {
								log.Printf("[ERROR] Could not persist calibration: %v", err)
							} else {
								log.Printf("[CALIBRATE] Saved calibration: dist improved to %d", newDist)
							}
						} else {
							log.Printf("[CALIBRATE] REJECTED: twiddle only reached dist=%d (need <%d) - not saving, will retry", newDist, calAcceptDist)
						}
					}
				}
			}
			
			// Identify card (with OCR fallback enabled).
			//
			// maxDist was 280, which is far looser than the measured data
			// supports and admitted outright false positives: a board view at
			// dist=172/margin=20 was accepted as "Golgari Thug" and would have
			// been SPOKEN. Measured on a calibrated box (2026-08-07):
			//   real card : dist 48-100, margin 72-88
			//   noise     : dist 170-202, margin 0-20
			// MATCH_MAX_DIST sits in the gap. Speaking a wrong card name is the
			// worst failure mode for an accessibility tool -- the user has no
			// way to know it is wrong -- so prefer silence when uncertain.
			hit := d.idx.Identify(cropGray, true, MATCH_MAX_DIST, MATCH_MIN_MARGIN, nil, true)
			
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
							
							hit2 := d.idx.Identify(cropGray, true, MATCH_MAX_DIST, MATCH_MIN_MARGIN, nil, false)
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
	
	// Coarse-to-fine hill climbing. The coarse passes (8%, 3%) let the search
	// escape a bad starting box — e.g. one scaled from a different display —
	// which 1%/0.1% steps alone cannot do, since they get stuck in whatever
	// local minimum the initial box happens to sit in.
	for _, pct := range []float64{0.08, 0.03, 0.01, 0.001} {
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
