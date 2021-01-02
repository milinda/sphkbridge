package main

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"io/ioutil"
)

type Configuration struct {
	Name       string  `hcl:"name"`
	Pin        string  `hcl:"pin"`
	StorageDir string  `hcl:"storage-dir,optional"`
	Broker     *Broker `hcl:"broker,block"`
}

type Broker struct {
	Url      string `hcl:"url"`
	UserName string `hcl:"username,optional"`
	Password string `hcl:"password,optional"`
}

func ParseConfig(configPath string) (c *Configuration, err error) {
	var diags hcl.Diagnostics

	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	file, diags := hclsyntax.ParseConfig(content, configPath, hcl.Pos{Line: 1, Column: 1})
	if diags != nil && diags.HasErrors() {
		return nil, fmt.Errorf("config parse: %w", diags)
	}

	c = &Configuration{}

	diags = gohcl.DecodeBody(file.Body, nil, c)
	if diags != nil && diags.HasErrors() {
		return nil, fmt.Errorf("config parse: %w", diags)
	}

	return c, nil
}

func DefaultConfig() *Configuration {
	return &Configuration{
		Name: "homekit-esphome-bridge",
		Pin:  "865369",
		Broker: &Broker{
			Url:      "tcp://localhost:1883",
			UserName: "",
			Password: "",
		},
	}
}
