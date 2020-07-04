# hkdoorbell - Homekit Doorbell

`hkdoorbell` is an open-source implementation of an HomeKit IP
doorbell.
It uses `ffmpeg` to access the camera stream and publishes the stream to HomeKit using [hc](https://github.com/brutella/hc) by Matthias Hochgatterer.
The doorbell camera stream can be viewed in any HomeKit app.

## Features

- live streaming via HomeKit with bi-directional audio
- works with any HomeKit app
- completely written in Go
- runs on multiple platforms (Linux, macOS)
- in memory cache for snapshots
- backend web service (default at 0.0.0.0:8080) with last 100
  snapshots

## Limitations

- only one person can answer and see the doorbell camera
- Secure video is not supported at the moment
- motion sensor is not supported (useful only if secure video is implemented)

## Get Started

*hkdoorbell uses Go modules and therefore requires Go 1.11 or higher.*

### Mac

The fastest way to get started is to

1. download the project on a Mac with a built-in iSight camera
```sh
git clone https://github.com/ra1nb0w/hkdoorbell && cd hkdooebell
```
2. run with `make run` or `make bin` to get the binary
3. open any HomeKit app and add the doorbell to HomeKit (pin for initial setup is `001 02 003`)
4. if you need to change parameters like pin read the help of the
   binary with `./hkdoorbell`

These steps require *git*, *go* and *ffmpeg* (with libfdk-aac) to be
 installed. On macOS you can install them via [macports](https://www.macports.org/install.php).

```sh
sudo port install git go
sudo port install ffmpeg +nonfree
```

For ffmpeg you can also use pre-compiled binary from [ffmpeg for
homebridge](https://github.com/homebridge/ffmpeg-for-homebridge) but
pay attention that it violates the GPL license.

To simulate the doorbell button you can write anything in the console and
press enter.

### Raspberry Pi

If you want to create your own doorbell, you can run
`hkdoorbell` on a Raspberry Pi with attached camera module and
input/output audio.

To create the binary run `make build-rpi` or to get the package run
`make package-rpi`.

The software requires the following things:
- ffmpeg with *libfdk-aac* and *h264_omx*. You can use a pre-compiled
  binary from [ffmpeg for homebridge](https://github.com/homebridge/ffmpeg-for-homebridge)
  but pay attention that it violates the GPL license.
- enable the camera with `raspi-config`
- input/audio HAT like ReSpeaker 2-Mics Pi HAT correctly configured;
  check with `alsamixer` the parameters and set `/etc/asound.conf`
  accordingly
- one gpio connected to a phisical button with a pull-up resistor and
  a capacitor; as default GPIO 17 is used; can be changed with a
  command line parameter

# Notes

- Compared to a "simple" camera plugin this plugin uses the HomeKit
  video doorbell profile. A "doorbell" notification with snapshot is
  sent to all iCloud connected devices. If the same (HomeKit) room
  containing this camera also has a Lock mechanism accessory, the
  notification will show a working UNLOCK button. HomeKit/iOS will link
  them together automatically when they are in the same room.

# License

`hkdoorbell` is available under the Apache License 2.0 license.
See the LICENSE file for more info.
