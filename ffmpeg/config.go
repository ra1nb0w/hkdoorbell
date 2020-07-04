package ffmpeg

// Config contains ffmpeg parameters
type Config struct {
	VideoDevice      string
	VideoFilename    string
	AudioDevice      string
	AudioNameInput   string
	AudioNameOutput  string
	H264Decoder      string
	H264Encoder      string
	MinVideoBitrate  int
}
