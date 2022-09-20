package main

import (
	"flag"
	"fyne.io/systray"
	"github.com/iMro0t/go-miraie/icon"
	"github.com/iMro0t/go-miraie/miraie"
	log "github.com/sirupsen/logrus"
)

var (
	client *miraie.Client
)

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	log.SetLevel(log.DebugLevel)
	username := flag.String("username", "", "username")
	password := flag.String("password", "", "password")
	flag.Parse()
	client = miraie.NewClient()
	checkError(client.Login(*username, *password))
	checkError(client.FetchHomes())
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(icon.Logo)
	systray.SetTooltip("MirAIe AC Control")

	for _, device := range client.Devices {
		_ = device.Connect()
	}

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()
}

func onExit() {
	for _, device := range client.Devices {
		device.Disconnect()
	}
}
