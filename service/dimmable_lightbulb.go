package service

import (
	"github.com/brutella/hc/service"
	"github.com/brutella/hc/characteristic"
)

type DimmableLightbulb struct {
	*service.Service

	On         *characteristic.On
	Brightness *characteristic.Brightness
}

func NewDimmableLightbulb() *DimmableLightbulb {
	svc := DimmableLightbulb{}
	svc.Service = service.New(service.TypeLightbulb)

	svc.On = characteristic.NewOn()
	svc.AddCharacteristic(svc.On.Characteristic)

	svc.Brightness = characteristic.NewBrightness()
	svc.AddCharacteristic(svc.Brightness.Characteristic)

	return &svc
}
