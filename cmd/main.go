package main

import (
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/natefinch/npipe.v2"
	"os/exec"
)

func main() {

	// сделать cmd сервер
	pipeReadListener, err := npipe.Listen(`\\.\pipe\$UFO82_FROM_PEER_TO_MASTER$`)
	if err != nil {
		panic(err)
	}
	defer pipeReadListener.Close()

	pipeWriteListener, err := npipe.Listen(`\\.\pipe\$UFO82_FROM_MASTER_TO_PEER$`)
	if err != nil {
		panic(err)
	}
	defer pipeWriteListener.Close()

	if err := exec.Command(appFolderFileName("ufo82.exe")).Start(); err != nil {
		panic(err)
	}

	readerPipeConn, err := pipeReadListener.Accept()
	if err != nil {
		panic(err)
	}

	defer readerPipeConn.Close()

	writerPipeConn, err := pipeWriteListener.Accept()
	if err != nil {
		panic(err)
	}
	defer writerPipeConn.Close()

	app := newApp(writerPipeConn)
	fmt.Println("END APP:", app.Run(readerPipeConn))
	fmt.Println("CLOSE APP:", app.Close())
}
