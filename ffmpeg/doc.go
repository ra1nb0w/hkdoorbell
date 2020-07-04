// Package ffmpeg lets you access the camera via ffmpeg to stream video and to create snapshots.
//
// This package requires the `ffmpeg` command line tool to be installed. Install by running
// - use https://github.com/homebridge/ffmpeg-for-homebridge on linux
// - `sudo port install ffmpeg +nonfree` on macOS
//
// HomeKit supports multiple video codecs but h264 is mandatory. So make sure that a h264 decoder for ffmpeg is installed too.
package ffmpeg
