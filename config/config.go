package config

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type (
	Psql struct {
		ConnectionParams   string `yaml:"ConnectionParams"`
		MaxOpenConnections int    `yaml:"MaxOpenConnections"`
	}

	Config struct {
		GRPC struct {
			Host string `yaml:"Host"`
		} `yaml:"GRPC"`
		Kafka struct {
			Broker  string `yaml:"Broker"`
			GroupId string `yaml:"GroupId"`
			Topic   string `yaml:"Topic"`
		} `yaml:"Kafka"`
		Cache         Psql   `yaml:"Cache"`
		DataProcessed Psql   `yaml:"DataProcessed"`
		EventsPool    Psql   `yaml:"EventsPool"`
		Decoder       string `yaml:"Decoder"`
		Xxdb          struct {
			Host string `yaml:"Host"`
			User string `yaml:"User"`
			Pass string `yaml:"Pass"`
		} `yaml:"Xxdb"`
		InfluxDb struct {
			Host     string `yaml:"Host"`
			Database string `yaml:"Database"`
		} `yaml:"InfluxDb"`
	}
)

func MustLoad() *Config {
	var (
		_, b, _, _ = runtime.Caller(0)
		basepath   = filepath.Dir(b)
	)

	file, err := ioutil.ReadFile(path.Join(basepath + "/config.yaml"))
	if err != nil {
		log.Fatal(err)
	}

	data := Config{Cache: Psql{MaxOpenConnections: 5},
		DataProcessed: Psql{MaxOpenConnections: 10},
		EventsPool:    Psql{MaxOpenConnections: 10}}
	if err := yaml.Unmarshal(file, &data); err != nil {
		log.Fatal(err)
	}
	return &data
}

func InitLogging(logFile string) {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok { // LOG_LEVEL not set, let's default to debug
		lvl = "debug"
	}

	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.DebugLevel
	}

	logrus.SetLevel(ll) // set global log level

	formatFilePath := func(path string) string {
		arr := strings.Split(path, "/")
		return arr[len(arr)-1]
	}

	prettyfier := func(f *runtime.Frame) (string, string) {
		// wanted file and line to look like this `file="engine.go:141`
		return "", fmt.Sprintf("%s:%d", formatFilePath(f.File), f.Line)
	}

	logPtr := flag.Bool("log", false, "log to file")
	flag.Parse()

	var out io.Writer
	if !*logPtr {
		out = os.Stdout
		logrus.SetFormatter(&logrus.TextFormatter{
			CallerPrettyfier: prettyfier,
		})
	} else {
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat:  "02-01-2006 15:04:05",
			CallerPrettyfier: prettyfier,
		})
		usr, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}

		fname := filepath.Join(usr.HomeDir, logFile)
		out, err = os.OpenFile(fname, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			log.Errorf("%s %s", fname, err)
			os.Exit(1)
		}
	}
	logrus.SetReportCaller(true)
	logrus.SetOutput(out)
}
