package main

import (
	"GMS/handing/login"
)

func main() {
	loginServer := login.Server{}

	loginServer.RunStartupConfigurations()
}
