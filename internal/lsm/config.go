package lsm

type Config struct {
	MemTableSize    int
	MaxLevel        int
	LevelMaxFiles   []int
	TargetFileSize  int
	DataDir         string
}

func DefaultConfig() Config {
	return Config{
		MemTableSize:   1 << 20,
		MaxLevel:       4,
		LevelMaxFiles:  []int{4, 8, 16, 32},
		TargetFileSize: 1 << 19,
		DataDir:        "./lsm_data",
	}
}

func (c *Config) validate() {
	if c.MemTableSize <= 0 {
		c.MemTableSize = 1 << 20
	}
	if c.MaxLevel <= 0 {
		c.MaxLevel = 4
	}
	if len(c.LevelMaxFiles) == 0 {
		c.LevelMaxFiles = []int{4, 8, 16, 32}
	}
	if c.TargetFileSize <= 0 {
		c.TargetFileSize = 1 << 19
	}
	if c.DataDir == "" {
		c.DataDir = "./lsm_data"
	}
	for len(c.LevelMaxFiles) < c.MaxLevel {
		last := c.LevelMaxFiles[len(c.LevelMaxFiles)-1]
		c.LevelMaxFiles = append(c.LevelMaxFiles, last*2)
	}
}
