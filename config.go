package sqsjkr

import (
	"fmt"

	"github.com/kayac/go-config"
)

// Config is the sqsjkr config
type Config struct {
	Account AccountSection `toml:"account"`
	Kicker  KickerSection  `toml:"kicker"`
	SQS     SQSSection     `toml:"sqs"`
}

// AccountSection is aws account information
type AccountSection struct {
	Profile string `toml:"profile"`
	ID      string `toml:"id"`
	Region  string `toml:"region"`
}

// KickerSection is the config of command kicker
type KickerSection struct {
	MaxConcurrentNum int    `toml:"max_concurrent_num"`
	Trigger          string `toml:"life_time_trigger"`
	StatsPort        int    `toml:"stats_port"`
	StatsSocket      string `toml:"stats_socket"`
}

// SQSSection is the AWS SQS configure
type SQSSection struct {
	QueueName string `toml:"queue_name"`
}

// NewConfig create sqsjkr config
func NewConfig() *Config {
	return &Config{
		Account: AccountSection{},
		Kicker:  KickerSection{},
		SQS:     SQSSection{},
	}
}

// SetAWSAccount set aws account
func (c *Config) SetAWSAccount(id, profile, region string) {
	c.Account.ID = id
	c.Account.Region = region
	c.Account.Profile = profile
}

// SetSQSQueue set sqs queue name
func (c *Config) SetSQSQueue(qname string) {
	c.SQS.QueueName = qname
}

// SetKickerConfig set kicker config
func (c *Config) SetKickerConfig(num int, trigger string) {
	c.Kicker.MaxConcurrentNum = num
	c.Kicker.Trigger = trigger
}

// SetConcurrentNum set number of concurrent to execute job
func (c *Config) SetConcurrentNum(num int) {
	c.Kicker.MaxConcurrentNum = num
}

// SetTriggerCommand set trigger command
func (c *Config) SetTriggerCommand(trigger string) {
	c.Kicker.Trigger = trigger
}

// SetStatsPort set stats api port number
func (c *Config) SetStatsPort(port int) error {
	if c.Kicker.StatsSocket != "" {
		return fmt.Errorf("Already set stats api unix domain socket. if you use tcp listener, unset StatsSocket.")
	}
	c.Kicker.StatsPort = port
	return nil
}

// SetStatsSocket set unix domain socket path
func (c *Config) SetStatsSocket(sock string) error {
	if c.Kicker.StatsPort != 0 {
		return fmt.Errorf("Already set stats api port number. if you use unix domain socket, unset StatsPort.")
	}
	c.Kicker.StatsSocket = sock
	return nil
}

// LoadConfig loads config file by config file path
func LoadConfig(path string) (*Config, error) {
	var conf Config
	if err := config.LoadWithEnvTOML(&conf, path); err != nil {
		return nil, err
	}

	// Set Default Value if null
	if conf.Kicker.MaxConcurrentNum == 0 {
		conf.Kicker.MaxConcurrentNum = DefaultMaxCocurrentNum
	}

	return &conf, (&conf).Validate()
}

// Validate config validation
func (c *Config) Validate() error {
	if c.SQS.QueueName == "" {
		return fmt.Errorf("queue_name is required")
	}

	if c.Account.ID == "" {
		return fmt.Errorf("aws account id is required")
	}

	if c.Account.Region == "" {
		return fmt.Errorf("aws region is required")
	}

	if c.Kicker.StatsPort != 0 && c.Kicker.StatsSocket != "" {
		return fmt.Errorf("could not specify both stats api port and unix domain socket")
	}

	return nil
}
