package main

// Programe de surveillance
// Ce projet s'inspire de https://github.com/a-bali/janitor.
// Version
// Tester lecture mqtt

import (
	
	//"context"
	_ "embed"
	//"encoding/json"
	"fmt"
	//"hash/fnv"
	"io/ioutil"
	//"math"
	//"net/http"
	//"net/url"
	"os"
	//"os/exec"
	//"runtime"
	//"strconv"
	//"strings"
	"sync"
	//"text/template"
	"time"

	//mqtt "github.com/eclipse/paho.mqtt.golang"
	//tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/yaml.v2"
)

// Config stores the variables for runtime configuration.
// Config stores the variables for runtime configuration.
type Config struct {
	Debug   bool
	LogSize int
	Alert struct {
		Gotify struct {
			Token  string
			Server string
		}
        }
}

// TimedEntry stores a string with timestamp.
type TimedEntry struct {
	Timestamp time.Time
	Value     string
}

var (
	config     *Config
	configFile string
	configLock = new(sync.RWMutex)
        uptime = time.Now()
	logLock    = new(sync.RWMutex)
	logHistory []TimedEntry
)

const (
	// MAXLOGSIZE defines the maximum lenght of the log history maintained (can be overridden in config)
	MAXLOGSIZE = 1000
	// Status flags for monitoring.
)
func main() {
	// load initial config
	if len(os.Args) != 2 {
		fmt.Println("Usage: " + os.Args[0] + " <configfile>")
		os.Exit(1)
	}
	configFile = os.Args[1]
	loadConfig()
}


// Loads or reloads the configuration and initializes MQTT and Telegram connections accordingly.
func loadConfig() {

	// set up initial config for logging and others to work
	if config == nil {
		config = new(Config)
		setDefaults(config)
	}

	// (re)populate config struct from file
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log("Unable to load config: " + err.Error())
		return
	}

	newconfig := new(Config)

	err = yaml.Unmarshal(yamlFile, &newconfig)
	if err != nil {
		log("Unable to load config: " + err.Error())
	}

	setDefaults(newconfig)

	configLock.Lock()
	config = newconfig
	configLock.Unlock()

	debug("Loaded config: " + fmt.Sprintf("%+v", getConfig()))
}

// Set defaults for configuration values.
func setDefaults(c *Config) {
	if c.LogSize == 0 {
		c.LogSize = MAXLOGSIZE
	}
}

// getConfig returns the current configuration.
func getConfig() *Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}

// Omits a debug log entry, if debug logging is enabled.
func debug(s string) {
	if getConfig().Debug {
		log("(" + s + ")")
	}
}

// Processes a log entry, prepending it to logHistory, truncating logHistory if needed.
func log(s string) {
	logLock.Lock()
	entry := TimedEntry{time.Now(), s}
	fmt.Printf("[%s] %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Value)
	logHistory = append(logHistory, TimedEntry{})
	copy(logHistory[1:], logHistory)
	logHistory[0] = entry

	if len(logHistory) > getConfig().LogSize {
		logHistory = logHistory[:getConfig().LogSize]
	}
	logLock.Unlock()
}
