package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zemnmez/geothermal"
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

	var l geothermal.Login
	var username, password string
	fmt.Println("Username:")
	fmt.Scanln(&username)

	fmt.Println("Password:")
	fmt.Scanln(&password)

	l, err = c.Login(username, password)
	if err != nil {
		return
	}

	for !l.Complete {
		if l.Message != "" {
			fmt.Println(l.Message)
		}

		thingsToDo := false
		if l.Captcha != "" {
			thingsToDo = true

			fmt.Println("Complete CAPTCHA", l.CaptchaURL())
			fmt.Scanln(&l.CaptchaSolution)
		}

		if l.SteamGuard {
			thingsToDo = true

			fmt.Println("SteamGuard code:")
			fmt.Scanln(&l.SteamGuardCode)
		}

		if !thingsToDo {
			// un/pw fail
			return errors.New("Failure with no CAPTCHA or SteamGuard to complete. " +
				"You probably got your username or password wrong.")
		}

		if err = l.Attempt(); err != nil {
			return
		}
	}

	//looks like it worked :)
	fmt.Println("Log in successful, saving session")

	f, err := os.Create("session.json")
	if err != nil {
		return
	}

	if err = json.NewEncoder(f).Encode(c); err != nil {
		return
	}

	f.Close()

	return
}
