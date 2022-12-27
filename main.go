package main

// Programe de suveillance

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/yaml.v2"
)

// Config stores the variables for runtime configuration.
