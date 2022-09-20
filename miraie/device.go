package miraie

import (
	"encoding/json"
	"errors"
	"fmt"
	"fyne.io/systray"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

type Device struct {
	DeviceId   string   `json:"deviceId"`
	DeviceName string   `json:"deviceName"`
	Topic      []string `json:"topic"`

	apiToken ApiToken
	homeId   string
	topic    string
	logger   *log.Entry
	client   MQTT.Client

	status Status

	statusMenu  *systray.MenuItem
	onOff       *systray.MenuItem
	temperature *systray.MenuItem
	tempSubs    []*systray.MenuItem
	mode        *systray.MenuItem
	modeSubs    map[AcMode]*systray.MenuItem
	speed       *systray.MenuItem
	speedSubs   map[FanSpeed]*systray.MenuItem
}
type Toggle string
type AcMode string
type FanSpeed string

const (
	ToggleOn  Toggle = "on"
	ToggleOff Toggle = "off"

	ModeAuto AcMode = "auto"
	ModeDry  AcMode = "dry"
	ModeFan  AcMode = "fan"

	SpeedAuto FanSpeed = "auto"
	SpeedHigh FanSpeed = "high"
	SpeedMed  FanSpeed = "medium"
	SpeedLow  FanSpeed = "low"
	SpeedQt   FanSpeed = "quiet"
)

var (
	modes  = []AcMode{ModeAuto, ModeDry, ModeFan}
	speeds = []FanSpeed{SpeedAuto, SpeedQt, SpeedLow, SpeedMed, SpeedHigh}
)

type Status struct {
	PS    Toggle   `json:"ps"`
	ACTmp string   `json:"actmp"`
	ACMd  AcMode   `json:"acmd"`
	ACFs  FanSpeed `json:"acfs"`
}

type MsgPayload struct {
	KI    int      `json:"ki,omitempty"`
	CNT   string   `json:"cnt,omitempty"`
	SID   string   `json:"sid,omitempty"`
	PS    Toggle   `json:"ps,omitempty"`
	ACTmp string   `json:"actmp,omitempty"`
	ACMd  AcMode   `json:"acmd,omitempty"`
	ACFs  FanSpeed `json:"acfs,omitempty"`
}

func (d *Device) Connect() (err error) {
	if len(d.Topic) < 1 {
		err = errors.New("can't connect to device, topic empty")
		return
	}
	d.logger = log.WithField("device", d.DeviceName)
	d.topic = d.Topic[0]
	err = d.connect()
	if err != nil {
		d.logger.Error(err)
		return
	}
	d.setupTray()
	err = d.subscribe(d.topic+"/status", d.onStatusUpdate)
	if err != nil {
		return
	}
	return
}

func (d *Device) TogglePower() {
	if d.status.PS == ToggleOn {
		d.status.PS = ToggleOff
	} else {
		d.status.PS = ToggleOn
	}
	msg := d.baseMsg()
	msg.PS = d.status.PS
	d.updateControl(msg)
	d.logger.WithField("power", d.status.PS).Info("power toggled")
	return
}

func (d *Device) SetTemp(temp int) {
	msg := d.baseMsg()
	msg.ACTmp = fmt.Sprintf("%.1f", float64(temp))
	d.updateControl(msg)
	d.logger.WithField("tmp", msg.ACTmp).Info("temp updated")
}

func (d *Device) SetMode(mode AcMode) {
	msg := d.baseMsg()
	msg.ACMd = mode
	d.updateControl(msg)
	d.logger.WithField("mode", msg.ACMd).Info("mode updated")
}

func (d *Device) SetSpeed(speed FanSpeed) {
	msg := d.baseMsg()
	msg.ACFs = speed
	d.updateControl(msg)
	d.logger.WithField("speed", msg.ACFs).Info("speed updated")
}

func (d *Device) Disconnect() {
	d.client.Disconnect(0)
	d.logger.Info("device disconnected")
}

func (d *Device) baseMsg() MsgPayload {
	return MsgPayload{
		KI:  1,
		CNT: "an",
		SID: "1",
	}
}

func (d *Device) updateControl(msg MsgPayload) {
	d.logger.WithField("msg", fmt.Sprintf("%#v", msg)).Debug("updating control")
	payload, _ := json.Marshal(msg)
	token := d.client.Publish(d.topic+"/control", byte(0), false, payload)
	if token.Wait() && token.Error() != nil {
		d.logger.Error(token.Error())
		return
	}
}

func (d *Device) onStatusUpdate(client MQTT.Client, message MQTT.Message) {
	d.logger.Debug(string(message.Payload()))
	err := json.Unmarshal(message.Payload(), &d.status)
	if err != nil {
		log.Error(err)
		return
	}
	if d.status.PS == ToggleOn {
		d.statusMenu.SetTitle("Status: " + strings.ToTitle(string(ToggleOn)))
		d.onOff.SetTitle(strings.ToTitle(string(ToggleOff)))
		d.temperature.Enable()
		d.mode.Enable()
		d.speed.Enable()
	} else if d.status.PS == ToggleOff {
		d.statusMenu.SetTitle("Status: " + strings.ToUpper(string(ToggleOff)))
		d.onOff.SetTitle(strings.ToUpper(string(ToggleOn)))
		d.temperature.Disable()
		d.mode.Disable()
		d.speed.Disable()
	}
	if t, e := strconv.ParseFloat(d.status.ACTmp, 64); e == nil {
		for i, sub := range d.tempSubs {
			if i == int(t) && sub != nil && !sub.Checked() {
				sub.Check()
			} else if sub != nil && sub.Checked() {
				sub.Uncheck()
			}
		}
	}

	for mode, item := range d.modeSubs {
		if mode == d.status.ACMd && item != nil && !item.Checked() {
			item.Check()
		} else if item != nil && item.Checked() {
			item.Uncheck()
		}
	}

	for speed, item := range d.speedSubs {
		if speed == d.status.ACFs && item != nil && !item.Checked() {
			item.Check()
		} else if item != nil && item.Checked() {
			item.Uncheck()
		}
	}
}

func (d *Device) setupTray() {
	item := systray.AddMenuItem(d.DeviceName, "")

	s := strings.ToUpper(string(ToggleOff))
	if d.status.PS == ToggleOn {
		strings.ToUpper(string(ToggleOn))
	}
	d.statusMenu = item.AddSubMenuItem("Status: "+s, "")
	d.statusMenu.Disable()

	title := strings.ToUpper(string(ToggleOn))
	if d.status.PS == ToggleOn {
		title = strings.ToUpper(string(ToggleOff))
	}
	d.onOff = item.AddSubMenuItem(title, "")
	go func(d *Device) {
		for range d.onOff.ClickedCh {
			d.TogglePower()
		}
	}(d)

	d.temperature = item.AddSubMenuItem("Temperature", "")
	d.temperature.Disable()
	d.tempSubs = make([]*systray.MenuItem, 30)
	tmp, _ := strconv.ParseFloat(d.status.ACTmp, 64)
	for i := 16; i < 29; i++ {
		d.tempSubs[i] = d.temperature.AddSubMenuItemCheckbox(fmt.Sprintf("%d", i), "", int(tmp) == i)
		go func(x int) {
			for range d.tempSubs[x].ClickedCh {
				d.SetTemp(x)
			}
		}(i)
	}

	d.mode = item.AddSubMenuItem("Mode", "")
	d.mode.Disable()
	d.modeSubs = make(map[AcMode]*systray.MenuItem)
	for _, mode := range modes {
		d.modeSubs[mode] = d.mode.AddSubMenuItemCheckbox(strings.ToTitle(string(mode)), "", d.status.ACMd == mode)
		go func(m AcMode) {
			for range d.modeSubs[m].ClickedCh {
				d.SetMode(m)
			}
		}(mode)
	}

	d.speed = item.AddSubMenuItem("Fan Speed", "")
	d.speed.Disable()
	d.speedSubs = make(map[FanSpeed]*systray.MenuItem)
	for _, speed := range speeds {
		d.speedSubs[speed] = d.speed.AddSubMenuItemCheckbox(strings.ToTitle(string(speed)), "", speed == d.status.ACFs)
		go func(s FanSpeed) {
			for range d.speedSubs[s].ClickedCh {
				d.SetSpeed(s)
			}
		}(speed)
	}
}
