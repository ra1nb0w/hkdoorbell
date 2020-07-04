package ffmpeg

import (
	"fmt"
	"github.com/brutella/hc/log"
	"github.com/brutella/hc/rtp"
	"os/exec"
	"strings"
	"runtime"
	"syscall"
	"io"
)

type stream struct {
	videoDevice     string
	videoFilename   string
	audioDevice     string
	audioInputName  string
	audioOutputName string
	h264Decoder     string
	h264Encoder     string
	minVideoBitrate int

	req             rtp.SetupEndpoints
	resp            rtp.SetupEndpointsResponse

	cmd             *exec.Cmd
	cmd2            *exec.Cmd

	rtpProxyPort1   uint16
	rtpProxyPort2   uint16
}

func (s *stream) isActive() bool {
	return s.cmd != nil
}

func (s *stream) stop() {
	log.Debug.Println("stop stream")

	if s.cmd != nil {
		s.cmd.Process.Signal(syscall.SIGINT)
		// avoid zombie (SIGCHLD)
		s.cmd.Process.Wait()
		s.cmd = nil
	}

        if s.cmd2 != nil {
                s.cmd2.Process.Signal(syscall.SIGINT)
		// avoid zombie (SIGCHLD)
		s.cmd2.Process.Wait()
                s.cmd2 = nil
        }
}

