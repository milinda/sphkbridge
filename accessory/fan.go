package accessory

import (
	"github.com/brutella/hc/accessory"
	"github.com/milinda/sphkbridge/service"
)

type Fan struct {
	*accessory.Accessory
	Fan *service.Fan
}

func NewFan(info accessory.Info) *Fan {
	acc := Fan{}
	acc.Accessory = accessory.New(info, accessory.TypeFan)
	acc.Fan = service.NewFan()

	acc.Fan.Speed.SetValue(100)

	acc.AddService(acc.Fan.Service)

	return &acc
}
