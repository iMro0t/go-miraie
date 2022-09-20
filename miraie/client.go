package miraie

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"net/http"
	"time"
)

const (
	name     = "MirAIe"
	clientId = "PBcMcfG19njNCL8AOgvRzIC8AjQa"
	loginUrl = "https://auth.miraie.in/simplifi/v1/userManagement/login"
	homesUrl = "https://app.miraie.in/simplifi/v1/homeManagement/homes"
)

type ApiToken struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	UserId       string `json:"userId"`
}

func init() {
	rand.Seed(time.Now().UnixMilli())
}

type Client struct {
	username string
	password string
	apiToken ApiToken

	Devices []*Device

	client     MQTT.Client
	httpClient *http.Client
	logger     *log.Entry
}

func NewClient() *Client {
	m := new(Client)
	m.httpClient = &http.Client{Timeout: time.Second * 30, Transport: &AddAuthHeader{c: m}}
	m.logger = log.WithField("client", name)
	return m
}

func (m *Client) Login(username, password string) (err error) {
	m.username = username
	m.password = password
	if m.username == "" || m.password == "" {
		err = fmt.Errorf("missing credentials")
		return
	}
	err = m.login()
	if err != nil {
		return
	}
	m.logger.Info("logged in successfully")
	return
}

func (m *Client) login() (err error) {
	reqData, err := json.Marshal(map[string]string{
		"clientId": clientId,
		"email":    m.username,
		"password": m.password,
		"scope":    fmt.Sprintf("an_%d", rand.Intn(1000000000)),
	})
	if err != nil {
		return
	}
	resp, err := m.httpClient.Post(loginUrl, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var rjson struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&rjson)
		err = errors.New(rjson.Message)
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&m.apiToken)
	return
}

func (m *Client) FetchHomes() (err error) {
	resp, err := m.httpClient.Get(homesUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var rjson struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&rjson)
		err = errors.New(rjson.Message)
		return
	}
	var rjson []struct {
		HomeId string `json:"homeId"`
		Spaces []struct {
			Devices []Device `json:"Devices"`
		} `json:"spaces"`
	}
	err = json.NewDecoder(resp.Body).Decode(&rjson)
	if err != nil {
		return
	}

	for _, home := range rjson {
		for _, space := range home.Spaces {
			if len(space.Devices) > 0 {
				m.Devices = make([]*Device, 0)
				m.logger.Infof("found %d Devices", len(space.Devices))
				for _, device := range space.Devices {
					device.homeId = home.HomeId
					device.apiToken = m.apiToken
					m.Devices = append(m.Devices, &device)
				}
				break
			}
		}
		break
	}
	return
}

type AddAuthHeader struct {
	c *Client
}

func (a *AddAuthHeader) RoundTrip(r *http.Request) (*http.Response, error) {
	if a.c != nil && a.c.apiToken.AccessToken != "" {
		r.Header.Set("Authorization", "Bearer "+a.c.apiToken.AccessToken)
	}
	return http.DefaultTransport.RoundTrip(r)
}
