package geothermal

import (
	"fmt"
)

//A SteamID64 is an unsigned 64 bit integer that
//can represent practically any entity on Steam.
type SteamID uint64

type Universe uint8

const (
	Unspecified Universe = iota
	Public
	Beta
	Internal
	Dev
	RC
)

var universeMap = map[Universe]string{
	Unspecified: "Unspecified",
	Public:      "Public",
	Beta:        "Beta",
	Internal:    "Internal",
	Dev:         "Dev",
	RC:          "RC",
}

func (u Universe) String() string { return universeMap[u] }

type AccountType uint8

const (
	InvalidAccountType AccountType = iota
	Individual
	Multiseat
	GameServer
	AnonGameServer
	Pending
	ContentServer
	Clan
	Chat
	//P2P SuperSeeder
	SuperSeeder
	AnonUser
)

var accountTypeMap = map[AccountType]string{
	InvalidAccountType: "InvalidAccountType",
	Individual:         "Individual",
	Multiseat:          "Multiseat",
	GameServer:         "GameServer",
	AnonGameServer:     "AnonGameServer",
	Pending:            "Pending",
	ContentServer:      "ContentServer",
	Clan:               "Clan",
	Chat:               "Chat",
	//P2P SuperSeeder
	SuperSeeder: "SuperSeeder",
	AnonUser:    "AnonUser",
}

func (a AccountType) String() string { return accountTypeMap[a] }

func (a AccountType) Letter() string {
	switch a {
	case InvalidAccountType:
		return "I"
	case Individual:
		return "U"
	case Multiseat:
		return "M"
	case GameServer:
		return "G"
	case AnonGameServer:
		return "A"
	case Pending:
		return "P"
	case ContentServer:
		return "C"
	case Clan:
		return "g"
	case Chat:
		return "c"
	case SuperSeeder, AnonUser:
		return ""
	default:
		panic(fmt.Errorf("Unknown AccountType %d", a))
	}
}

func NewID64(id uint32, instance uint8, t AccountType, u Universe) (s SteamID) {
	//The low 32 bits represents the ID number for the account (the "account number").
	s = SteamID(id) |
		//The next 20 bits represents the instance of the account.
		//It is usually set to 1 for user accounts.
		(SteamID(instance) << 32) |
		//The next 4 bits represents the type of account.
		(SteamID(t) << 52) |
		//The next 8 bits represents the "Universe" the steam account belongs to.
		(SteamID(u) << 56)
	return
}

func UserSteamID(id uint32) SteamID {
	return NewID64(id, 1, Individual, Public)
}

func GroupSteamID(id uint32) SteamID {
	return NewID64(id, 0, Clan, Public)
}

func (s SteamID) Decompose() (id uint32, instance uint8, t AccountType, u Universe) {
	//32 bits
	id = uint32(s & 0xffffffff)

	c := s >> 32
	//20 bits
	instance = uint8(c & 0x3ff)

	c = c >> 20
	//4 bits
	t = AccountType(c & 0xf)

	c = c >> 4
	//8 bits
	u = Universe(c & 0xff)

	return
}
