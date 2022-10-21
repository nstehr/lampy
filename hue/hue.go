package hue

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/lucasb-eyer/go-colorful"
)

type Bridge struct {
	InstanceName string
	HostName     string
	IpAddress    net.IP
	client       *http.Client
	credentials  *credentials
}

type Config struct {
	Name             string      `json:"name"`
	DatastoreVersion string      `json:"datastoreversion"`
	SwVersion        string      `json:"swversion"`
	ApiVersion       string      `json:"apiversion"`
	Mac              string      `json:"mac"`
	Bridgeid         string      `json:"bridgeid"`
	FactoryNew       bool        `json:"factorynew"`
	ReplacesBridgeId interface{} `json:"replacesbridgeid"`
	ModelId          string      `json:"modelid"`
	StarterKitId     string      `json:"starterkitid"`
}

type Light struct {
	ID    string `json:"id"`
	IDV1  string `json:"id_v1"`
	Owner struct {
		Rid   string `json:"rid"`
		Rtype string `json:"rtype"`
	} `json:"owner"`
	Metadata struct {
		Name      string `json:"name"`
		Archetype string `json:"archetype"`
	} `json:"metadata"`
	On struct {
		On bool `json:"on"`
	} `json:"on"`
	Dimming struct {
		Brightness  float64 `json:"brightness"`
		MinDimLevel float64 `json:"min_dim_level"`
	} `json:"dimming"`
	DimmingDelta struct {
	} `json:"dimming_delta"`
	ColorTemperature struct {
		Mirek       int  `json:"mirek"`
		MirekValid  bool `json:"mirek_valid"`
		MirekSchema struct {
			MirekMinimum int `json:"mirek_minimum"`
			MirekMaximum int `json:"mirek_maximum"`
		} `json:"mirek_schema"`
	} `json:"color_temperature"`
	ColorTemperatureDelta struct {
	} `json:"color_temperature_delta"`
	Color struct {
		Xy struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
		Gamut struct {
			Red struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			} `json:"red"`
			Green struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			} `json:"green"`
			Blue struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			} `json:"blue"`
		} `json:"gamut"`
		GamutType string `json:"gamut_type"`
	} `json:"color,omitempty"`
	Dynamics struct {
		Status       string   `json:"status"`
		StatusValues []string `json:"status_values"`
		Speed        float64  `json:"speed"`
		SpeedValid   bool     `json:"speed_valid"`
	} `json:"dynamics"`
	Alert struct {
		ActionValues []string `json:"action_values"`
	} `json:"alert"`
	Signaling struct {
	} `json:"signaling"`
	Mode    string `json:"mode"`
	Type    string `json:"type"`
	Effects struct {
		StatusValues []string `json:"status_values"`
		Status       string   `json:"status"`
		EffectValues []string `json:"effect_values"`
	} `json:"effects,omitempty"`
}

type credentials struct {
	username  string
	clientKey string
}

type lightResponse struct {
	Errors []interface{} `json:"errors"`
	Data   []Light       `json:"data"`
}

const (
	bridgeService  = "_hue._tcp"
	applicationKey = "hue-application-key"
)

func CreateBridge(ip net.IP) *Bridge {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	b := Bridge{IpAddress: ip, client: client}
	return &b
}

func DiscoverBridge() (*Bridge, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	defer cancel()
	err = resolver.Browse(ctx, bridgeService, "local", entries)
	if err != nil {
		return nil, err
	}
	log.Println("searching for bridge")
	var entry *zeroconf.ServiceEntry
	foundEntry := make(chan *zeroconf.ServiceEntry)
	// what we do is spin of a goroutine that will process the entries registered in
	// mDNS for our service.  As soon as we detect there is one with an IP4 address
	// we send it off and cancel to stop the searching.
	// there is an issue, https://github.com/grandcat/zeroconf/issues/27 where we
	// could get an entry back without an IP4 addr, it will come in later as an update
	// so we wait until we find the addr, or timeout
	go func(results <-chan *zeroconf.ServiceEntry, foundEntry chan *zeroconf.ServiceEntry) {
		for e := range results {
			if (len(e.AddrIPv4)) > 0 {
				foundEntry <- e
				cancel()
			}
		}
	}(entries, foundEntry)

	select {
	// we only expect one hub to be found for now now
	case entry = <-foundEntry:
		log.Println("Found bridge")
	case <-ctx.Done():
		log.Println("bridge search timeout, no bridge")
	}
	if entry == nil {
		return nil, errors.New("no bridge found")
	}

	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	b := Bridge{InstanceName: entry.Instance, HostName: entry.HostName, IpAddress: entry.AddrIPv4[0], client: client}
	return &b, nil
}

