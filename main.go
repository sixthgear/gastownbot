package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

func main() {

	config := new(GastownBotConfig)

	if buf, err := ioutil.ReadFile("./config/gastownbot.json"); err != nil {
		log.Fatalf("Unable to read config file: %v", err)
	} else if err := json.Unmarshal(buf, &config); err != nil {
		log.Fatalf("Unable to parse config: %v", err)
	} else {
		bot := New(config)
		bot.Go()
	}

}
