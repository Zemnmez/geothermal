//Package geothermal implements a fast simple and stable Steam trading and chat bot.
package geothermal

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

var (
	SteamCommunityURL = "https://steamcommunity.com"
	steamCommunityURL *url.URL
)

//A Client is a Steam Community session.
//
//Remember to set Client.Timeout!
type Client struct {
	http.Client
	SessionID string
}

func (c Client) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Jar.Cookies(steamCommunityURL))
}

func (c *Client) UnmarshalJSON(b []byte) (err error) {
	if c.Client.Jar == nil {
		if c.Client.Jar, err = cookiejar.New(nil); err != nil {
			return
		}
	}

	var co []*http.Cookie
	if err = json.Unmarshal(b, &co); err != nil {
		return
	}

	c.Client.Jar.SetCookies(steamCommunityURL, co)

	return
}

var NoSID = errors.New("could not get sessionid, ensure logged in")

func (c Client) sessionID() (sid string, err error) {
	for i := 0; i < 2; i++ {
		for _, v := range c.Client.Jar.Cookies(steamCommunityURL) {
			if v.Name == "sessionid" {
				return url.QueryUnescape(v.Value)
			}
		}

		//Attempt to get cookie set
		c.Get(SteamCommunityURL)
	}

	err = NoSID

	return
}

func (c Client) Logout() (err error) {
	_, err = c.PostForm(
		SteamCommunityURL+"/login/logout",
		url.Values{"sessionid": {c.SessionID}},
	)

	if err != nil {
		return
	}

	return
}

func init() {
	var err error
	if steamCommunityURL, err = url.Parse(SteamCommunityURL); err != nil {
		panic(err)
	}
}
