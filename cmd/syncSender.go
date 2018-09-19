package main

import (
	"github.com/fpawel/ufo82/internal/hardware"
	"github.com/fpawel/ufo82/internal/ufo82"
	"net"
)

type syncSender struct {
	hardwareConnectionError chan string
	done                    chan error
	interrupt,
	years,

	hardwareConnected,
	hardwareDisconnected,
	newParty chan bool
	monthsOfYear                   chan int
	partiesOfYearMonthDay          chan ufo82.YearMonthDay
	daysOfYearMonth                chan ufo82.YearMonth
	productsOfParty                chan ufo82.PartyID
	sensitivitiesOfProduct         chan ufo82.ProductID
	applyCurrentProductOrderSerial chan ufo82.ProductOrderSerial
	hardwareReading                chan hardware.Reading
	infoMessage                    chan InfoMessage
	hardwareConfig                 chan hardware.Config
	hardwareCurrentPlace           chan int
	comports                       chan []string
}

func newSyncSender(writerPipeConn net.Conn, db ufo82.DB) (x syncSender) {

	sender := newSender(db, writerPipeConn)

	// отправить года
	sender.years()

	// отправить текущую партию
	sender.currentParty()

	x.done = make(chan error)
	x.comports = make(chan []string)
	x.interrupt = make(chan bool, 2)
	x.years = make(chan bool)
	x.newParty = make(chan bool)
	x.hardwareConnected = make(chan bool)
	x.hardwareDisconnected = make(chan bool)
	x.hardwareConnectionError = make(chan string)
	x.monthsOfYear = make(chan int)
	x.partiesOfYearMonthDay = make(chan ufo82.YearMonthDay)
	x.daysOfYearMonth = make(chan ufo82.YearMonth)

	x.productsOfParty = make(chan ufo82.PartyID)
	x.sensitivitiesOfProduct = make(chan ufo82.ProductID)
	x.applyCurrentProductOrderSerial = make(chan ufo82.ProductOrderSerial)
	x.hardwareReading = make(chan hardware.Reading)
	x.infoMessage = make(chan InfoMessage)
	x.hardwareConfig = make(chan hardware.Config)
	x.hardwareCurrentPlace = make(chan int)

	go x.run(sender)

	return x
}

func (x syncSender) Close() error {
	x.interrupt <- true
	return <-x.done
}

func (x syncSender) CreateNewParty() {
	x.newParty <- true
}

func (x syncSender) SendYears() {
	x.years <- true
}

func (x syncSender) SendMonths(year int) {
	x.monthsOfYear <- year
}

func (x syncSender) SendDays(ym ufo82.YearMonth) {
	x.daysOfYearMonth <- ym
}

func (x syncSender) SendParties(m ufo82.YearMonthDay) {
	x.partiesOfYearMonthDay <- m
}

func (x syncSender) SendProductsOfParty(partyID ufo82.PartyID) {
	x.productsOfParty <- partyID
}

func (x syncSender) SendSensitivitiesOfProduct(productID ufo82.ProductID) {
	x.sensitivitiesOfProduct <- productID
}

func (x syncSender) ApplyCurrentProductOrderSerial(p ufo82.ProductOrderSerial) {
	x.applyCurrentProductOrderSerial <- p
}

func (x syncSender) SendInfoMessage(m InfoMessage) {
	x.infoMessage <- m
}

func (x syncSender) HardwareReading(s hardware.Reading) {
	x.hardwareReading <- s
}

func (x syncSender) HardwareConnected() {
	x.hardwareConnected <- true
}
func (x syncSender) HardwareDisconnected() {
	x.hardwareDisconnected <- true
}

func (x syncSender) HardwareConnectionError(errStr string) {
	x.hardwareConnectionError <- errStr
}

func (x syncSender) HardwareConfig(config hardware.Config) {
	x.hardwareConfig <- config
}

func (x syncSender) HardwareCurrentPlace(n int) {
	x.hardwareCurrentPlace <- n
}

func (x syncSender) ComPorts(ports []string) {
	x.comports <- ports
}

func (x syncSender) run(senderMessages *sender) {

	defer func() {
		x.done <- senderMessages.pipeError
	}()
	var currentProducts []ufo82.Product

	for {

		select {

		case <-x.interrupt:
			return

		case <-x.newParty:
			senderMessages.CreateNewParty()
			currentProducts = senderMessages.db.GetLastPartyProducts()

		case <-x.years:
			senderMessages.years()

		case year := <-x.monthsOfYear:
			senderMessages.monthsOfYear(year)

		case ym := <-x.daysOfYearMonth:
			senderMessages.daysOfYearMonth(ym)

		case ym := <-x.partiesOfYearMonthDay:
			senderMessages.partiesOfMonthYearDay(ym)

		case partyID := <-x.productsOfParty:
			senderMessages.PartyAndItsProducts(partyID)

		case productID := <-x.sensitivitiesOfProduct:
			senderMessages.sensitivitiesOfProduct(productID)

		case z := <-x.applyCurrentProductOrderSerial:
			senderMessages.applyCurrentProductOrderSerial(z)
			currentProducts = senderMessages.db.GetLastPartyProducts()

		case m := <-x.infoMessage:
			senderMessages.InfoMessage(m)

		case <-x.hardwareConnected:
			currentProducts = senderMessages.db.GetLastPartyProducts()
			for _, p := range currentProducts {
				senderMessages.db.ClearProductSensitivities(p.ProductID)
			}
			senderMessages.HardwareConnected()

		case <-x.hardwareDisconnected:
			senderMessages.HardwareDisconnected()

		case errStr := <-x.hardwareConnectionError:
			senderMessages.HardwareConnectionError(errStr)

		case s := <-x.hardwareReading:
			if s.Error == nil {
				for _, p := range currentProducts {
					if p.Order == int64(s.Pin) {
						senderMessages.db.AddNewSensitivity(p.ProductID, s.Value)
					}
				}
			}
			senderMessages.HardwareReading(s)

		case config := <-x.hardwareConfig:
			senderMessages.AppConfig(config)

		case n := <-x.hardwareCurrentPlace:
			senderMessages.HardwareCurrentPlace(n)

		case ports := <-x.comports:
			senderMessages.ComPorts(ports)

		}
	}
}
