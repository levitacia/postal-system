package main

import (
	"postal-system/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}

	postalService := service.NewPostalService(cfg)
	if err != nil {
		return
	}

	postalService.Start()
}
