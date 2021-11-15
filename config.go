package miraid

import "github.com/Mrs4s/go-cqhttp/modules/config"

type Config struct {
	Method  string `yaml:"method"`
	MiraiGo struct {
		Uin      int64  `yaml:"uin"`
		Password string `yaml:"password"`
	} `yaml:"miraigo"`
	CQHTTP struct {
		HTTP      config.HTTPServer
		WSReverse config.WebsocketReverse
	} `yaml:"cqhttp"`
}
