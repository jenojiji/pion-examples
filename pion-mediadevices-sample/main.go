package main

import (
	"fmt"
	"image/jpeg"
	"log"
	"os"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/prop"
	_ "github.com/pion/mediadevices/pkg/driver/camera"
)

func main() {
	// ── 1. Enumerate all available media devices ──────────────────────────────
	devices := mediadevices.EnumerateDevices()
	fmt.Printf("=== Found %d device(s) ===\n", len(devices))
	for _, d := range devices {
		fmt.Printf("  [%v] Label: %q  DeviceID: %q\n", d.Kind, d.Label, d.DeviceID)
	}
	if len(devices) == 0 {
		log.Fatal("No devices found — check CGO_ENABLED=1, libv4l-dev, and video group membership")
	}
	fmt.Println()

	// ── 2. Open a video stream from the default camera ────────────────────────

	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(c *mediadevices.MediaTrackConstraints) {
			// Request 640×480; the driver will pick the nearest supported mode.
			c.Width = prop.Int(640)
			c.Height = prop.Int(480)

			// Optional: target a specific device by DeviceID from EnumerateDevices
			// c.DeviceID = prop.String("/dev/video0")
		},
	})
	if err != nil {
		log.Fatalf("GetUserMedia failed: %v\n\n"+
			"Make sure:\n"+
			"  • a V4L2 camera is connected (/dev/video*)\n"+
			"  • your user is in the 'video' group  (sudo usermod -aG video $USER)\n"+
			"  • the uvcvideo kernel module is loaded (modprobe uvcvideo)\n", err)
	}

	// ── 3. Grab the first video track ─────────────────────────────────────────
	videoTracks := stream.GetVideoTracks()
	if len(videoTracks) == 0 {
		log.Fatal("No video tracks found in stream")
	}

	// Cast to *mediadevices.VideoTrack to access the frame reader
	videoTrack, ok := videoTracks[0].(*mediadevices.VideoTrack)
	if !ok {
		log.Fatal("Failed to cast track to *mediadevices.VideoTrack")
	}
	defer videoTrack.Close()

	// ── 4. Create a reader and pull one frame ─────────────────────────────────
	//
	// NewReader(false) → false means "do NOT transform/encode"; give raw frames.
	// Read() returns (image.Image, releaseFunc, error).
	// You MUST call release() when done — it recycles the buffer back to the driver.
	reader := videoTrack.NewReader(false)

	img, release, err := reader.Read()
	if err != nil {
		log.Fatalf("Failed to read frame: %v", err)
	}
	defer release() // always release the frame buffer

	fmt.Printf("Captured frame: %T  bounds=%v\n", img, img.Bounds())

	// ── 5. Encode the frame as JPEG and write to disk ─────────────────────────
	outPath := "capture.jpg"
	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer f.Close()

	opts := &jpeg.Options{Quality: 90}
	if err := jpeg.Encode(f, img, opts); err != nil {
		log.Fatalf("JPEG encode failed: %v", err)
	}

	fmt.Printf("Saved → %s\n", outPath)
}
