package main

import (
	"github.com/lxn/win"
	"os"
	"path/filepath"
	"syscall"
)

const (
	appName = "ufo82"
)

func appFolderPath() string {
	var appDataPath string
	if appDataPath = os.Getenv("MYAPPDATA"); len(appDataPath) == 0 {
		var buf [win.MAX_PATH]uint16
		if !win.SHGetSpecialFolderPath(0, &buf[0], win.CSIDL_APPDATA, false) {
			panic("SHGetSpecialFolderPath failed")
		}
		appDataPath = syscall.UTF16ToString(buf[0:])
	}
	appDataPath = filepath.Join(appDataPath, "Аналитприбор", appName)
	_, err := os.Stat(appDataPath)
	if err != nil {
		if os.IsNotExist(err) { // создать каталог если его нет
			os.Mkdir(appDataPath, os.ModePerm)
		} else {
			panic(err)
		}
	}
	return appDataPath
}

func appFolderFileName(filename string) string {
	return filepath.Join(appFolderPath(), filename)
}
