package geothermal

import (
	"encoding/xml"
	"net/http"
)

func Friends(c *http.Client, user string) (i []uint64, err error) {
	var x struct {
		Friends []uint64 `xml:"friends>friend"`
	}

	rs, err := c.Get(SteamCommunityURL + "/" + user + "/friends?xml=1")
	if err != nil {
		return
	}

	if err = xml.NewDecoder(rs.Body).Decode(&x); err != nil {
		return
	}

	rs.Body.Close()

	i = x.Friends

	return
}

func (c *Client) Friends() ([]uint64, error) {
	return Friends(&c.Client, "my")
}
