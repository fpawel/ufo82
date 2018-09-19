package main

import (
	"fmt"
	"github.com/fpawel/procmq"
	"github.com/fpawel/ufo82/internal/hardware"
	"github.com/fpawel/ufo82/internal/ufo82"
	"net"
)

const (
	PeerMsgYears = iota
	PeerMsgMonthsOfYear
	PeerMsgDaysOfYearMonth
	PeerMsgPartiesOfYearMonthDay
	PeerMsgProductsOfParty
	PeerMsgSensitivitiesOfProduct
	PeerCurrentProductSerial
	PeerCreateNewParty
	PeerPortName
	PeerPlaceChecked
	PeerStartHardware
	PeerStopHardware
)

type app struct {
	pipe     procmq.ProcessMQ
	hardware hardware.Provider
	peer     syncSender
	db       ufo82.DB
}

func newApp(writerPipeConn net.Conn) *app {
	x := new(app)
	x.db = ufo82.MustConnectDB(appFolderFileName("products.db"))
	x.peer = newSyncSender(writerPipeConn, x.db)
	x.hardware = hardware.NewProvider(x.peer, appFolderFileName("hardware.json"))
	return x
}

func (x *app) Close() error {
	fmt.Println("CLOSE HARDWARE:", x.hardware.Close())
	fmt.Println("CLOSE PEER:", x.peer.Close())
	fmt.Println("CLOSE DATABASE:", x.db.Close())
	return nil
}

func (x *app) Run(readerPipeConn net.Conn) error {
	pipe := procmq.Conn{Conn: readerPipeConn}
	for {
		cmd, err := pipe.ReadUInt32()
		if err != nil {
			return err
		}
		switch cmd {
		case PeerMsgYears:
			x.peer.SendYears()

		case PeerMsgMonthsOfYear:
			year, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			x.peer.SendMonths(int(year))

		case PeerMsgDaysOfYearMonth:
			year, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			month, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			x.peer.SendDays(ufo82.YearMonth{int(year), int(month)})

		case PeerMsgPartiesOfYearMonthDay:
			year, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			month, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			day, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			x.peer.SendParties(ufo82.YearMonthDay{Year: int(year), Month: int(month), Day: int(day)})
		case PeerMsgProductsOfParty:
			partyID, err := pipe.ReadUInt64()
			if err != nil {
				return err
			}
			x.peer.SendProductsOfParty(ufo82.PartyID(partyID))
		case PeerMsgSensitivitiesOfProduct:
			productID, err := pipe.ReadUInt64()
			if err != nil {
				return err
			}
			x.peer.SendSensitivitiesOfProduct(ufo82.ProductID(productID))
		case PeerCurrentProductSerial:
			order, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			serial, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			x.peer.ApplyCurrentProductOrderSerial(
				ufo82.ProductOrderSerial{
					Order:  int(order),
					Serial: int(serial),
				})
		case PeerCreateNewParty:
			x.peer.CreateNewParty()

		case PeerPortName:
			portName, err := pipe.ReadString()
			if err != nil {
				return err
			}
			x.hardware.SetPortName(portName)

		case PeerPlaceChecked:
			checked, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			order, err := pipe.ReadUInt32()
			if err != nil {
				return err
			}
			x.hardware.SetChecked(int(order), checked != 0)

		case PeerStartHardware:
			x.hardware.Start()
		case PeerStopHardware:
			x.hardware.Stop()

		default:
			panic(fmt.Errorf("unknown message: %d", cmd))
		}
	}

}
