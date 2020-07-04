package main

import (
	"bufio"
	"flag"
	"image"
	"os"
	"runtime"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/log"

	"github.com/ra1nb0w/hkdoorbell"
	"github.com/ra1nb0w/hkdoorbell/backend"
	"github.com/ra1nb0w/hkdoorbell/ffmpeg"

	"net/http"
	_ "net/http/pprof"
)

func main() {

	// Platform dependent flags
	var videoDevice *string
	var videoFilename *string
	var audioDevice *string
	var audioNameInput *string
	var audioNameOutput *string
	var h264Encoder *string
	var h264Decoder *string
	var buttonGPIO *int

	// Command line arguments
	if runtime.GOOS == "linux" {
		videoDevice = flag.String("video_device", "v4l2", "video input device")
		videoFilename = flag.String("vide_filename", "/dev/video0", "video input device filename")
		audioDevice = flag.String("audio_device", "alsa", "audio input/output device")
		audioNameInput = flag.String("audio_name_input", "default", "audio input name device")
		audioNameOutput = flag.String("audio_name_output", "default", "audio output name device")
		h264Decoder = flag.String("h264_decoder", "", "h264 video decoder")
		h264Encoder = flag.String("h264_encoder", "h264_omx", "h264 video encoder")
		buttonGPIO = flag.Int("button_gpio", 17, "GPIO number connected to the button")
	} else if runtime.GOOS == "darwin" { // macOS
		videoDevice = flag.String("input_device", "avfoundation", "video input device")
		videoFilename = flag.String("input_filename", "default", "video input device filename")
		audioDevice = flag.String("audio_device", "avfoundation", "audio input/output device")
		audioNameInput = flag.String("audio_name_input", "default", "audio input name device")
		audioNameOutput = flag.String("audio_name_output", "default", "audio output name device")
		h264Decoder = flag.String("h264_decoder", "", "h264 video decoder")
		h264Encoder = flag.String("h264_encoder", "libx264", "h264 video encoder")
		buttonGPIO = new(int)
	} else {
		log.Info.Fatalf("%s platform is not supported", runtime.GOOS)
	}
	var minVideoBitrate *int = flag.Int("min_video_bitrate", 0, "minimum video bit rate in kbps")
	var verbose *bool = flag.Bool("verbose", true, "Verbose logging")
	var dataDir *string = flag.String("data_dir", "Doorbell", "Path to data directory")
	var pin *string = flag.String("pin", "00102003", "Pin used to associate the accesory to Homekit")
	var profile *bool = flag.Bool("profile", false, "Enable http pprof")
	var profile_addr *string = flag.String("profile_addr", "localhost:8383", "pprof address:port")
	var backend_addr *string = flag.String("backend_addr", "0.0.0.0:8080", "address:port of the backend web service")

	flag.Parse()

	if *verbose {
		log.Debug.Enable()
		ffmpeg.EnableVerboseLogging()
	}

	accInfo := accessory.Info{
		Name:             "Doorbell",
		FirmwareRevision: "1.0",
		SerialNumber:     "l33t",
		Manufacturer:     "Davide Gerhard",
		Model:            "PiDoorBell",
	}
	doorbell := hkdoorbell.NewDoorbell(accInfo)

	cfg := ffmpeg.Config{
		VideoDevice:     *videoDevice,
		VideoFilename:   *videoFilename,
		AudioDevice:     *audioDevice,
		AudioNameInput:  *audioNameInput,
		AudioNameOutput: *audioNameOutput,
		H264Decoder:     *h264Decoder,
		H264Encoder:     *h264Encoder,
		MinVideoBitrate: *minVideoBitrate,
	}

	ffmpeg := hkdoorbell.SetupFFMPEGStreaming(doorbell, cfg)

	// configure homekit
	config := hc.Config{Pin: *pin, StoragePath: *dataDir}

	t, err := hc.NewIPTransport(config, doorbell.Accessory)
	if err != nil {
		log.Info.Panic(err)
	}

	// start backend http web server
	db_file := *dataDir + "/history.sqlite"
	bk := backend.InitBackend(db_file, *backend_addr)
	go bk.StartWebService()

	// save a snapshot when the button is pressed
	onButtonPressed := func() {
		// this is the size used by preview on IOS
		// we hope that it doesn't change :)
		img, err := ffmpeg.Snapshot(1280, 960)

		if img != nil && err == nil {
			bk.InsertSnapshot(img)
		}
	}

	// instantiate and start the button used to activate the doorbell notification
	b := hkdoorbell.InitButton(
		*buttonGPIO,
		doorbell.Control.ProgrammableSwitchEvent,
		bufio.NewScanner(os.Stdin),
		onButtonPressed)

	// start the button
	if runtime.GOOS == "linux" {
		go b.StartLinux()
	} else if runtime.GOOS == "darwin" {
		go b.StartMacOS()
	} else {
		log.Info.Fatalf("%s platform doesn't support button", runtime.GOOS)
	}

	// enable pprof
	if *profile {
		log.Debug.Println("Start pprof at " + *profile_addr)
		go http.ListenAndServe(*profile_addr, nil)
	}

	// enable snapshot callback
	t.CameraSnapshotReq = func(width, height uint) (*image.Image, error) {
		return ffmpeg.Snapshot(width, height)
	}

	// close all connection when exit
	hc.OnTermination(func() {
		bk.StopWebService()
		b.Stop()
		<-t.Stop()
	})

	// start the homekit backend
	t.Start()
}
