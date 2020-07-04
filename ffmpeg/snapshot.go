package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"os/exec"
	"strings"
	"time"
)

// snapshot returns an image by grapping a frame of the video stream.
func snapshot(width uint, height uint, inputDevice string, inputFilename string) (*image.Image, error) {

	// we check if the snapshot is already in cache to speed up multiple requests
	// generally during button press

	// context to kill the process if not complete in time
	ctx, cancel := context.WithTimeout(context.Background(), 3000*time.Millisecond)
	defer cancel()

	// height "-2" keeps the aspect ratio
	arg := fmt.Sprintf("-hide_banner -f %s -framerate 30 -i %s -vf scale=%d:-2 -frames:v 1 -f mjpeg pipe:1", inputDevice, inputFilename, width)
	args := strings.Split(arg, " ")

	jg, err := exec.CommandContext(ctx, "ffmpeg", args[:]...).Output()
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(jg))

	if err != nil {
		return nil, err
	}

	return &img, nil
}
