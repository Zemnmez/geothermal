package geothermal

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

//Represents a complete or incomplete Steam login sesssion. If Complete is false, additional
//input may be required. If a CAPTCHA needs completing, Captcha will be a non-empty string.
//If a SteamGuard code is required, SteamGuard will be true.
//
//Once the appropriate fields are filled in, run Login.Attempt() again. See the code of CompleteInteractive
//for an example login completion loop.
type Login struct {
	//Login has completed, no extra input required.
	Complete bool

	encPw        []byte
	username     string
	rsatimestamp string

	//Email which SteamGuard code was emailed to, if SteamGuard is true
	EmailDomain string

	//Server sent messages "please complete the captcha below"
	Message string

	//CAPTCHA ID for captcha that needs completing
	//see CaptchaURL() for the image url.
	//
	//Solution should be placed in CaptchaSolution before attempting
	//again.
	Captcha string
	//Completed solution for the CAPTCHA
	CaptchaSolution string

	//A SteamGuard code is required for the login process to continue
	SteamGuard bool
	//Fill with SteamGuard code to continue login process
	SteamGuardCode string
	//Set to 'human readable name', SteamGuard will record this computer / login by this name.
	ComputerName string

	c *Client
}

func (l Login) CaptchaURL() string {
	return SteamCommunityURL + "/public/captcha.php?gid=" + l.Captcha
}

func (l *Login) Attempt() (err error) {
	var rsp struct {
		Complete bool `json:"login_complete"`
		Success  bool `json:"success"`

		//Captcha
		CaptchaNeeded bool   `json:"captcha_needed"`
		CaptchaGID    string `json:"captcha_gid"`

		//Email stuff
		EmailAuthNeeded bool   `json:"emailauth_needed"`
		EmailDomain     string `json:"emaildomain"`
		EmailSteamID    string `json:"emailsteamid"`

		TransferParameters struct {
			SteamID uint64 `json:"steamid,string"`
			Token   string `json:"token"`
		} `json:"transfer_parameters"`
		Message string `json:"message"`
	}

	r, err := l.c.PostForm(
		SteamCommunityURL+"/login/dologin",
		url.Values{
			"username":          {l.username},
			"password":          {base64.StdEncoding.EncodeToString(l.encPw)},
			"emailauth":         {l.SteamGuardCode},
			"loginfriendlyname": {l.ComputerName},
			"captchaGID":        {l.Captcha},
			"captcha_text":      {l.CaptchaSolution},
			"rsatimestamp":      {l.rsatimestamp},
			"remember_login":    {"false"}, // doesn't matter
		},
	)

	err = json.NewDecoder(r.Body).Decode(&rsp)

	r.Body.Close()
	if err != nil {
		return
	}

	l.Complete = rsp.Complete
	if rsp.CaptchaNeeded {
		l.Captcha = rsp.CaptchaGID
	}

	// if will probably be optimized out? I like the form here
	l.SteamGuard = rsp.EmailAuthNeeded

	l.EmailDomain = rsp.EmailDomain

	l.Message = rsp.Message

	//	println(fmt.Sprintf("rsp: %+v login: %+v", rsp, l))

	//set SessionID
	if l.c.SessionID, err = l.c.sessionID(); err != nil {
		return
	}

	return
}

var GetRSAKeyFailed = errors.New("GetRSAKey returned failure.")

//Returns the RSAKey parameters associated with this username for login.
func (c *Client) getRSAKey(username string) (exp int, mod *big.Int, timestamp string, err error) {

	var r *http.Response

	if r, err = c.PostForm(
		SteamCommunityURL+"/login/getrsakey",
		url.Values{
			"username": {username},
		},
	); err != nil {
		return
	}

	var rsp struct {
		Exp       string `json:"publickey_exp"`
		Mod       string `json:"publickey_mod"`
		Success   bool   `json:"success"`
		Timestamp string `json:"timestamp"`
	}

	err = json.NewDecoder(r.Body).Decode(&rsp)

	if r.Body.Close(); err != nil {
		return
	}

	if !rsp.Success {
		err = GetRSAKeyFailed
		return
	}

	var expB, modB []byte
	expB, err = hex.DecodeString(rsp.Exp)
	if err != nil {
		return
	}
	modB, err = hex.DecodeString(rsp.Mod)
	if err != nil {
		return
	}

	exp = int(new(big.Int).SetBytes(expB).Uint64())
	mod = new(big.Int).SetBytes(modB)
	timestamp = rsp.Timestamp

	return
}

//Attempts a login into a steam account on this Client. Login returns a
//Login value which may represent a complete or incomplete login.
func (c *Client) Login(username, password string) (l Login, err error) {
	if c.Client.Jar == nil {
		c.Client.Jar, err = cookiejar.New(nil)
		if err != nil {
			return
		}
	}

	var (
		exp int
		mod *big.Int
	)

	if exp, mod, l.rsatimestamp, err = c.getRSAKey(username); err != nil {
		return
	}

	if l.encPw, err = rsa.EncryptPKCS1v15(
		rand.Reader,
		&rsa.PublicKey{mod, exp},
		[]byte(password),
	); err != nil {
		return
	}

	l.username = username

	l.c = c

	err = l.Attempt()
	return
}

func input(val interface{}, prompt ...interface{}) (err error) {
	if _, err = fmt.Print(prompt...); err != nil {
		return
	}

	if _, err = fmt.Scanln(val); err != nil {
		return
	}

	return
}

func inputf(format string, val interface{}, prompt ...interface{}) (err error) {
	if _, err = fmt.Printf(format, prompt...); err != nil {
		return
	}

	if _, err = fmt.Scanln(val); err != nil {
		return
	}

	return
}

//Interactively prompts the user "Please complete CAPTCHA" with input from stdin
// to complete the login CAPTCHA.
func (l *Login) PromptCAPTCHA() (err error) {
	return input(&l.CaptchaSolution, "CAPTCHA: ", l.CaptchaURL())
}

//Interactively prompts the user "Please enter SteamGuard code: " with input from stdin
// to pass SteamGuard.
func (l *Login) PromptSteamGuard() (err error) {
	return input(&l.SteamGuardCode, "SteamGuard code: ")
}

var ErrLoginFail = errors.New("failure with no CAPTCHA or SteamGuard to complete, verify username & password")

//Interactively completes the login process, prompting the user to enter SteamGuard and
//CAPTCHA codes where needed.
func (l *Login) CompleteInteractive() (err error) {
	for !l.Complete {
		if l.Message != "" {
			if _, err = fmt.Println(l.Message); err != nil {
				return
			}
		}

		thingsDone := 0
		if l.Captcha != "" {
			thingsDone++

			if err = l.PromptCAPTCHA(); err != nil {
				return
			}
		}

		if l.SteamGuard {
			thingsDone++

			if err = l.PromptSteamGuard(); err != nil {
				return
			}
		}

		if thingsDone < 1 {
			return ErrLoginFail
		}

		if err = l.Attempt(); err != nil {
			return
		}
	}

	return
}

//Interactively completes the Steam login process, prompting the user for missing credentials through
//stdin and stdout as needed.
func (c *Client) InteractiveLogin() (err error) {
	var username string
	if err = input(&username, "Username: "); err != nil {
		return
	}

	if _, err = fmt.Print("Password: "); err != nil {
		return
	}

	var password []byte
	if password, err = terminal.ReadPassword(0); err != nil {
		return
	}

	if _, err = fmt.Println(""); err != nil {
		return
	}

	l, err := c.Login(username, string(password))
	if err != nil {
		return
	}

	if err = l.CompleteInteractive(); err != nil {
		return
	}

	return
}
