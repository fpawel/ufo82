package main

import (
	"fmt"
	"github.com/fpawel/procmq"
	"github.com/fpawel/ufo82/internal/hardware"
	"github.com/fpawel/ufo82/internal/ufo82"
	"net"
	"runtime"
	"time"
)

const (
	msgYears = iota
	msgMonthsOfYear
	msgDaysOfYearMonth
	msgPartiesOfYearMonthDay
	msgProductsOfParty
	msgSensitivitiesOfProduct
	msgCurrentParty
	msgInfoMessage
	msgHardwareSensitivity
	msgHardwareConnected
	msgHardwareDisconnected
	msgHardwareConnectionError
	msgHardwareConfig
	msgHardwareCurrentPlace
	msgComPorts
)

type sender struct {
	db        ufo82.DB
	conn      procmq.Conn
	pipeError error
}

type InfoMessage struct {
	Text, Color string
}

func newSender(db ufo82.DB, conn net.Conn) *sender {
	return &sender{
		db:   db,
		conn: procmq.Conn{Conn: conn},
	}
}

func (x *sender) failed() bool {
	if x.pipeError == nil {
		return false
	}
	_, _, line, _ := runtime.Caller(2)
	x.pipeError = fmt.Errorf("%v: %d", x.pipeError, line)
	return true
}

func (x *sender) writeUInt32(v uint32) {
	if x.failed() {
		return
	}
	x.pipeError = x.conn.WriteUInt32(v)
}

func (x *sender) writeUInt64(v uint64) {
	if x.failed() {
		return
	}
	x.pipeError = x.conn.WriteUInt64(v)
}

func (x *sender) writeString(s string) {
	if x.failed() {
		return
	}
	x.pipeError = x.conn.WriteString(s)
}

func (x *sender) writeTime(t time.Time) {
	if x.failed() {
		return
	}
	x.pipeError = x.conn.WriteTime(t)
}

func (x *sender) writeFloat64(v float64) {
	if x.failed() {
		return
	}
	x.pipeError = x.conn.WriteFloat64(v)
}

func (x *sender) CreateNewParty() {
	x.db.CreateNewParty()
	x.years()
	x.currentParty()
	x.InfoMessage(InfoMessage{"создана новая партия приборов", "clBlue"})
}

func (x *sender) AppConfig(config hardware.Config) {
	x.writeUInt32(msgHardwareConfig)
	// отправить имя ком порта из настроек
	x.writeString(config.SerialPortName)
	// отправить выбранность мест стенда из настроек
	for i := 0; i < 10; i++ {
		v := uint32(0)
		if config.CheckedPlaces[i] {
			v = 1
		}
		x.writeUInt32(v)
	}
	return
}

func (x *sender) InfoMessage(m InfoMessage) {
	x.writeUInt32(msgInfoMessage)
	x.writeString(m.Text)
	x.writeString(m.Color)
}

func (x *sender) party(party ufo82.Party) {
	x.writeUInt64(uint64(party.PartyID))
	x.writeTime(party.CreatedAt)
}

func (x *sender) product(product ufo82.Product) {
	x.writeUInt64(uint64(product.ProductID))
	x.writeUInt32(uint32(product.Order))
	x.writeUInt32(uint32(product.ProductNumber))
}

func (x *sender) partyAndItsProducts(partyID ufo82.PartyID) {
	party, products := x.db.GetPartyByID(partyID)
	x.party(party)
	x.writeUInt32(uint32(len(products)))
	for _, product := range products {
		x.product(product)
	}
}

func (x *sender) currentParty() {
	partyID := x.db.GetLastPartyID()
	x.writeUInt32(msgCurrentParty)
	x.partyAndItsProducts(partyID)
}

func (x *sender) years() {
	years := x.db.GetYears()
	x.writeUInt32(msgYears)
	x.writeUInt32(uint32(len(years)))
	for _, y := range years {
		x.writeUInt32(uint32(y))
	}
}

func (x *sender) monthsOfYear(year int) {
	months := x.db.GetMonthsOfYear(year)
	x.writeUInt32(msgMonthsOfYear)
	x.writeUInt32(uint32(year))
	x.writeUInt32(uint32(len(months)))
	for _, m := range months {
		x.writeUInt32(uint32(m))
	}
}

func (x *sender) daysOfYearMonth(ym ufo82.YearMonth) {
	days := x.db.GetDaysOfYearMonth(ym)
	x.writeUInt32(msgDaysOfYearMonth)
	x.writeUInt32(uint32(ym.Year))
	x.writeUInt32(uint32(ym.Month))
	x.writeUInt32(uint32(len(days)))
	for _, m := range days {
		x.writeUInt32(uint32(m))
	}
}

func (x *sender) partiesOfMonthYearDay(ym ufo82.YearMonthDay) {
	parties := x.db.GetPartiesOfYearMonthDay(ym)
	x.writeUInt32(msgPartiesOfYearMonthDay)
	x.writeUInt32(uint32(ym.Year))
	x.writeUInt32(uint32(ym.Month))
	x.writeUInt32(uint32(ym.Day))

	x.conn.WriteUInt32(uint32(len(parties)))
	for _, party := range parties {
		x.party(party)
	}
	return
}

func (x *sender) sensitivitiesOfProduct(productID ufo82.ProductID) {
	ds := x.db.GetSensitivitiesByProductID(productID)

	x.writeUInt32(msgSensitivitiesOfProduct)
	x.writeUInt64(uint64(productID))

	x.writeUInt32(uint32(len(ds)))
	for _, m := range ds {
		x.writeTime(m.StoredAt)
		x.writeFloat64(m.Value)
	}
	return
}

func (x *sender) PartyAndItsProducts(partyID ufo82.PartyID) {
	x.writeUInt32(msgProductsOfParty)
	x.partyAndItsProducts(partyID)
}

func (x *sender) applyCurrentProductOrderSerial(inp ufo82.ProductOrderSerial) {
	msg := x.db.ApplyCurrentProductSerial(inp)
	x.currentParty()
	x.years()
	x.InfoMessage(InfoMessage{msg, "clNavy"})
}

func (x *sender) HardwareReading(s hardware.Reading) {
	errStr := ""
	if s.Error != nil {
		errStr = s.Error.Error()
	}

	x.writeUInt32(msgHardwareSensitivity)
	x.writeUInt32(uint32(s.Pin))
	x.writeUInt32(uint32(s.Status))
	x.writeFloat64(float64(s.Value))
	x.writeString(errStr)

}

func (x *sender) HardwareConnected() {

	x.writeUInt32(msgHardwareConnected)
}

func (x *sender) HardwareDisconnected() {
	x.writeUInt32(msgHardwareDisconnected)
}

func (x *sender) HardwareConnectionError(errStr string) {
	x.writeUInt32(msgHardwareConnectionError)
	x.writeString(errStr)
}

func (x *sender) HardwareCurrentPlace(n int) {
	x.writeUInt32(msgHardwareCurrentPlace)
	x.writeUInt32(uint32(n))
}

func (x *sender) ComPorts(ports []string) {
	x.writeUInt32(msgComPorts)
	x.writeUInt32(uint32(len(ports)))
	for _, s := range ports {
		x.writeString(s)
	}

}
