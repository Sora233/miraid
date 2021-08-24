package miraid

import (
	"encoding/hex"
	"github.com/Mrs4s/MiraiGo/client"
)

type Bot struct {
	config   *Config
	isQrcode bool

	*client.QQClient
}

func (b *Bot) Init(config *Config) error {
	bot := &Bot{
		config: config,
	}
	switch config.Method {
	case "miraigo":
		if config.MiraiGo.Uin == 0 || len(config.MiraiGo.Password) == 0 {
			bot.QQClient = client.NewClientEmpty()
			bot.isQrcode = true
		} else {
			if config.MiraiGo.UseMd5 {
				hexb, err := hex.DecodeString(config.MiraiGo.Password)
				if err != nil {
					return ErrInvalidPasswordMD5
				}
				if len(hexb) != 16 {
					return ErrInvalidPasswordMD5
				}
				var pwMd5 = [16]byte{}
				for idx, b := range hexb {
					pwMd5[idx] = b
				}
				bot.QQClient = client.NewClientMd5(config.MiraiGo.Uin, pwMd5)

			}
		}
	case "cqhttp":

	default:
		return ErrUnknownMethod
	}
	return nil
}

func (b *Bot) Run() {

}

func NewMiraid() *Bot {
	return new(Bot)
}
