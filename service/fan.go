package service

import (
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/hc/service"
)

type Fan struct {
	*service.Service

	On    *characteristic.On
	Speed *characteristic.RotationSpeed
}

func NewFan() *Fan {
	svc := Fan{}
	svc.Service = service.New(service.TypeFan)

	svc.On = characteristic.NewOn()
	svc.AddCharacteristic(svc.On.Characteristic)

	svc.Speed = characteristic.NewRotationSpeed()
	svc.AddCharacteristic(svc.Speed.Characteristic)

	return &svc
}
