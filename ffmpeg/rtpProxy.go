package ffmpeg

import (
	"fmt"
        "net"
	"github.com/brutella/hc/log"
)

type rtpProxy struct {
	rtpProxyExit      bool
	controllerIPAddr  string
	bindPort          uint16
	localRTPPort1     uint16
	localRTPPort2     uint16
}


func (r *rtpProxy) start() {
        log.Debug.Println(fmt.Sprintf("start rtp proxy: %s:%d - local RTP ports: %d and %d",
		r.controllerIPAddr, r.bindPort, r.localRTPPort1, r.localRTPPort2))

	r.rtpProxyExit = false

	connection, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: int(r.bindPort),
		IP:   net.ParseIP("0.0.0.0"),
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	defer connection.Close()
	buffer := make([]byte, 2048)

	for {
		n, addr, _ := connection.ReadFromUDP(buffer)
		if (addr.String() == fmt.Sprintf("%s:%d", r.controllerIPAddr, r.bindPort)) {
			//log.Debug.Println("From " + addr.String() + "; send to local")
			a, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", r.localRTPPort1))
			_, _ = connection.WriteTo(buffer[0:n], a)
			a, _ = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", r.localRTPPort2))
			_, _ = connection.WriteTo(buffer[0:n], a)
		} else {
			//log.Debug.Println("From " + addr.String() + "; send to remote")
			a, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", r.controllerIPAddr, r.bindPort))
			_, _ = connection.WriteTo(buffer[0:n], a)
		}

		if (r.rtpProxyExit) {
			return
		}
	}
}


func (r *rtpProxy) stop() {
	r.rtpProxyExit = true
	log.Debug.Println("stop rt proxy")
}
