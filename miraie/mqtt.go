package miraie

import (
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"math/rand"
)

func (d *Device) connect() (err error) {
	opts := MQTT.NewClientOptions()
	opts.AddBroker("tls://mqtt.miraie.in:8883")
	opts.SetUsername(d.homeId)
	opts.SetPassword(d.apiToken.AccessToken)
	opts.SetCleanSession(false)
	opts.SetClientID(fmt.Sprintf("an%d%d", rand.Intn(10000000000000000), rand.Intn(1000000)))
	opts.SetOnConnectHandler(func(client MQTT.Client) {
		d.logger.Info("connected to device")
	})

	d.client = MQTT.NewClient(opts)
	token := d.client.Connect()
	if token.Wait() && token.Error() != nil {
		err = token.Error()
		return
	}
	return
}

func (d *Device) subscribe(topic string, callback MQTT.MessageHandler) (err error) {
	token := d.client.Subscribe(topic, byte(0), callback)
	if token.Wait() && token.Error() != nil {
		err = token.Error()
		return
	}
	return
}
