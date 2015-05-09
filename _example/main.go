package main

import (
	"encoding/base64"
	"fmt"
	"github.com/zemnmez/geothermal"
	"github.com/zemnmez/geothermal/chat"
	"os"
)

func main() {
	if err := do(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

var sessionfile string

func do() (err error) {
	var c geothermal.Client

	if err = c.InteractiveLogin(); err != nil {
		return
	}

	ch, err := chat.New(&c.Client)
	if err != nil {
		return
	}

	for {
		var m []chat.Message
		m, err = ch.Poll()
		if err != nil {
			return
		}

		for _, s := range m {
			switch v := s.(type) {
			case chat.SaytextMessage:
				if v.Self {
					continue
				}
				if err = ch.Say(uint64(geothermal.UserSteamID(v.AccountIDFrom)), base64.StdEncoding.EncodeToString([]byte(v.Text))); err != nil {
					return
				}
			}
		}
	}

	return
}
