package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	caccessory "github.com/milinda/sphkbridge/accessory"
	"github.com/skip2/go-qrcode"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	lights     map[string]*ESPHomeDimmerSwitch
	fans       map[string]*ESPHomeFan
	mqttClient mqtt.Client
)

func createLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, error := config.Build()

	if error != nil {
		log.Panic("Cannot initialize logger.", error)
	}

	return logger
}

func connectMqtt(brokerUrl string, userName string, password string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(brokerUrl)

	if len(userName) > 0 && len(password) > 0 {
		opts.SetUsername(userName)
		opts.SetPassword(password)
	}

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return client, nil
}

func setupNewFan(config map[string]interface{}, c *Configuration) {
	name := fmt.Sprintf("%v", config["name"])
	speedCommandTopic := fmt.Sprintf("%v", config["speed_command_topic"])
	speedStateTopic := fmt.Sprintf("%v", config["speed_state_topic"])
	powerCommandTopic := fmt.Sprintf("%v", config["command_topic"])
	powerStateTopic := fmt.Sprintf("%v", config["state_topic"])

	zap.S().Infof("Registering new ESPHomeFan for %s at topic %s", name, powerStateTopic)

	if token := mqttClient.Subscribe(powerStateTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
		var power = false
		state := string(msg.Payload())

		if state == "ON" {
			power = true
		}

		fan, found := fans[name]

		if found {
			fan.Power = power
			fan.Accessory.Fan.On.SetValue(power)
		} else {
			accInfo := accessory.Info{
				Name:         name,
				Manufacturer: "Treatlife",
				Model:        "DS03",
			}

			acc := caccessory.NewFan(accInfo)
			acc.Fan.On.SetValue(power)

			var tConfig hc.Config
			if c.StorageDir != "" {
				tConfig = hc.Config{Pin: c.Pin, StoragePath: fmt.Sprintf("%s/%s", c.StorageDir, name)}
			} else {
				tConfig = hc.Config{Pin: c.Pin}
			}
			transport, err := hc.NewIPTransport(tConfig, acc.Accessory)
			if err != nil {
				zap.S().Panic(err)
			}

			go func() {
				uri, _ := transport.XHMURI()
				qrcode.WriteFile(uri, qrcode.Medium, 256, fmt.Sprintf("%s.png", name))
				transport.Start()
			}()

			fan := &ESPHomeFan{
				Id:                name,
				Accessory:         acc,
				Transport:         transport,
				Power:             power,
				StateCommandTopic: powerCommandTopic,
				SpeedCommandTopic: speedCommandTopic,
				MqClient:          mqttClient,
			}

			fans[name] = fan

			acc.OnIdentify(func() {
				zap.S().Infof("Identifying accessory %s", name)
			})

			acc.Fan.On.OnValueRemoteUpdate(func(power bool) {
				fan.SetPower(power)
			})

			acc.Fan.Speed.OnValueRemoteUpdate(func(speed float64) {
				fan.SetSpeed(speed)
			})

			if token := mqttClient.Subscribe(speedStateTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
				var speed float64

				fan, found := fans[name]
				if !found {
					zap.S().Warnf("Could not find fan instance with name %s. This should not happen at this state.", name)
				} else {
					speedStr := string(msg.Payload())

					if speedStr == "low" {
						speed = 34.0
					} else if speedStr == "medium" {
						speed = 68.0
					} else if speedStr == "high" {
						speed = 100.0
					} else {
						speed = 0.0
					}

					fan.Speed = speed
					fan.Accessory.Fan.Speed.SetValue(speed)
				}
			}); token.Wait() && token.Error() != nil {
				zap.S().Panicf("Could not subscribe to topic %s", speedStateTopic)
			}
		}
	}); token.Wait() && token.Error() != nil {
		zap.S().Panicf("Could not subscribe to topic %s", powerStateTopic)
	}
}

