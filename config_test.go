package sqsjkr

import (
	"os"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("AWS_REGION", "ap-northeast-1")
	m.Run()
}

func TestLoadConfig(t *testing.T) {
	expect := &Config{
		Account: AccountSection{
			Profile: "test-profile",
			ID:      "12345678",
			Region:  "ap-northeast-1",
		},
		SQS: SQSSection{
			QueueName: "test_queue",
		},
		Kicker: KickerSection{
			MaxConcurrentNum: 5,
			Trigger:          "./test/trigger_test.sh",
			StatsPort:        8061,
		},
	}

	conf, err := LoadConfig("./test/sqsjkr.toml")
	if err != nil {
		t.Errorf("Load config error: %s", err)
	}

	if !reflect.DeepEqual(conf, expect) {
		t.Errorf("Unexpected load config data: got=%v, expect=%v", conf, expect)
	}
}

func TestUseSocketAndPort(t *testing.T) {
	_, err := LoadConfig("./test/use_unix_domain_and_tcp.toml")
	if err == nil {
		t.Error("should be invalid config becase to use unix domain socket and http listener")
	}
}

func TestNoRequiredParamsLoadConfig(t *testing.T) {
	conf, err := LoadConfig("./test/no_required_params.toml")

	if err == nil {
		t.Errorf("conf.SQS.QueueName:%s, conf.Account.ID:%s, expect is conf.SQS.QueueNam and conf.Account.ID are empty",
			conf.SQS.QueueName, conf.Account.ID)
	}
}

func TestSetAWSAccount(t *testing.T) {
	c := NewConfig()
	c.SetAWSAccount("123456789", "default", "ap-northeast-1")

	if c.Account.ID != "123456789" {
		t.Errorf("failed to set aws account id: got=%s, expected=123456789", c.Account.ID)
	}

	if c.Account.Region != "ap-northeast-1" {
		t.Errorf("failed to set aws account region: got=%s, expected=ap-northeast-1", c.Account.Region)
	}
}

func TestSetSQSQueue(t *testing.T) {
	c := NewConfig()
	c.SetSQSQueue("test_queue_name")

	if c.SQS.QueueName != "test_queue_name" {
		t.Errorf("failed to set SQS queue name: got=%s, expected=test_queue_name", c.Account.ID)
	}
}

func TestSetKickerConfig(t *testing.T) {
	c := NewConfig()
	c.SetKickerConfig(5, "echo hello")

	if c.Kicker.MaxConcurrentNum != 5 {
		t.Errorf("failed to set kicker max_concurrent_num: got=%d, expected=5", c.Kicker.MaxConcurrentNum)
	}

	if c.Kicker.Trigger != "echo hello" {
		t.Errorf("failed to set kicker trigger: got=%s, expected=echo hello", c.Kicker.Trigger)
	}
}

func TestSetConcurrentNum(t *testing.T) {
	c := NewConfig()
	c.SetConcurrentNum(5)

	if c.Kicker.MaxConcurrentNum != 5 {
		t.Errorf("failed to set kicker max_concurrent_num: got=%d, expected=5", c.Kicker.MaxConcurrentNum)
	}
}

func TestSetTriggerCommand(t *testing.T) {
	c := NewConfig()
	c.SetTriggerCommand("echo hello")

	if c.Kicker.Trigger != "echo hello" {
		t.Errorf("failed to set kicker trigger: got=%s, expected=echo hello", c.Kicker.Trigger)
	}
}
