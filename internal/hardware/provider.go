package hardware

import (
	"encoding/binary"
	"fmt"
	"github.com/fpawel/guartutils/comport"
	"github.com/fpawel/guartutils/fetch"
	"github.com/fpawel/guartutils/modbus"
	"github.com/pkg/errors"
	"github.com/tarm/serial"
	"math"
	"time"
)

type Peer interface {
	HardwareConnected()
	HardwareDisconnected()
	HardwareConnectionError(string)
	HardwareReading(s Reading)
	HardwareConfig(config Config)
	HardwareCurrentPlace(int)
	ComPorts([]string)
}

type Reading struct {
	Pin    int
	Status uint16
	Value  float32
	Error  error
}

type Provider struct {
	peer                           Peer
	chStart, chStop, chComportDone chan struct{}
	comports,
	interrupt, done chan bool
	setPinChecked                chan pinChecked
	setPortName                  chan string
	chGetPinChecked              chan pinCheckedR
	chChanInterruptComport       chan chan struct{}
	chChanCurrentWorkInterrupted chan chan bool
}

type pinChecked struct {
	checked bool
	pin     int
}

type pinCheckedR struct {
	ch  chan checkedR
	pin int
}

type checkedR struct {
	checked    bool
	hasChecked bool
}

func NewProvider(peer Peer, configFilename string) Provider {

	x := Provider{
		peer:                         peer,
		chStart:                      make(chan struct{}),
		chStop:                       make(chan struct{}),
		chComportDone:                make(chan struct{}),
		comports:                     make(chan bool),
		setPinChecked:                make(chan pinChecked),
		setPortName:                  make(chan string),
		done:                         make(chan bool),
		interrupt:                    make(chan bool, 3),
		chGetPinChecked:              make(chan pinCheckedR),
		chChanInterruptComport:       make(chan chan struct{}),
		chChanCurrentWorkInterrupted: make(chan chan bool),
	}
	go x.run(configFilename)
	go comport.NotifyAvailablePortsChange(x.comports)
	return x
}

func (x Provider) getPinChecked(pin int) checkedR {
	ch := make(chan checkedR)
	x.chGetPinChecked <- pinCheckedR{ch, pin}
	return <-ch
}

func (x Provider) runComPort(serialPortName string) {
	defer func() {
		x.chComportDone <- struct{}{}
	}()

	port := comport.NewPort(comport.Config{
		Serial: serial.Config{
			ReadTimeout: time.Millisecond,
			Baud:        9600,
			Name:        serialPortName,
		},
		Fetch: fetch.Config{
			MaxAttemptsRead: 1,
			ReadTimeout:     time.Second,
			ReadByteTimeout: 50 * time.Millisecond,
		},
	})

	if err := port.Open(); err != nil {
		x.peer.HardwareConnectionError(err.Error())
		return
	}
	x.peer.HardwareConnected()

	defer func() {
		x.peer.HardwareDisconnected()
		if err := port.Close(); err != nil {
			x.peer.HardwareConnectionError(err.Error())
		}
	}()
	for {
		for pin := 0; pin < 10; pin++ {
			c := x.getPinChecked(pin)
			if !c.hasChecked {
				x.peer.HardwareConnectionError("не выбраны места")
				return
			}
			if !c.checked {
				continue
			}
			x.peer.HardwareCurrentPlace(pin)
			reading := x.readPin(port, pin)
			x.peer.HardwareCurrentPlace(-1)
			x.peer.HardwareReading(reading)
			err := reading.Error
			if fetch.ConnectionFailed(err) {
				x.peer.HardwareConnectionError(err.Error())
				return
			}
			if fetch.Canceled(err) || x.CurrentWorkInterrupted() {
				return
			}
		}
	}
}

func (x Provider) CurrentWorkInterrupted() bool {
	ch := make(chan bool)
	x.chChanCurrentWorkInterrupted <- ch
	return <-ch
}

func (x Provider) SubscribeInterrupted(ch chan struct{}, subscribe bool) {
	if subscribe {
		x.chChanInterruptComport <- ch
	} else {
		x.chChanInterruptComport <- nil
	}

}

func (x Provider) Close() error {
	x.interrupt <- true
	<-x.done
	return nil
}

