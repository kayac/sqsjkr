// sample code how to use sqsjkr
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"runtime"

	"github.com/kayac/sqsjkr"
	"github.com/kayac/sqsjkr/lock"
	"github.com/kayac/sqsjkr/throttle"
)

var (
	confPath    string
	level       string
	showVersion bool
	version     string
	buildDate   string
	profile     string
	region      string
	table       string
	statsSock   string
	statsPort   int
)

func main() {
	flag.StringVar(&confPath, "conf", "/etc/sqsjkr/config.toml", "sqsjkr config file")
	flag.BoolVar(&showVersion, "version", false, "display version")
	flag.StringVar(&level, "log-level", "info", "log level")
	flag.StringVar(&profile, "profile", "", "aws profile")
	flag.StringVar(&region, "region", "", "aws region")
	flag.StringVar(&table, "lock-table", "sqsjkr", "lock & throttle DynamoDB table name")
	flag.StringVar(&statsSock, "stats-socket", "", "sqsjkr stats api socket path")
	flag.IntVar(&statsPort, "stats-port", 0, "sqsjkr stats api port")
	flag.Parse()

	if showVersion {
		fmt.Println("sqsjkr version:", version)
		fmt.Println("build date:", buildDate)
		fmt.Printf("Compiler: %s %s\n", runtime.Compiler, runtime.Version())
		return
	}

	// init Context
	ctx := context.Background()

	// init config
	conf, err := sqsjkr.LoadConfig(confPath)
	if err != nil {
		log.Println(err)
		return
	}

	// overwrite stats socket
	if statsSock != "" {
		if err := conf.SetStatsSocket(statsSock); err != nil {
			panic(err)
		}
	}

	// overwrite stats port
	if statsPort != 0 {
		if err := conf.SetStatsPort(statsPort); err != nil {
			panic(err)
		}
	}

	// overwrite profile
	if profile != "" {
		conf.Account.Profile = profile
	}

	// overwrite region
	if region != "" {
		conf.Account.Region = region
	}

	// init sqsjkr
	sjkr, err := sqsjkr.New(conf)
	if err != nil {
		panic(err)
	}

	// configure Locker
	locker := lock.NewDynamodbLock(
		conf.Account.Profile,
		conf.Account.Region,
		table,
	)
	sjkr.SetLocker(locker)

	// configure throttler
	throttler := throttle.NewDynamodbThrottle(
		ctx,
		conf.Account.Profile,
		conf.Account.Region,
		table,
		sjkr.RetentionPeriod,
	)
	sjkr.SetThrottler(throttler)

	// run sqsjkr
	if err := sqsjkr.Run(ctx, sjkr, level); err != nil {
		log.Println("[error] ", err)
	}
}