func (s *stream) start(video rtp.VideoParameters, audio rtp.AudioParameters) error {
	log.Debug.Println("start stream")

	ffmpegVideo := "-hide_banner" +
		fmt.Sprintf(" -f %s", s.videoDevice) +
		fmt.Sprintf(" -framerate %d", s.framerate(video.Attributes)) +
		fmt.Sprintf("%s", s.videoDecoderOption(video))

	if runtime.GOOS == "linux" {
		ffmpegVideo += fmt.Sprintf(" -i %s", s.videoFilename)
	}  else if runtime.GOOS == "darwin" {
		ffmpegVideo += fmt.Sprintf(" -i %s:%s", s.videoFilename, s.audioInputName)
	}

	ffmpegVideo += " -an" +
		fmt.Sprintf(" -codec:v %s", s.videoEncoder(video))

	if runtime.GOOS == "darwin" {
		ffmpegVideo += " -pix_fmt yuv420p -vsync vfr"
	}

	ffmpegVideo += " -preset ultrafast -tune zerolatency" +
		// height "-2" keeps the aspect ratio
		fmt.Sprintf(" -video_size %d:-2", video.Attributes.Width) +
		fmt.Sprintf(" -framerate %d", video.Attributes.Framerate) +
		fmt.Sprintf(" -level:v %s", videoLevel(video.CodecParams))

	if runtime.GOOS == "linux" {
                ffmpegVideo += fmt.Sprintf(" -profile:v %s", videoProfile(video.CodecParams))
	}

	ffmpegVideo += " -f copy" +
		fmt.Sprintf(" -b:v %dk", s.videoBitrate(video)) +
		fmt.Sprintf(" -payload_type %d", video.RTP.PayloadType) +
		fmt.Sprintf(" -ssrc %d", s.resp.SsrcVideo) +
		" -f rtp -srtp_out_suite AES_CM_128_HMAC_SHA1_80" +
		fmt.Sprintf(" -srtp_out_params %s", s.req.Video.SrtpKey()) +
		fmt.Sprintf(" srtp://%s:%d?rtcpport=%d&localrtcpport=%d&pkt_size=%s&timeout=60",
			s.req.ControllerAddr.IPAddr,
			s.req.ControllerAddr.VideoRtpPort,
			s.req.ControllerAddr.VideoRtpPort,
			s.req.ControllerAddr.VideoRtpPort,
			videoMTU(s.req))

	ffmpegAudio := " -fflags nobuffer" +
		" -flags low_delay -probesize 32 -analyzeduration 0 "
	if runtime.GOOS == "linux" {
		ffmpegAudio += fmt.Sprintf("-f %s -i %s -vn", s.audioDevice, s.audioInputName)
	}  else if runtime.GOOS == "darwin" {
		ffmpegAudio += "-vn"
	}

	ffmpegAudio += fmt.Sprintf(" %s", audioCodecOption(audio)) +
		" -flags +global_header" +
		fmt.Sprintf(" -ar %s", audioSamplingRate(audio)) +
		fmt.Sprintf(" -b:a %dk -bufsize 48k", audio.RTP.Bitrate) +
		" -ac 1" +
		fmt.Sprintf(" -payload_type %d", audio.RTP.PayloadType) +
		fmt.Sprintf(" -ssrc %d", s.resp.SsrcAudio) +
		" -f rtp -srtp_out_suite AES_CM_128_HMAC_SHA1_80" +
		fmt.Sprintf(" -srtp_out_params %s", s.req.Audio.SrtpKey()) +
		fmt.Sprintf(" srtp://127.0.0.1:%d?rtcpport=%d&localrtcpport=%d&pkt_size=%s&timeout=60",
			s.req.ControllerAddr.AudioRtpPort,
			s.req.ControllerAddr.AudioRtpPort,
			s.rtpProxyPort1,
			audioMTU())

	args := strings.Split(ffmpegVideo + ffmpegAudio, " ")
	cmd := exec.Command("ffmpeg", args[:]...)
	cmd.Stdout = Stdout
	cmd.Stderr = Stderr

	// TODO manage different codec
	ffmpegAudio2SDP := "v=0\n" +
		"o=- 0 0 IN IP4 127.0.0.1\n" +
		"s=No Name\n" +
		"c=IN IP4 127.0.0.1\n" +
		"t=0 0\n" +
		"a=tool:libavformat 58.29.100\n" +
		fmt.Sprintf("m=audio %d RTP/AVP 110\n", s.rtpProxyPort2) +
		fmt.Sprintf("b=AS:%d\n", audio.RTP.Bitrate) +
		"a=rtpmap:110 MPEG4-GENERIC/16000/1\n" +
		"a=fmtp:110 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3; config=F8F0212C00BC00\n" +
		fmt.Sprintf("a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:%s", s.req.Audio.SrtpKey())

	// second ffmpeg instance to manage audio from IOS
	ffmpegAudio2 := "-hide_banner" +
	" -fflags nobuffer -flags low_delay -probesize 32 -analyzeduration 0" +
	" -protocol_whitelist rtp,srtp,crypto,file,udp,pipe -f sdp" +
	" -vn" +
	fmt.Sprintf(" %s", audioCodecOption(audio)) +
	" -flags +global_header" +
	" -i pipe:"

	cmd2_exec := "ffmpeg"
	if runtime.GOOS == "linux" {
		ffmpegAudio2 += fmt.Sprintf(" -f %s %s -async %s", s.audioDevice, s.audioOutputName, audioSamplingRate(audio))
	} else if runtime.GOOS == "darwin" {
		ffmpegAudio2 += " -nodisp -sync ext"
		// 04/07/2020 we need to use ffplay on macOS
		// since AudioToolbox output is only in trunk
		cmd2_exec = "ffplay"
	}

	args2 := strings.Split(ffmpegAudio2, " ")
	cmd2 := exec.Command(cmd2_exec, args2[:]...)
	cmd2.Stdout = Stdout
	cmd2.Stderr = Stderr

	// pipe the SDP header
	stdin2, stdin2_err := cmd2.StdinPipe()
	if stdin2_err != nil {
		log.Debug.Println(stdin2_err)
	}
	defer stdin2.Close()
	io.WriteString(stdin2, ffmpegAudio2SDP)

	log.Debug.Println(cmd)
	log.Debug.Println(cmd2)
	log.Debug.Println(ffmpegAudio2SDP)

	err := cmd.Start()
	err2:= cmd2.Start()
	if err == nil {
		s.cmd = cmd
	}

	if err2 == nil {
		s.cmd2 = cmd2
	}

	return err
}

// TODO (mah) test
func (s *stream) suspend() {
	log.Debug.Println("suspend stream")
	s.cmd.Process.Signal(syscall.SIGSTOP)
	s.cmd2.Process.Signal(syscall.SIGSTOP)
}

// TODO (mah) test
func (s *stream) resume() {
	log.Debug.Println("resume stream")
	s.cmd.Process.Signal(syscall.SIGCONT)
	s.cmd2.Process.Signal(syscall.SIGCONT)
}

