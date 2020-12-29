package accessory

import (
	"github.com/brutella/hc/accessory"
	cservice "github.com/milinda/sphkbridge/service"
)

type DimmableLightbulb struct {
	*accessory.Accessory
	Lightbulb *cservice.DimmableLightbulb
}

// NewLightbulb returns an light bulb accessory which one light bulb service.
func NewDimmableLightbulb(info accessory.Info) *DimmableLightbulb {
	acc := DimmableLightbulb{}
	acc.Accessory = accessory.New(info, accessory.TypeLightbulb)
	acc.Lightbulb = cservice.NewDimmableLightbulb()

	acc.Lightbulb.Brightness.SetValue(100)

	acc.AddService(acc.Lightbulb.Service)

	return &acc
}
