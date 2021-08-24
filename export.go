package miraid

var ebot = NewMiraid()

func Init(config *Config) {
	ebot.Init(config)
}

func Run() {
	ebot.Run()
}