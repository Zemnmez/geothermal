package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"regexp"
)

const debug = true

var reAuthToken = regexp.MustCompile(`CWebAPI\s*\(\s*(?:[^,]+,){2}\s*"([0-9a-f]{32})"\s*\)`)

var ErrNoAuthToken = errors.New("steam chat: could not retrive chat auth token, the format may have changed or you may not be logged in")

//Retrieves a chat access token from a Steam authenticated HTTP client.
func ChatAccessToken(c *http.Client) (accessToken string, err error) {
	chatPage, err := c.Get("http://steamcommunity.com/chat")
	if err != nil {
		return
	}

	chatPageBt, err := ioutil.ReadAll(chatPage.Body)
	chatPage.Body.Close()

	token := reAuthToken.FindSubmatch(chatPageBt)
	if token == nil {

		err = ErrNoAuthToken
		return
	}

	accessToken = string(token[1])

	return
}

type Chat struct {
	Steamid      uint64
	Error        error
	Umqid        uint64
	Timestamp    uint
	UtcTimestamp uint
	Message      uint
	Push         uint
	AccessToken  string
	Pollid       uint

	c *http.Client
}

type Message interface {
	MessageType() MessageType
	MessageTimestamp() uint
}

type MessageType uint

const (
	_ MessageType = iota
	Saytext
	Typing
	Personastate
	Leftconversation
)

var (
	_ Message = BaseMessage{}
	_ Message = TypingMessage{}
	_ Message = PersonastateMessage{}
)

func (BaseMessage) MessageType() MessageType { return 0 }
func (m BaseMessage) MessageTimestamp() uint { return m.Timestamp }

type BaseMessage struct {
	Type          string
	Timestamp     uint   `json:"timestamp"`
	UtcTimestamp  uint   `json:"utc_timestamp"`
	AccountIDFrom uint32 `json:"accountid_from"`
}

func (LeftconversationMessage) MessageType() MessageType { return Leftconversation }

type LeftconversationMessage struct {
	BaseMessage
}

func (TypingMessage) MessageType() MessageType { return Typing }

type TypingMessage struct {
	BaseMessage
}

func (SaytextMessage) MessageType() MessageType { return Saytext }

type SaytextMessage struct {
	BaseMessage
	Text string
	Self bool
}

func (PersonastateMessage) MessageType() MessageType { return Personastate }

type PersonastateMessage struct {
	BaseMessage
	PersonaName  string `json:"persona_name"`
	Personastate uint   `json:"persona_state"`
}

func New(c *http.Client) (ch Chat, err error) {
	if ch.AccessToken, err = ChatAccessToken(c); err != nil {
		return
	}

	ch.c = c

	if err = ch.login(); err != nil {
		return
	}

	return
}

func debugRequest(r *http.Response) {
	b, err := httputil.DumpResponse(r, true)
	if err != nil {
		panic(err)
	}

	b2, err := httputil.DumpRequest(r.Request, true)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+q %+q", b, b2)
}

func responseError(rsi *http.Response, erri error) (rs *http.Response, err error) {
	rs = rsi
	err = erri
	if err != nil {
		return
	}

	if rs.StatusCode != 200 {
		err = errors.New("steam chat: " + rs.Status)
	}

	return
}

func (c *Chat) login() (err error) {
	rs, err := responseError(c.c.PostForm(
		"https://api.steampowered.com/ISteamWebUserPresenceOAuth/Logon/v0001/",
		url.Values{"access_token": {c.AccessToken}},
	))

	//debugRequest(rs)

	if err != nil {
		return
	}

	var j struct {
		Steamid      uint64 `json:"steamid,string"`
		Error        string `json:"error"`
		Umqid        uint64 `json:"umqid,string"`
		Timestamp    uint   `json:"timestamp"`
		UtcTimestamp uint   `json:"utc_timestamp"`
		Message      uint   `json:"message"`
		Push         uint   `json:"push"`
	}

	if err = json.NewDecoder(rs.Body).Decode(&j); err != nil {
		return
	}

	c.Steamid = j.Steamid

	if j.Error != "OK" {
		return errors.New(j.Error)
	}

	c.Umqid = j.Umqid

	c.Timestamp = j.Timestamp

	c.UtcTimestamp = j.UtcTimestamp

	c.Message = j.Message

	c.Push = j.Push

	return
}

type InvalidMessage string

func (i InvalidMessage) Error() string { return "steam chat: invalid message type " + string(i) }

func (c *Chat) Say(sid uint64, text string) (err error) {
	rs, err := responseError(c.c.PostForm(
		"https://api.steampowered.com/ISteamWebUserPresenceOAuth/Message/v0001/",
		url.Values{
			"umqid":        {fmt.Sprint(c.Umqid)},
			"access_token": {c.AccessToken},
			"text":         {text},
			"type":         {"saytext"},
			"steamid_dst":  {fmt.Sprint(sid)},
		},
	))

	if err != nil {
		return
	}

	var j struct {
		Error string
	}

	if err = json.NewDecoder(rs.Body).Decode(&j); err != nil {
		return
	}

	rs.Body.Close()

	if j.Error != "OK" {
		return errors.New("steam chat: " + j.Error)
	}

	return

}

func (c *Chat) Poll() (ms []Message, err error) {
	rs, err := responseError(c.c.PostForm(
		"https://api.steampowered.com/ISteamWebUserPresenceOAuth/Poll/v0001/",
		url.Values{
			"umqid":          {fmt.Sprint(c.Umqid)},
			"message":        {fmt.Sprint(c.Message)},
			"pollid":         {fmt.Sprint(c.Pollid)},
			"sectimeout":     {"35"},
			"secidletime":    {"0"}, // get rekt volvo
			"use_accountids": {"1"},
			"access_token":   {c.AccessToken},
		},
	))

	if err != nil {
		return
	}

	var j struct {
		Messages    []json.RawMessage
		Messagelast uint
		Error       string
	}

	if err = json.NewDecoder(rs.Body).Decode(&j); err != nil {
		return
	}

	rs.Body.Close()

	c.Pollid++

	c.Message = j.Messagelast // thanks volvo

	if j.Error != "OK" {
		err = errors.New("steam chat: " + j.Error)
		return
	}

	ms = make([]Message, len(j.Messages))

	for i, v := range j.Messages {
		// parse as generic message
		var b BaseMessage
		if err = json.Unmarshal([]byte(v), &b); err != nil {
			return
		}

		var m Message
		switch b.Type {
		case "saytext":
			m = &SaytextMessage{}
		case "my_saytext":
			m = &SaytextMessage{Self: true}
		case "typing":
			m = &TypingMessage{}
		case "personastate":
			m = &PersonastateMessage{}
		case "leftconversation":
			m = &LeftconversationMessage{}
		default:
			/*	err = InvalidMessage(fmt.Sprintf("%+q %+q", b.Type, v))
				return */
			fmt.Println(InvalidMessage(fmt.Sprintf("%+q %+q", b.Type, v)))
		}

		if err = json.Unmarshal([]byte(v), m); err != nil {
			return
		}

		ms[i] = reflect.Indirect(reflect.ValueOf(m)).Interface().(Message)
	}

	return
}
