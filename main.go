package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	batt_stat_file     = "/sys/class/power_supply/battery/status"
	current_limit_file = "/sys/class/power_supply/battery/constant_charge_current_max"
	capacity_file      = "/sys/class/power_supply/bms/capacity"
	capacity_raw_file  = "/sys/class/power_supply/bms/capacity_raw"
	current_input_file = "/sys/class/power_supply/usb/input_current_now"
)

var (
	Threshold int
	Stop      bool = false
	Enabled   bool = false
)

type Info struct {
	Status              string
	BattLevel, AmpLimit int
}

func main() {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigchan
		Stop = true
	}()
	fmt.Println("Welcome. This program is written by Rajvaibhav Raskar.")
	fmt.Println("Press Ctl+C to quit and restore charging config.")
	http.HandleFunc("/", handler)
	fmt.Println("Server Started on http://127.0.0.1:64001")
	go func() {
		err := http.ListenAndServe("127.0.0.1:64001", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()
	for {
		if Stop {
			restoreConfig()
			fmt.Println("Max Charging Current setting restored.")
			os.Exit(0)
		}
		if Enabled {
			inCurrent, err := readIntFromFile(current_input_file)
			if err != nil {
				log.Fatal(err)
			}
			var charging bool
			if inCurrent > 0 {
				charging = true
			} else {
				charging = false
			}
			if charging {
				raw_cap, err := readIntFromFile(capacity_raw_file)
				if err != nil {
					log.Fatal(err)
				}
				AmpLimit := 1000
				switch {
				case raw_cap > Threshold:
					AmpLimit = 1000
				case raw_cap < Threshold-25:
					AmpLimit = 3000000
				case raw_cap < Threshold:
					AmpLimit = 50000
				}
				setChargeCurrent(AmpLimit)
			} else {
				setChargeCurrent(3000000)
			}
		}
		time.Sleep(time.Second)
	}
}

func readIntFromFile(file string) (int, error) {
	byte_str, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	str := strings.TrimSpace(strings.ToLower(string(byte_str)))
	return strconv.Atoi(str)
}

func writeIntToFile(file string, val int) error {
	f, err := os.OpenFile(file, os.O_RDWR, 0000)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	_, err = f.WriteString(strconv.Itoa(val))
	return err
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		var response_html string = ""
		response_html = response_html + PART1
		if Enabled {
			response_html = response_html + fmt.Sprintf(`<p class="mb-4">Bypass Charging Enabled at %.f.</p>`,
				float64(Threshold)/100) + "\n" + `<a href="/disable" class="btn btn-warning">Disable</a>` + "\n"
		} else {
			response_html += `<p class="mb-4">Bypass Charging Disabled.</p>` + "\n" +
				`<a href="/enable" class="btn btn-success">Enable</a>` + "\n"
		}
		response_html = response_html + PART2
		fmt.Fprintln(w, response_html)

	} else {
		path := strings.TrimPrefix(r.URL.Path, "/")
		switch {
		case path == "enable":
			setThreshold(0)
			Enabled = true
		case path == "disable":
			Enabled = false
			restoreConfig()
		default:
			batt_level, err := strconv.Atoi(path)
			if err == nil && batt_level >= 39 && batt_level <= 100 {
				setThreshold(batt_level)
				Enabled = true
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func setThreshold(batt_level int) {
	var err error
	if batt_level == 0 {
		batt_level, err = readIntFromFile(capacity_file)
		if err != nil {
			log.Fatal(err)
		}
	}
	switch {
	case batt_level == 100:
		Threshold = 9975
	case batt_level == 99:
		Threshold = 9875
	default:
		Threshold = batt_level * 100
	}
}

func restoreConfig() {
	err := writeIntToFile(current_limit_file, 3000000)
	if err != nil {
		log.Fatal(err)
	}
}

func setChargeCurrent(current int) {
	current_, err := readIntFromFile(current_limit_file)
	if err != nil {
		log.Fatal(err)
	}
	if current == current_ {
		return
	} else {
		err = writeIntToFile(current_limit_file, current)
		if err != nil {
			log.Fatal(err)
		}
	}
}
