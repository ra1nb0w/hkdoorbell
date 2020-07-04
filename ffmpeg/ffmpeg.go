package ffmpeg

import (
	"fmt"
	"image"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/brutella/hc/log"
	"github.com/brutella/hc/rtp"

	"github.com/patrickmn/go-cache"
)

// StreamID is the type of the stream identifier
type StreamID string

// FFMPEG lets you interact with video stream.
type FFMPEG interface {
	PrepareNewStream(rtp.SetupEndpoints, rtp.SetupEndpointsResponse) StreamID
	Start(StreamID, rtp.VideoParameters, rtp.AudioParameters) error
	Stop(StreamID)
	Suspend(StreamID)
	Resume(StreamID)
	ActiveStreams() int
	Reconfigure(StreamID, rtp.VideoParameters, rtp.AudioParameters) error
	Snapshot(width, height uint) (*image.Image, error)
}

var Stdout = ioutil.Discard
var Stderr = ioutil.Discard

// EnableVerboseLogging enables verbose logging of ffmpeg to stdout.
func EnableVerboseLogging() {
	Stdout = os.Stdout
	Stderr = os.Stderr
}

type ffmpeg struct {
	cfg        Config
	mutex      *sync.Mutex
	streams    map[StreamID]*stream
	rtpProxies map[StreamID]*rtpProxy
	snapCache  *cache.Cache
}

// New returns a new ffmpeg handle to start and stop video streams and to make snapshots.
func New(cfg Config) *ffmpeg {
	return &ffmpeg{
		cfg:        cfg,
		mutex:      &sync.Mutex{},
		streams:    make(map[StreamID]*stream, 0),
		rtpProxies: make(map[StreamID]*rtpProxy, 0),
		// how many milliseconds that the snapshot will be cached
		snapCache: cache.New(10000*time.Millisecond, 10000*time.Millisecond),
	}
}

func (f *ffmpeg) PrepareNewStream(req rtp.SetupEndpoints, resp rtp.SetupEndpointsResponse) StreamID {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// we need to generate to generate two random ports for the proxy
	// we choose to statically use from 3000 to 5000
	// TODO check if they are different for every other stream
	rtpp1 := uint16(rand.Intn(1000) + 3000)
	rtpp2 := uint16(rand.Intn(1000) + 4000)

	id := StreamID(req.SessionId)
	s := &stream{f.videoInputDevice(), f.videoInputFilename(), f.audioDevice(), f.audioInputName(), f.audioOutputName(),
		f.cfg.H264Decoder, f.cfg.H264Encoder, f.cfg.MinVideoBitrate, req, resp, nil, nil, rtpp1, rtpp2}
	f.streams[id] = s

	c := &rtpProxy{false, req.ControllerAddr.IPAddr, req.ControllerAddr.AudioRtpPort, rtpp1, rtpp2}
	f.rtpProxies[id] = c

	return id
}

func (f *ffmpeg) ActiveStreams() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return len(f.streams)
}

func (f *ffmpeg) Start(id StreamID, video rtp.VideoParameters, audio rtp.AudioParameters) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	s, err := f.getStream(id)
	if err != nil {
		log.Info.Println("start:", err)
		return err
	}

	c, err := f.getRtpProxy(id)
	if err != nil {
		log.Info.Println("start:", err)
		return err
	}

	// we use goroutine to run the threaded RTP proxy
	go c.start()

	// run the stream
	return s.start(video, audio)
}

func (f *ffmpeg) Stop(id StreamID) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	s, err := f.getStream(id)
	if err != nil {
		log.Info.Println("stop:", err)
		return
	}

	c, err := f.getRtpProxy(id)
	if err != nil {
		log.Info.Println("stop:", err)
		return
	}

	c.stop()
	delete(f.rtpProxies, id)
	s.stop()
	delete(f.streams, id)
}

func (f *ffmpeg) Suspend(id StreamID) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if s, err := f.getStream(id); err != nil {
		log.Info.Println("suspend:", err)
	} else {
		s.suspend()
	}
}

func (f *ffmpeg) Resume(id StreamID) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if s, err := f.getStream(id); err != nil {
		log.Info.Println("resume:", err)
	} else {
		s.resume()
	}
}

func (f *ffmpeg) Reconfigure(id StreamID, video rtp.VideoParameters, audio rtp.AudioParameters) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	s, err := f.getStream(id)
	if err != nil {
		log.Info.Println("reconfigure:", err)
		return err
	}

	return s.reconfigure(video, audio)
}

func (f *ffmpeg) getStream(id StreamID) (*stream, error) {
	if s, ok := f.streams[id]; ok {
		return s, nil
	}

	return nil, &StreamNotFoundError{id}
}

func (f *ffmpeg) getRtpProxy(id StreamID) (*rtpProxy, error) {
	if s, ok := f.rtpProxies[id]; ok {
		return s, nil
	}

	return nil, &StreamNotFoundError{id}
}

func (f *ffmpeg) Snapshot(width, height uint) (*image.Image, error) {

	img, found := f.snapCache.Get(string(width) + string(height))
	if found {
		log.Info.Println("Return a cached snapshot")
		return img.(*image.Image), nil
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()

	shot, err := snapshot(width, height, f.videoInputDevice(), f.videoInputFilename())

	if shot != nil {
		f.snapCache.Set(string(width)+string(height), shot, cache.DefaultExpiration)
	}

	return shot, err
}

func (f *ffmpeg) videoInputDevice() string {
	return f.cfg.VideoDevice
}

func (f *ffmpeg) videoInputFilename() string {
	return f.cfg.VideoFilename
}

func (f *ffmpeg) audioDevice() string {
	return f.cfg.AudioDevice
}

func (f *ffmpeg) audioInputName() string {
	return f.cfg.AudioNameInput
}

func (f *ffmpeg) audioOutputName() string {
	return f.cfg.AudioNameOutput
}

type StreamNotFoundError struct {
	id StreamID
}

func (e *StreamNotFoundError) Error() string {
	return fmt.Sprintf("StreamID(%v) not found", []byte(e.id))
}
