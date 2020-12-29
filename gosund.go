package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/brutella/hc"
	"github.com/milinda/sphkbridge/accessory"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

type GosundDimmerSwitch struct {
	Id              string
	Accessory       *accessory.DimmableLightbulb
	Transport       hc.Transport
	Power           bool
	Brightness      int
	CommandTopic    string
	BrightnessTopic string
	MqClient        mqtt.Client
}

type GosundDimmerSwitchState struct {
	State      string `json:"state"`
	Brightness int    `json:"Brightness"`
}

func (g *GosundDimmerSwitch) SetBrightness(pct int) error {
	if pct > 100 || pct < 0 {
		return errors.New(fmt.Sprintf("invalid Brightness percentage %d", pct))
	}

	if token := g.MqClient.Publish(g.BrightnessTopic, 0, false, fmt.Sprintf("%d", pct));
		token.Wait() && token.Error() != nil {
		zap.S().Error(token.Error())
		return errors.New(fmt.Sprintf("could not publish to topic %s", g.BrightnessTopic))
	}

	return nil
}

func (g *GosundDimmerSwitch) SetPower(power bool) error {
	var powerStr string
	var state GosundDimmerSwitchState
	if power {
		powerStr = "ON"
	} else {
		powerStr = "OFF"
	}

	if g.Brightness == 0 && power {
		zap.S().Info(fmt.Sprintf("Truning the light %s since brightness is 0.", g.Id))
		state = GosundDimmerSwitchState{
			State: "OFF",
		}
	} else {
		state = GosundDimmerSwitchState{
			State:      powerStr,
			Brightness: int((float64(g.Brightness) / float64(100)) * 255),
		}
	}

	stateMsg, err := json.Marshal(state)

	if err != nil {
		zap.S().Error(err)
		return errors.New("could not convert state to JSON")
	}

	if token := g.MqClient.Publish(g.CommandTopic, 0, false, stateMsg);
		token.Wait() && token.Error() != nil {
		zap.S().Error(token.Error())
		return errors.New(fmt.Sprintf("could not publish to topic %s", g.CommandTopic))
	}

	return nil
}
