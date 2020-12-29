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
	lights     map[string]*GosundDimmerSwitch
	pin        string
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

func setupNewGosundDimmerSwitch(config map[string]interface{}) {
	name := fmt.Sprintf("%v", config["name"])
	stateTopic := fmt.Sprintf("%v", config["state_topic"])

	zap.S().Infof("Registering new GosundDimmerSwitch for %s at topic %s", name, stateTopic)

	if token := mqttClient.Subscribe(stateTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
		var gosundDSState *GosundDimmerSwitchState
		var power bool
		var brightness int
		err := json.Unmarshal(msg.Payload(), &gosundDSState)
		if err != nil {
			zap.S().Errorf("Could not unmarshall state %s", msg.Payload())
		} else {
			if gosundDSState.State == "ON" {
				power = true
			} else {
				power = false
			}
			brightness = int((float64(gosundDSState.Brightness) / float64(255)) * 100)

			gosundDS, found := lights[name]

			if found {
				gosundDS.Power = power
				gosundDS.Brightness = brightness
			} else {
				accInfo := accessory.Info{
					Name:         name,
					Manufacturer: "Gosund",
					Model:        "SW2",
				}

				acc := caccessory.NewDimmableLightbulb(accInfo)
				acc.Lightbulb.On.SetValue(power)
				acc.Lightbulb.Brightness.SetValue(brightness)

				tConfig := hc.Config{Pin: pin}
				transport, err := hc.NewIPTransport(tConfig, acc.Accessory)
				if err != nil {
					zap.S().Panic(err)
				}

				go func() {
					transport.Start()
				}()

				go func() {
					uri, _ := transport.XHMURI()
					qrcode.WriteFile(uri, qrcode.Medium, 256, fmt.Sprintf("%s.png", name))
				}()

				gosundDS = &GosundDimmerSwitch{
					Id:              name,
					Accessory:       acc,
					Transport:       transport,
					Power:           power,
					Brightness:      brightness,
					CommandTopic:    fmt.Sprintf("%v", config["command_topic"]),
					BrightnessTopic: fmt.Sprintf("%v/brightness_pct", config["command_topic"]),
					MqClient:        mqttClient,
				}

				lights[name] = gosundDS

				acc.OnIdentify(func() {
					zap.S().Infof("Identifying accessory %s", name)
				})

				acc.Lightbulb.On.OnValueRemoteUpdate(func(power bool) {
					gosundDS.SetPower(power)
				})

				acc.Lightbulb.Brightness.OnValueRemoteUpdate(func(brightness int) {
					gosundDS.SetBrightness(brightness)
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
		if strings.HasPrefix(msg.Topic(), "homeassistant/light/gosundsw2/") {
			var deviceConfig map[string]interface{}

			err = json.Unmarshal(msg.Payload(), &deviceConfig)
			if err == nil {
				_, found := lights[fmt.Sprintf("%v", deviceConfig["name"])]
				if !found {
					setupNewGosundDimmerSwitch(deviceConfig)
				}
			} else {
				zap.S().Errorf("Could not parse message %s", msg.Payload())
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

	lights = map[string]*GosundDimmerSwitch{}
	pin = config.Pin

	zap.S().Infof("HomeKit pin: %s", pin)

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