func setupNewDimmerSwitch(config map[string]interface{}, c *Configuration) {
	name := fmt.Sprintf("%v", config["name"])
	stateTopic := fmt.Sprintf("%v", config["state_topic"])

	zap.S().Infof("Registering new ESPHomeDimmerSwitch for %s at topic %s", name, stateTopic)

	if token := mqttClient.Subscribe(stateTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
		var gosundDSState *ESPHomeDimmerSwitchState
		var brightness int
		var power = true
		err := json.Unmarshal(msg.Payload(), &gosundDSState)
		if err != nil {
			zap.S().Errorf("Could not unmarshall state %s", msg.Payload())
		} else {
			if gosundDSState.State == "ON" {
				power = true
			}

			brightness = int((float64(gosundDSState.Brightness) / float64(255)) * 100)

			dimmerSwitch, found := lights[name]

			if found {
				dimmerSwitch.Power = power
				dimmerSwitch.Brightness = brightness
				dimmerSwitch.Accessory.Lightbulb.On.SetValue(power)
				dimmerSwitch.Accessory.Lightbulb.Brightness.SetValue(brightness)
			} else {
				var accInfo accessory.Info
				var isTreatlife bool = false
				if strings.HasPrefix(stateTopic, "treatlifeds03/") {
					accInfo = accessory.Info{
						Name:         name,
						Manufacturer: "Treatlife",
						Model:        "DS03",
					}
					isTreatlife = true
				} else {
					accInfo = accessory.Info{
						Name:         name,
						Manufacturer: "Gosund",
						Model:        "SW2",
					}
				}

				acc := caccessory.NewDimmableLightbulb(accInfo)
				acc.Lightbulb.On.SetValue(power)
				acc.Lightbulb.Brightness.SetValue(brightness)

				var tConfig hc.Config
				if c.StorageDir != "" {
					tConfig = hc.Config{Pin: c.Pin, StoragePath: fmt.Sprintf("%s/%s", c.StorageDir, name)}
				} else {
					tConfig = hc.Config{Pin: c.Pin}
				}
				transport, err := hc.NewIPTransport(tConfig, acc.Accessory)
				if err != nil {
					zap.S().Panic(err)
				}

				go func() {
					uri, _ := transport.XHMURI()
					qrcode.WriteFile(uri, qrcode.Medium, 256, fmt.Sprintf("%s.png", name))
					transport.Start()
				}()

				dimmerSwitch = &ESPHomeDimmerSwitch{
					Id:              name,
					Accessory:       acc,
					Transport:       transport,
					Power:           power,
					Brightness:      brightness,
					CommandTopic:    fmt.Sprintf("%v", config["command_topic"]),
					BrightnessTopic: fmt.Sprintf("%v/brightness_pct", config["command_topic"]),
					MqClient:        mqttClient,
					IsTreatLife:     isTreatlife,
				}

				lights[name] = dimmerSwitch

				acc.OnIdentify(func() {
					zap.S().Infof("Identifying accessory %s", name)
				})

				acc.Lightbulb.On.OnValueRemoteUpdate(func(power bool) {
					dimmerSwitch.SetPower(power)
				})

				acc.Lightbulb.Brightness.OnValueRemoteUpdate(func(brightness int) {
					dimmerSwitch.SetBrightness(brightness)
				})
			}
		}
	}); token.Wait() && token.Error() != nil {
		zap.S().Panicf("Could not subscribe to topic %s", stateTopic)
	}

}

func initialize(c *Configuration) {
	var err error
	// Setup logging
	var logger = createLogger()
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	// Setup MQTT connection
	mqttClient, err = connectMqtt(c.Broker.Url, c.Broker.UserName, c.Broker.Password)

	if err != nil {
		zap.S().Panicf("Cannot connect to MQTT broker at %s", c.Broker.Url)
	}

	if token := mqttClient.Subscribe("homeassistant/#", 0, func(client mqtt.Client, msg mqtt.Message) {
		if strings.HasPrefix(msg.Topic(), "homeassistant/light/gosundsw2/") ||
			strings.HasPrefix(msg.Topic(), "homeassistant/light/treatlifeds03") {
			var dimmerConfig map[string]interface{}

			err = json.Unmarshal(msg.Payload(), &dimmerConfig)
			if err == nil {
				_, found := lights[fmt.Sprintf("%v", dimmerConfig["name"])]
				if !found {
					go setupNewDimmerSwitch(dimmerConfig, c)
				}
			} else {
				zap.S().Errorf("could not parse message %s", msg.Payload())
			}
		} else if strings.HasPrefix(msg.Topic(), "homeassistant/fan/treatlifeds03") {
			var fanConfig map[string]interface{}

			err = json.Unmarshal(msg.Payload(), &fanConfig)
			if err == nil {
				_, found := fans[fmt.Sprintf("%v", fanConfig["name"])]
				if !found {
					go setupNewFan(fanConfig, c)
				}
			} else {
				zap.S().Errorf("could not parse message %s", msg.Payload())
			}
		}
	}); token.Wait() && token.Error() != nil {
		zap.S().Panic(token.Error())
	}
}

func main() {
	var config *Configuration
	var err error

	configPath := flag.String("config-path", "", "Configuration file path")
	flag.Parse()

	if configPath != nil && len(*configPath) > 0 {
		config, err = ParseConfig(*configPath)
		if err != nil {
			zap.S().Panic(err)
		}
	} else {
		zap.S().Info("Using default configuration.")
		config = DefaultConfig()
	}

	lights = map[string]*ESPHomeDimmerSwitch{}
	fans = map[string]*ESPHomeFan{}

	zap.S().Infof("HomeKit pin: %s", config.Pin)

	initialize(config)

	hc.OnTermination(func() {
		for _, light := range lights {
			<-light.Transport.Stop()
		}

		time.Sleep(500 * time.Millisecond)
		os.Exit(1)
	})

	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)
	<-quitChannel

	zap.S().Info("sphkbridge exiting...")
}
