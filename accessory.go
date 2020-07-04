package hkdoorbell

import (
	"github.com/brutella/hc/service"
	"github.com/brutella/hc/accessory"
)

// Doorbell provides RTP video streaming, Speaker and Mic controls
type Doorbell struct {
	*accessory.Accessory
	Control           *service.Doorbell
	StreamManagement1 *service.CameraRTPStreamManagement
	Speaker 	  *service.Speaker
	Microphone	  *service.Microphone
}

// NewDoorbell returns a Video Doorbell accessory.
func NewDoorbell(info accessory.Info) *Doorbell {
	acc := Doorbell{}
	acc.Accessory = accessory.New(info, accessory.TypeVideoDoorbell)
	acc.Control = service.NewDoorbell()
	acc.AddService(acc.Control.Service)

	acc.StreamManagement1 = service.NewCameraRTPStreamManagement()
	acc.AddService(acc.StreamManagement1.Service)

	acc.Speaker = service.NewSpeaker()
	acc.AddService(acc.Speaker.Service)

	acc.Microphone = service.NewMicrophone()
	acc.AddService(acc.Microphone.Service)

	return &acc
}
