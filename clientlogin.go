package geothermal

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type Login struct {
	encPw        []byte
	username     string
	rsatimestamp string

	//hint
	EmailDomain string
	Message     string

	Complete bool

	Captcha         string
	CaptchaSolution string

	SteamGuard     bool
	SteamGuardCode string
	ComputerName   string

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

//Makes a login attempt
//
//	l, err := c.Login("dave", "letmein")
//	if err != nil {
//		panic(err)
//	}
//
//	if l.Captcha {
//		l.CompleteCaptcha("WORDS")
//	}
//
//	if l.SteamGuard {
//		l.SteamGuard("CODE")
//	}
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
