package hkdoorbell

import (
	"bufio"
	"time"

	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/log"

	"github.com/ra1nb0w/hkdoorbell/rpi"
)

type Button struct {
	buttonExit       bool
	gpio             int
	switchButton     *characteristic.ProgrammableSwitchEvent
	stdinScanner     *bufio.Scanner
	runButtonPressed func()
}

func InitButton(gpio int, switchButton *characteristic.ProgrammableSwitchEvent, scanner *bufio.Scanner, runBut func()) *Button {
	return &Button{
		buttonExit:       false,
		gpio:             gpio,
		switchButton:     switchButton,
		stdinScanner:     scanner,
		runButtonPressed: runBut,
	}
}

func (b *Button) StartLinux() {
	p, err := rpi.OpenPin(b.gpio, rpi.IN)
	if err != nil {
		panic(err)
	}
	defer p.Close()

	c := 0

	for {
		if b.buttonExit {
			return
		}

		// we have an external pull-up
		// we avoid bouncing
		v, _ := p.Read()
		if v == 0 {
			c = 2
		}

		if c > 0 {
			c--
			if c == 0 && v == 1 {
				log.Debug.Println(">>> Someone pressed the doorbell button <<<")
				go b.runButtonPressed()
				b.switchButton.SetValue(1)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// to activate the button on macOS just write something
// on terminal and press enter
func (b *Button) StartMacOS() {
	for {
		if b.buttonExit {
			return
		}

		b.stdinScanner.Scan()
		// Holds the string that scanned
		if len(b.stdinScanner.Text()) != 0 {
			log.Debug.Println(">>> Someone pressed the doorbell button <<<")
			go b.runButtonPressed()
			b.switchButton.SetValue(1)
		}
	}
}

func (b *Button) Stop() {
	b.buttonExit = false
}
