package main

import (
	"GMS/handing/login"
	"github.com/obity/properties"
	"log"
)

func main() {
	loginServer := login.Server{}

	loginServer.RunStartupConfigurations()
}

func LoadConfig(file string) *properties.Properties {
	p := properties.NewProperties()
	err := p.LoadFromFile(file)
	if err != nil {
		log.Println(err)
		return nil
	}
	return p
}
