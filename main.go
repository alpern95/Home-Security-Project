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
	"strings"
	"sync"
	//"text/template"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	//tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/yaml.v2"
)

// Config stores the variables for runtime configuration.
// Config stores the variables for runtime configuration.
type Config struct {
	Debug   bool
	LogSize int
	Monitor struct {
		MQTT struct {
			Server          string
			Port            int
			User            string
			Password        string
			History         int
			StandardTimeout float64
			Targets         []struct {
				Topic   string
				Timeout int
			}
		}
        }
	Alert struct {
		Gotify struct {
			Token  string
			Server string
		}
        }
}


// MonitorData stores the actual status data of the monitoring process.
type MonitorData struct {
	MQTT map[string]*MQTTMonitorData
	sync.RWMutex
}

// MQTTTopic stores status information on a MQTT topic.
type MQTTMonitorData struct {
	FirstSeen     time.Time
	LastSeen      time.Time
	LastError     time.Time
	LastPayload   string
	History       []TimedEntry
	AvgTransmit   float64 `json:"-"`
	Timeout       float64 `json:"-"`
	CustomTimeout float64
	Status        int32
	Samples       int64
	Alerts        int64
	Deleted       bool
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

	monitorMqttClient mqtt.Client

	monitorData = MonitorData{
		MQTT: make(map[string]*MQTTMonitorData),
		//Ping: make(map[string]*PingMonitorData),
		//HTTP: make(map[string]*HTTPMonitorData),
		//Exec: make(map[string]*ExecMonitorData)
        }

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

	monitorData.Lock()
	// remove deleted MQTT targets
	for k := range monitorData.MQTT {
		if monitorData.MQTT[k].Deleted {
			delete(monitorData.MQTT, k)
		}
	}

	monitorData.Unlock()

	// connect MQTT if configured
	if getConfig().Monitor.MQTT.Server != "" {
		if monitorMqttClient != nil && monitorMqttClient.IsConnected() {
			monitorMqttClient.Disconnect(1)
			debug("Disconnected from MQTT (monitoring)")
		}
		opts := mqtt.NewClientOptions()
		opts.AddBroker(fmt.Sprintf("%s://%s:%d", "tcp", getConfig().Monitor.MQTT.Server, getConfig().Monitor.MQTT.Port))
		opts.SetUsername(getConfig().Monitor.MQTT.User)
		opts.SetPassword(getConfig().Monitor.MQTT.Password)
		opts.OnConnect = func(c mqtt.Client) {

			topics := make(map[string]byte)
			for _, t := range getConfig().Monitor.MQTT.Targets {
				topics[t.Topic] = byte(0)
			}

			// deduplicate MQTT topics (remove specific topics that are included in wildcard topics)
			for t, _ := range topics {
				if strings.Contains(t, "#") {
					for tt, _ := range topics {
						if matchMQTTTopic(t, tt) && t != tt {
							delete(topics, tt)
							debug(fmt.Sprintf("Deleting %s from MQTT subscription (included in %s)", tt, t))
						}
					}
				}
			}

			t := make([]string, 0)
			for i, _ := range topics {
				t = append(t, i)
			}

			if token := c.SubscribeMultiple(topics, onMessageReceived); token.Wait() && token.Error() != nil {
				log("Unable to subscribe to MQTT: " + token.Error().Error())
			} else {
				log("Subscribed to MQTT topics: " + strings.Join(t, ", "))
			}
		}

		monitorMqttClient = mqtt.NewClient(opts)
		if token := monitorMqttClient.Connect(); token.Wait() && token.Error() != nil {
			log("Unable to connect to MQTT for monitoring: " + token.Error().Error())
		} else {
			log("Connected to MQTT server for monitoring at " + opts.Servers[0].String())
		}
	}
}

// Set defaults for configuration values.
func setDefaults(c *Config) {
	if c.LogSize == 0 {
		c.LogSize = MAXLOGSIZE
	}
	if c.Monitor.MQTT.History == 0 {
		c.Monitor.MQTT.History = 10
	}
	if c.Monitor.MQTT.Port == 0 {
		c.Monitor.MQTT.Port = 1883
	}
	if c.Monitor.MQTT.StandardTimeout == 0 {
		c.Monitor.MQTT.StandardTimeout = 1.5
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

// Receives an MQTT message and updates status accordingly.
func onMessageReceived(client mqtt.Client, message mqtt.Message) {
	debug("MQTT: " + message.Topic() + ": " + string(message.Payload()))

	monitorData.Lock()
	defer monitorData.Unlock()

	e, ok := monitorData.MQTT[message.Topic()]
	if !ok {
		monitorData.MQTT[message.Topic()] = &MQTTMonitorData{}
		e = monitorData.MQTT[message.Topic()]
	}

	if e.Deleted {
		return
	}

	e.History = append(e.History, TimedEntry{time.Now(), string(message.Payload())})
	if len(e.History) > getConfig().Monitor.MQTT.History {
		e.History = e.History[1:]
	}

	var total float64 = 0
	for i, v := range e.History {
		if i > 0 {
			total += v.Timestamp.Sub(e.History[i-1].Timestamp).Seconds()
		}
	}
	e.AvgTransmit = total / float64(len(e.History)-1)
	if e.FirstSeen.IsZero() {
		e.FirstSeen = time.Now()
	}
	e.LastSeen = time.Now()
	e.LastPayload = string(message.Payload())
	e.Samples++

	//monitorData.MQTT[message.Topic()] = e

}

func matchMQTTTopic(pattern string, subject string) bool {
	sl := strings.Split(subject, "/")
	pl := strings.Split(pattern, "/")

	slen := len(sl)
	plen := len(pl)
	lasti := plen - 1

	for i := range pl {

		if len(pl[i]) == 0 && len(sl[i]) == 0 {
			continue
		}
		if len(pl[i]) == 0 {
			continue
		}
		if len(sl[i]) == 0 && pl[i][0] != '#' {
			return false
		}
		if pl[i][0] == '#' {
			return i == lasti
		}
		if pl[i][0] != '+' && pl[i] != sl[i] {
			return false
		}
	}
	return plen == slen
}