func (x Provider) Start() {
	x.chStart <- struct{}{}
}

func (x Provider) Stop() {
	x.chStop <- struct{}{}
}

func (x Provider) SetChecked(pin int, checked bool) {
	go func() {
		x.setPinChecked <- pinChecked{pin: pin, checked: checked}
	}()

}

func (x Provider) SetPortName(portName string) {
	go func() {
		x.setPortName <- portName
	}()
}

func (x Provider) run(configFilename string) {

	cfg := loadConfig(configFilename)
	x.peer.HardwareConfig(cfg)
	started := false
	currentWorkInterrupted := false
	waitComport := false
	defer func() {
		x.done <- true
	}()

	var interruptComport chan struct{}

gotoSelect:
	for {

		select {

		case ch := <-x.chChanCurrentWorkInterrupted:
			ch <- currentWorkInterrupted

		case c := <-x.chGetPinChecked:

			c.ch <- checkedR{
				checked:    cfg.CheckedPlaces[c.pin],
				hasChecked: cfg.CheckedPlaceExists(),
			}

		case ch := <-x.chChanInterruptComport:
			interruptComport = ch

		case <-x.interrupt:
			if started {
				if interruptComport != nil {
					interruptComport <- struct{}{}
				}
				currentWorkInterrupted = true
				waitComport = true
				continue
			}
			return

		case <-x.comports:
			ports, err := comport.GetAvailablePorts()
			if err != nil {
				x.peer.HardwareConnectionError(err.Error())
			}
			// отправить доступные компорты
			x.peer.ComPorts(ports)

			for _, p := range ports {
				if p == cfg.SerialPortName {
					continue gotoSelect
				}
			}

			if len(ports) > 0 {
				cfg.SerialPortName = ports[0]
				cfg.Save()
				// отправить конфиг
				x.peer.HardwareConfig(cfg)
			}

		case c := <-x.setPinChecked:
			cfg.CheckedPlaces[c.pin] = c.checked
			cfg.Save()

		case portName := <-x.setPortName:
			if cfg.SerialPortName != portName {
				cfg.SerialPortName = portName
				cfg.Save()
			}
		case <-x.chComportDone:
			if waitComport {
				return
			}
			started = false

		case <-x.chStart:
			if !started {
				started = true
				currentWorkInterrupted = false
				go x.runComPort(cfg.SerialPortName)
			}
		case <-x.chStop:
			if interruptComport != nil {
				interruptComport <- struct{}{}
			}
			currentWorkInterrupted = true
		}
	}
}

func (x Provider) readPin(port *comport.Port, pin int) (reading Reading) {

	reading.Pin = pin

	addr := modbus.Addr(17)
	n := byte(pin)
	if pin > 4 {
		n -= 5
		addr--
	}
	request := modbus.Request{
		Addr:                addr,
		ProtocolCommandCode: 3,
		Data:                append([]byte{0, 6*n + 2, 0, 1}),
	}
	bytes, err := port.Fetch(request.Bytes())
	if err != nil {
		reading.Error = err
		return
	}

	if reading.Error = request.CheckResponse(bytes); reading.Error != nil {
		return
	}
	if len(bytes) != 7 {
		reading.Error = fmt.Errorf("% X: длина ответа меньше семи", bytes)
		return
	}

	if reading.Status = binary.BigEndian.Uint16(bytes[3:]); reading.Status != 0 {
		reading.Error = fmt.Errorf("статус не ноль: %X", reading.Status)
		return
	}

	if x.CurrentWorkInterrupted() {
		reading.Error = errors.New("прервано")
		return
	}

	request.ProtocolCommandCode = 3
	request.Data = append([]byte{0, 6*n + 4, 0, 2})

	bytes, err = port.Fetch(request.Bytes())

	if err != nil {
		reading.Error = err
		return
	}
	if reading.Error = request.CheckResponse(bytes); reading.Error == nil {
		return
	}
	if len(bytes) != 9 {
		reading.Error = fmt.Errorf("длина ответа не девять: % X", bytes)
		return
	}
	reading.Value = math.Float32frombits(binary.BigEndian.Uint32(bytes[3:]))
	if reading.Value > 1000 {
		reading.Error = fmt.Errorf("больше тысячи: %v", reading.Value)
	}
	return

}
