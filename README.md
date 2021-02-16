[![Build Status](https://travis-ci.org/kayac/sqsjkr.svg?branch=master)](https://travis-ci.org/kayac/sqsjkr)

# sqsjkr
SQS Job Kicker is the simple worker to invoke command from SQS message

## Install

```
go get github.com/kayac/sqsjkr
```

## Usage

```go
package main
import (
    "context"
    "log"

    "github.com/kayac/sqsjkr"
)

func main() {
    // config
    conf := sqsjkr.NewConfig()
    conf.SetAWSAccount("aws_account_id", "aws_profile", "ap-northeast-1")
    conf.SetSQSQueue("your sqs queue name")
    conf.SetConcurrentNum(5)
    conf.SetTriggerCommand("echo trigger")

    // run sqsjkr
    ctx := context.Background()
    sjkr := sqsjkr.New(conf)
    if err := sqsjkr.Run(ctx, sjkr, "debug"); err != nil {
        log.Println("[error] ", err)
    }
}
```

## Config

- [account] section

params  | type   | description
------- | ------ | -----------------------
id      | string | AWS account id
profile | string | AWS profile name
region  | string | AWS region

- [sqs] section

params      | type   | description
----------- | ------ | ------------------------------------------
queue\_name | string | AWS SQS queue name

- [kicker] section

params               | type     | description
-------------------- | -------- | ------------------------------------------
max\_concurrent\_num | integer  | number of jobs concurrency
life\_time\_trigger  | string   | trigger command when to pass the lifetime
stats\_port          | integer  | port number of sqsjkr stats

You can load config by toml format file:

```toml
[account]
id = "12345678"
profile = "default"
region = "ap-northeast-1"

[sqs]
queue_name = "sqsjkr_queue"

[kicker]
max_concurrent_num = 5
life_time_trigger = "echo 'stdin!' | ./test/trigger_test.sh"
stats_port = 8061
```

```go
conf := sqsjkr.LoadConfig("/path/to/toml")
```

## Job definition


params            | type              | description
----------------- | ----------------- | -------------------------------------------------------------------------------------------------------------------
command           | string            | job command
env               | map               | environment variables
event\_id         | string            | job event uniq name (for example, AWS CloudWatch Event Scheduler ID(Name)).
life\_time        | integer or string | integer is fixed by second unit. string format requires unit name such as 'm', 's', and so on (e.g. 1s, 1m, 1h).
lock\_id          | string            | locks another job
abort\_if\_locked | bool              | if job is locked by lock\_id, new job give up without retry.
disable\_life\_time\_trigger | bool   | disable lifetime trigger even though a job is over the lifetime (default false).

- example:

```json
{
    "command": "echo 'hello sqsjkr!'",
    "env": {
        "PATH": "/usr/local/bin/:/usr/bin/:/sbin/:/bin",
        "MAILTO": "example@example.com"
    },
    "event_id": "cloudwatch_event_schedule_id_name",
    "lock_id": "lock_id",
    "life_time": "1m",
    "abort_if_locked": false
}
```

### LifeTime
The job waits for `life_time` if the other job which is same `lock_id` is executing. So, if a job requires too many time to process and don't want to execute frequently in short term, job should be set the proper `life_time`.

## Locker
SQS Job Kicker provides Locker interface which is like a feature of 'setlock' to avoid to execute same `lock_id`. sqsjkr package's sample uses DynamoDB as Locker backend. Show the following Locker interface:

```go
type Locker interface {
	Lock(string, string) error
	Unlock(string) error
}
```

You can set your custom Locker by `SetLocker(locker Locker)`:
```go
mylocker := NewMyLocker() // Your Locker
sjkr.SetLocker(mylocker)
```

## Throttler
sqsjkr provides Throttler interface which avoid to execute duplicated job. This repository includes the Throttler sample using DynamoDB.
Show following Throttler interface:

```go
type Locker interface {
	Set(string, string) error
	UnSet(string) error
}
```

You can set your custom Throttler by `SetThrottler(th Throttler)`:
```go
myThr := NewMyThrottler() // Your Throttler
sjkr.SetLocker(myThr)
```

## Stats HTTP endpoint

sqsjkr runs a HTTP server on port 8061 to get stats of myself.


```console
$ curl -s localhost:8061/stats/metrics/v2
{
  "workers": {
    "busy": 4,
    "idle": 6
  },
  "invocations": {
    "succeeded": 10,
    "failed": 2,
    "errored": 3
  }
}
```

## LICENSE

MIT