// TODO (mah) implement
func (s *stream) reconfigure(video rtp.VideoParameters, audio rtp.AudioParameters) error {
	if s.cmd != nil {
		log.Debug.Println("reconfigure() is not implemented")
	}

	return nil
}

func (s *stream) videoEncoder(param rtp.VideoParameters) string {
	switch param.CodecType {
	case rtp.VideoCodecType_H264:
		return s.h264Encoder
	}

	return "?"
}

func (s *stream) videoDecoderOption(param rtp.VideoParameters) string {
	switch param.CodecType {
	case rtp.VideoCodecType_H264:
		if s.h264Decoder != "" {
			return fmt.Sprintf(" -codec:v %s", s.h264Decoder)
		}
	}

	return ""
}

func (s *stream) videoBitrate(param rtp.VideoParameters) int {
	br := int(param.RTP.Bitrate)
	if s.minVideoBitrate > br {
		br = s.minVideoBitrate
	}

	return br
}

// https://superuser.com/a/564007
func videoProfile(param rtp.VideoCodecParameters) string {
	for _, p := range param.Profiles {
		switch p.Id {
		case rtp.VideoCodecProfileConstrainedBaseline:
			return "baseline"
		case rtp.VideoCodecProfileMain:
			return "main"
		case rtp.VideoCodecProfileHigh:
			return "high"
		default:
			break
		}
	}

	return ""
}

func (s *stream) framerate(attr rtp.VideoCodecAttributes) byte {
	if s.videoDevice == "avfoundation" {
		// avfoundation only supports 30 fps on a
		// MacBook Pro (Retina, 15-inch, Late 2013) running macOS 10.12 Sierra
		return 30
	}

	return attr.Framerate
}

// https://superuser.com/a/564007
func videoLevel(param rtp.VideoCodecParameters) string {
	for _, l := range param.Levels {
		switch l.Level {
		case rtp.VideoCodecLevel3_1:
			return "3.1"
		case rtp.VideoCodecLevel3_2:
			return "3.2"
		case rtp.VideoCodecLevel4:
			return "4.0"
		default:
			break
		}
	}

	return ""
}

func videoMTU(setup rtp.SetupEndpoints) string {
	switch setup.ControllerAddr.IPVersion {
	case rtp.IPAddrVersionv4:
		return "1378"
	case rtp.IPAddrVersionv6:
		return "1228"
	}

	return "1378"
}

func audioMTU() string {
	return "188"
}

// https://trac.ffmpeg.org/wiki/audio%20types
func audioCodecOption(param rtp.AudioParameters) string {
	switch param.CodecType {
	case rtp.AudioCodecType_PCMU:
		log.Debug.Println("audioCodec(PCMU) not supported")
	case rtp.AudioCodecType_PCMA:
		log.Debug.Println("audioCodec(PCMA) not supported")
	case rtp.AudioCodecType_AAC_ELD:
		// requires ffmpeg built with --enable-libfdk-aac
		return "-acodec libfdk_aac -aprofile aac_eld"
	case rtp.AudioCodecType_Opus:
		// bad quality
		return fmt.Sprintf("-codec:a libopus")
	case rtp.AudioCodecType_MSBC:
		log.Debug.Println("audioCodec(MSBC) not supported")
	case rtp.AudioCodecType_AMR:
		log.Debug.Println("audioCodec(AMR) not supported")
	case rtp.AudioCodecType_ARM_WB:
		log.Debug.Println("audioCodec(ARM_WB) not supported")
	}

	return ""
}

func audioVariableBitrate(param rtp.AudioParameters) string {
	switch param.CodecParams.Bitrate {
	case rtp.AudioCodecBitrateVariable:
		return "on"
	case rtp.AudioCodecBitrateConstant:
		return "off"
	default:
		log.Info.Println("variableBitrate() undefined bitrate", param.CodecParams.Bitrate)
		break
	}

	return "?"
}

func audioSamplingRate(param rtp.AudioParameters) string {
	switch param.CodecParams.Samplerate {
	case rtp.AudioCodecSampleRate8Khz:
		return "8k"
	case rtp.AudioCodecSampleRate16Khz:
		return "16k"
	case rtp.AudioCodecSampleRate24Khz:
		return "24k"
	default:
		log.Info.Println("audioSamplingRate() undefined samplrate", param.CodecParams.Samplerate)
		break
	}

	return ""
}