func (b *Bridge) GetConfig() (*Config, error) {
	resp, err := b.client.Get(fmt.Sprintf("https://%s/api/0/config", b.IpAddress.String()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	config := Config{}
	json.Unmarshal(body, &config)
	return &config, nil

}

func (b *Bridge) Authenticate(appName, instanceName string) error {
	type auth struct {
		DeviceType        string `json:"devicetype"`
		GenerateClientKey bool   `json:"generateclientkey"`
	}

	credRetriever := func(a auth) (*credentials, error) {

		// shortcircuit if we can pull some creds from the environment
		clientKey := os.Getenv("HUE_CLIENT_KEY")
		username := os.Getenv("HUE_USERNAME")

		if clientKey != "" {
			credentials := credentials{clientKey: clientKey, username: username}

			return &credentials, nil
		}

		authJSON, err := json.Marshal(a)
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("https://%s/api", b.IpAddress.String())
		resp, err := b.client.Post(url, "application/json", bytes.NewBuffer(authJSON))

		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var respBody []map[string]map[string]interface{}
		err = json.Unmarshal(body, &respBody)
		if err != nil {
			return nil, err
		}
		log.Println(respBody)
		v := respBody[0]

		if val, ok := v["success"]; ok {
			credentials := credentials{username: val["username"].(string), clientKey: val["clientkey"].(string)}
			return &credentials, nil
		}
		if val, ok := v["error"]; ok {
			if val["description"] == "link button not pressed" {
				return nil, nil
			} else {
				return nil, errors.New(val["description"].(string))
			}
		}

		return nil, errors.New("could not parse auth request")

	}

	a := auth{DeviceType: fmt.Sprintf("%s#%s", appName, instanceName), GenerateClientKey: true}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	started := time.Now()
	for {
		creds, err := credRetriever(a)
		if err != nil {
			return err
		}

		if creds != nil {
			log.Println("clientKey: " + creds.clientKey)
			log.Println("username: " + creds.username)
			b.credentials = creds
			break
		}
		log.Println("Link Button Not Pressed, Please Press To Authenticate...")
		now := <-ticker.C
		if now.Sub(started) > 2*time.Minute {
			return errors.New("timed out waiting for authentication")
		}
	}

	return nil
}

func (b *Bridge) GetLights() ([]Light, error) {
	url := fmt.Sprintf("https://%s/clip/v2/resource/light", b.IpAddress.String())
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add(applicationKey, b.credentials.username)
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	lightResp := lightResponse{}
	err = json.Unmarshal(body, &lightResp)
	if err != nil {
		return nil, err
	}
	if len(lightResp.Errors) > 0 {
		msg := fmt.Sprintf("error getting lights %v", lightResp.Errors)
		return nil, errors.New(msg)
	}
	return lightResp.Data, nil
}

func (b *Bridge) GetLight(id string) (*Light, error) {
	url := fmt.Sprintf("https://%s/clip/v2/resource/light/%s", b.IpAddress.String(), id)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add(applicationKey, b.credentials.username)
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	lightResp := lightResponse{}
	err = json.Unmarshal(body, &lightResp)
	if err != nil {
		return nil, err
	}
	if len(lightResp.Errors) > 0 {
		msg := fmt.Sprintf("error getting lights %v", lightResp.Errors)
		return nil, errors.New(msg)
	}
	return &lightResp.Data[0], nil
}

func (b *Bridge) GetLightByName(name string) (*Light, error) {
	url := fmt.Sprintf("https://%s/clip/v2/resource/light", b.IpAddress.String())
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add(applicationKey, b.credentials.username)
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	lightResp := lightResponse{}
	err = json.Unmarshal(body, &lightResp)
	if err != nil {
		return nil, err
	}
	if len(lightResp.Errors) > 0 {
		msg := fmt.Sprintf("error getting lights %v", lightResp.Errors)
		return nil, errors.New(msg)
	}

	for _, l := range lightResp.Data {
		if l.Metadata.Name == name {
			return &l, nil
		}
	}
	return nil, fmt.Errorf("light named: %s not found", name)
}

func (b *Bridge) ToggleLight(id string, on bool) error {
	url := fmt.Sprintf("https://%s/clip/v2/resource/light/%s", b.IpAddress.String(), id)
	body := map[string]map[string]bool{
		"on": {
			"on": on,
		},
	}
	rb, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(rb))
	req.Header.Add(applicationKey, b.credentials.username)
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not turn light, status code: %s", resp.Status)
	}

	return nil
}

func (b *Bridge) AdjustBrightness(id string, percentage float64) error {
	if percentage < 0 {
		return errors.New("brightness percentage less than 0")
	}
	if percentage > 100 {
		return errors.New("brightness percentage greater than 100")
	}
	url := fmt.Sprintf("https://%s/clip/v2/resource/light/%s", b.IpAddress.String(), id)
	body := map[string]map[string]float64{
		"dimming": {
			"brightness": percentage,
		},
	}
	rb, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(rb))
	req.Header.Add(applicationKey, b.credentials.username)
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not adjust brightness, status code: %s", resp.Status)
	}

	return nil
}

func (b *Bridge) SetColor(id string, c colorful.Color) error {
	x, y := getXy(c)
	url := fmt.Sprintf("https://%s/clip/v2/resource/light/%s", b.IpAddress.String(), id)
	body := map[string]map[string]map[string]float64{
		"color": {
			"xy": {
				"x": x,
				"y": y,
			},
		},
	}
	rb, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(rb))
	req.Header.Add(applicationKey, b.credentials.username)
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not adjust brightness, status code: %s", resp.Status)
	}

	return nil
}

func getXy(c colorful.Color) (float64, float64) {
	// from: https://developers.meethue.com/develop/application-design-guidance/color-conversion-formulas-rgb-to-xy-and-back/#Color-rgb-to-xy
	red := 0.0
	green := 0.0
	blue := 0.0
	if c.R > 0.04045 {
		red = math.Pow((c.Clamped().R+0.055)/(1.0+0.055), 2.4)
	} else {
		red = (c.R / 12.92)
	}
	if c.G > 0.04045 {
		green = math.Pow((c.Clamped().G+0.055)/(1.0+0.055), 2.4)
	} else {
		green = (c.G / 12.92)
	}
	if c.B > 0.04045 {
		blue = math.Pow((c.Clamped().B+0.055)/(1.0+0.055), 2.4)
	} else {
		blue = c.B / 12.92
	}

	x1 := red*0.4124 + green*0.3576 + blue*0.1805
	y1 := red*0.2126 + green*0.7152 + blue*0.0722
	z1 := red*0.0193 + green*0.1192 + blue*0.9505

	x := x1 / (x1 + y1 + z1)
	y := y1 / (x1 + y1 + z1)
	//brightness := y1
	// TODO: calculate closeness to gamut provided by light, for now just...yolo
	return x, y
}
