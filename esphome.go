package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/brutella/hc"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/milinda/sphkbridge/accessory"
	"go.uber.org/zap"
)


type ESPHomeFan struct {
	Id                string
	Accessory         *accessory.Fan
	Transport         hc.Transport
	Power             bool
	Speed             float64
	SpeedCommandTopic string
	StateCommandTopic string
	MqClient          mqtt.Client
}
type ESPHomeDimmerSwitch struct {
	Id              string
	Accessory       *accessory.DimmableLightbulb
	Transport       hc.Transport
	IsTreatLife     bool
	Power           bool
	Brightness      int
	CommandTopic    string
	BrightnessTopic string
	MqClient        mqtt.Client
}

type ESPHomeDimmerSwitchState struct {
	State      string `json:"state"`
	Brightness int    `json:"brightness"`
}

func (g *ESPHomeDimmerSwitch) SetBrightness(pct int) error {
	if pct > 100 || pct < 0 {
		return errors.New(fmt.Sprintf("invalid Brightness percentage %d", pct))
	}

	if g.IsTreatLife {
		var powerStr string
		if g.Power {
			powerStr = "ON"
		} else {
			powerStr = "OFF"
		}

		state := ESPHomeDimmerSwitchState{State: powerStr, Brightness: int((float64(pct) / float64(100)) * 255)}
		stateMsg, err := json.Marshal(state)
		if err != nil {
			zap.S().Error(err)
			return errors.New("could not convert state to JSON")
		}

		zap.S().Info(stateMsg)
		if token := g.MqClient.Publish(g.CommandTopic, 0, false, stateMsg);
			token.Wait() && token.Error() != nil {
			zap.S().Error(token.Error())
			return errors.New(fmt.Sprintf("could not publish to topic %s", g.CommandTopic))
		}

		return nil
	}

	if token := g.MqClient.Publish(g.BrightnessTopic, 0, false, fmt.Sprintf("%d", pct));
		token.Wait() && token.Error() != nil {
		zap.S().Error(token.Error())
		return errors.New(fmt.Sprintf("could not publish to topic %s", g.BrightnessTopic))
	}

	return nil
}

func (g *ESPHomeDimmerSwitch) SetPower(power bool) error {
	var powerStr string
	var state ESPHomeDimmerSwitchState
	if power {
		powerStr = "ON"
	} else {
		powerStr = "OFF"
	}

	if g.Brightness == 0 && power {
		zap.S().Info(fmt.Sprintf("Truning the light %s since brightness is 0.", g.Id))
		state = ESPHomeDimmerSwitchState{
			State: "OFF",
		}
	} else {
		state = ESPHomeDimmerSwitchState{
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

func (g *ESPHomeFan) SetSpeed(speed float64) error {
	var speedStr string

	if speed <= 34 {
		speedStr = "low"
	} else if speed > 34 && speed <= 68 {
		speedStr = "medium"
	} else {
		speedStr = "high"
	}

	if token := g.MqClient.Publish(g.SpeedCommandTopic, 0, false, speedStr);
		token.Wait() && token.Error() != nil {
		zap.S().Error(token.Error())
		return errors.New(fmt.Sprintf("could not publish to topic %s", g.SpeedCommandTopic))
	}

	return nil
}

func (g *ESPHomeFan) SetPower(power bool) error {
	var powerStr string

	if power {
		powerStr = "ON"
	} else {
		powerStr = "OFF"
	}

	if token := g.MqClient.Publish(g.StateCommandTopic, 0, false, powerStr);
		token.Wait() && token.Error() != nil {
		zap.S().Error(token.Error())
		return errors.New(fmt.Sprintf("could not publish to topic %s", g.StateCommandTopic))
	}

	return nil
}
