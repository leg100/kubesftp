package internal

func Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return getOrCreateHostKeys(cfg)
}
