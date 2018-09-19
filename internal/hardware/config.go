package hardware

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Config struct {
	SerialPortName string
	CheckedPlaces  [10]bool
	filename       string
}

func loadConfig(filename string) Config {

	r := Config{filename: filename}
	// считать настройки приложения из сохранённого файла json
	b, err := ioutil.ReadFile(filename)
	if err == nil {
		err = json.Unmarshal(b, &r)
	}
	if err != nil {
		fmt.Println("кофиг железа:", err, filename)
		r = defaultConfig()
		r.Save()
	}
	return r
}

func (x Config) Save() {
	// сохранить конфиг
	b, err := json.MarshalIndent(x, "", "    ")
	if err != nil {
		panic(err)
	}

	file, err := os.Create(x.filename)
	if err != nil {
		fmt.Println("unable to create hardware config file:", err)
		return
	}
	if _, err := file.Write(b); err != nil {
		fmt.Println("unable to write hardware config file:", err)
		return
	}
}

func defaultConfig() Config {
	return Config{
		SerialPortName: "COM1",
	}
}

func (x Config) CheckedPlaceExists() bool {
	for i := 0; i < 10; i++ {
		if x.CheckedPlaces[i] {
			return true
		}
	}
	return false
}
