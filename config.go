package miraid

type Config struct {
	Method  string `toml:"method"`
	MiraiGo struct {
		Uin      int64  `toml:"uin"`
		Password string `toml:"password"`
		UseMd5   bool   `toml:"use_md5"`
	} `toml:"miraigo"`
	CQHttp struct {
		HttpEndpoint string `toml:"http_endpoint"`
	} `toml:"cqhttp"`
}
