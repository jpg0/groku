package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var CONFIG string

const (
	VERSION = "0.4.1"
	USAGE   = `usage: groku [--version] [--help] <command> [<args>]

CLI remote for your Roku

Commands:
  home            Return to the home screen
  rev             Reverse
  fwd             Fast Forward
  select          Select
  left            Left
  right           Right
  up              Up
  down            Down
  back            Back
  info            Info
  backspace       Backspace
  enter           Enter
  search          Search
  replay          Replay
  play            Play
  pause           Pause
  discover        Discover a roku on your local network
  text            Send text to the Roku
  apps            List installed apps on your Roku
  app             Launch specified app
`
)

type dictonary struct {
	XMLName xml.Name `xml:"apps"`
	Apps    []app    `xml:"app"`
}

type app struct {
	Name string `xml:",chardata"`
	ID   string `xml:"id,attr"`
}

type grokuConfig struct {
	Address   string `json:"address"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	CONFIG = fmt.Sprintf("%s/groku.json", os.TempDir())

	if len(os.Args) == 1 || os.Args[1] == "--help" || os.Args[1] == "-help" ||
		os.Args[1] == "--h" || os.Args[1] == "-h" || os.Args[1] == "help" {
		fmt.Println(USAGE)
		os.Exit(0)
	}

	if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version" ||
		os.Args[1] == "--version") {
		fmt.Printf("groku version %s\n", VERSION)
		os.Exit(0)
	}

	switch os.Args[1] {
	case "home", "rev", "fwd", "select", "left", "right", "down", "up",
		"back", "info", "backspace", "enter", "search":
		http.PostForm(fmt.Sprintf("%vkeypress/%v", getRokuAddress(), os.Args[1]), nil)
		os.Exit(0)
	case "replay":
		http.PostForm(fmt.Sprintf("%vkeypress/%v", getRokuAddress(), "InstantReplay"), nil)
		os.Exit(0)
	case "play", "pause":
		http.PostForm(fmt.Sprintf("%vkeypress/%v", getRokuAddress(), "Play"), nil)
		os.Exit(0)
	case "discover":
		fmt.Println("Found roku at", getRokuAddress())
		os.Exit(0)
	case "text":
		if len(os.Args) < 3 {
			fmt.Println(USAGE)
			os.Exit(1)
		}

		roku := getRokuAddress()
		for _, c := range os.Args[2] {
			http.PostForm(fmt.Sprintf("%skeypress/Lit_%s", roku, url.QueryEscape(string(c))), nil)
		}
		os.Exit(0)
	case "apps":
		dict := queryApps()
		for _, a := range dict.Apps {
			fmt.Println(a.Name)
		}
		os.Exit(0)
	case "app":
		if len(os.Args) < 3 {
			fmt.Println(USAGE)
			os.Exit(1)
		}

		dict := queryApps()

		for _, a := range dict.Apps {
			if a.Name == os.Args[2] {
				http.PostForm(fmt.Sprintf("%vlaunch/%v", getRokuAddress(), a.ID), nil)
				os.Exit(0)
			}
		}
		fmt.Printf("App %q not found\n", os.Args[2])
		os.Exit(1)
	default:
		fmt.Println(USAGE)
		os.Exit(1)
	}
}

func queryApps() dictonary {
	resp, err := http.Get(fmt.Sprintf("%squery/apps", getRokuAddress()))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer resp.Body.Close()

	var dict dictonary
	if err := xml.NewDecoder(resp.Body).Decode(&dict); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return dict
}

func findRoku() string {
	ssdp, err := net.ResolveUDPAddr("udp", "239.255.255.250:1900")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	socket, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, err = socket.WriteToUDP([]byte("M-SEARCH * HTTP/1.1\r\n"+
		"HOST: 239.255.255.250:1900\r\n"+
		"MAN: \"ssdp:discover\"\r\n"+
		"ST: roku:ecp\r\n"+
		"MX: 3 \r\n\r\n"), ssdp)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	answerBytes := make([]byte, 1024)
	err = socket.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, _, err = socket.ReadFromUDP(answerBytes[:])
	if err != nil {
		fmt.Println("Could not find your Roku!")
		os.Exit(1)
	}

	ret := strings.Split(string(answerBytes), "\r\n")
	location := strings.TrimPrefix(ret[6], "LOCATION: ")

	return location
}

func getRokuAddress() string {
	var configFile *os.File
	var config grokuConfig

	configFile, err := os.Open(CONFIG)

	// the config file doesn't exist, but that's okay
	if err != nil {
		config.Address = findRoku()
		config.Timestamp = time.Now().Unix()
	} else {
		// the config file exists
		if err := json.NewDecoder(configFile).Decode(&config); err != nil {
			config.Address = findRoku()
		}

		//if the config file is over 60 seconds old, then replace it
		if config.Timestamp == 0 || time.Now().Unix()-config.Timestamp > 60 {
			config.Address = findRoku()
			config.Timestamp = time.Now().Unix()
		}
	}

	if b, err := json.Marshal(config); err == nil {
		ioutil.WriteFile(CONFIG, b, os.ModePerm)
	}

	return config.Address
}
