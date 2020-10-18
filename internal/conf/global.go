package conf

var globalConf *OperatorConfig

// Get the global operator configuration object.
func Get() *OperatorConfig {
	return globalConf
}

// Load the global operator configuration.
func Load(s *Source) error {
	c, err := s.Read()
	if err != nil {
		return err
	}
	globalConf = c
	return nil
}
