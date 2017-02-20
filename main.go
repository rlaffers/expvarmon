package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/gizak/termui.v1"

	"github.com/Sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Structure of all arguments (CLI or config items) to this program
type arguments struct {
	interval time.Duration
	ports    string
	vars     string
	dummy    bool
	self     bool
	endpoint string
	debug    bool
}

var (
	defaults arguments = arguments{
		interval: 5 * time.Second,
		ports:    "",
		vars:     "mem:memstats.Alloc,mem:memstats.Sys,mem:memstats.HeapAlloc,mem:memstats.HeapInuse,duration:memstats.PauseNs,duration:memstats.PauseTotalNs",
		dummy:    false,
		self:     false,
		endpoint: "/debug/vars",
		debug:    false,
	}

	interval time.Duration
	_        = flag.Duration("i", defaults.interval, "Polling interval")

	urls []string
	_    = flag.String("ports", defaults.ports, "Ports/URLs for accessing services expvars (start-end,port2,port3,https://host:port)")

	varsArg []string
	_       = flag.String("vars", defaults.vars, "Vars to monitor (comma-separated)")

	dummy bool
	_     = flag.Bool("dummy", defaults.dummy, "Use dummy (console) output")

	self bool
	_    = flag.Bool("self", defaults.self, "Monitor itself")

	endpoint string
	_        = flag.String("endpoint", defaults.endpoint, "URL endpoint for expvars")

	debug bool
	_     = flag.Bool("debug", defaults.debug, "Turn debugging mode on")

	logger *logrus.Logger
)

func init() {
	logger = logrus.New()
}

func main() {
	flag.Usage = Usage
	flag.Parse()

	initConfiguration()

	if debug {
		logger.Level = logrus.DebugLevel
	}

	DefaultEndpoint = endpoint

	// Process ports/urls
	ports, _ := ParsePortsSlice(urls)
	if self {
		port, err := StartSelfMonitor()
		if err == nil {
			ports = append(ports, port)
		}
	}
	if len(ports) == 0 {
		fmt.Fprintln(os.Stderr, "no ports specified. Use -ports arg to specify ports of Go apps to monitor")
		Usage()
		os.Exit(1)
	}
	if interval <= 0 {
		fmt.Fprintln(os.Stderr, "update interval is not valid. Valid examples: 5s, 1m, 1h30m")
		Usage()
		os.Exit(1)
	}

	// Process vars
	vars, err := ParseVarsSlice(varsArg)
	if err != nil {
		log.Fatal(err)
	}

	// Init UIData
	data := NewUIData(vars)
	for _, port := range ports {
		service := NewService(port, vars)
		data.Services = append(data.Services, service)
	}

	// Start proper UI
	var ui UI
	if len(data.Services) > 1 {
		ui = &TermUI{}
	} else {
		ui = &TermUISingle{}
	}
	if dummy {
		ui = &DummyUI{}
	}

	if err := ui.Init(*data); err != nil {
		log.Fatal(err)
	}
	defer ui.Close()

	tick := time.NewTicker(interval)
	evtCh := termui.EventCh()

	UpdateAll(ui, data)
	for {
		select {
		case <-tick.C:
			UpdateAll(ui, data)
		case e := <-evtCh:
			if e.Type == termui.EventKey && e.Ch == 'q' {
				return
			}
			if e.Type == termui.EventResize {
				ui.Update(*data)
			}
		}
	}
}

// UpdateAll collects data from expvars and refreshes UI.
func UpdateAll(ui UI, data *UIData) {
	var wg sync.WaitGroup
	for _, service := range data.Services {
		wg.Add(1)
		go service.Update(&wg)
	}
	wg.Wait()

	data.LastTimestamp = time.Now()

	ui.Update(*data)
}

// Usage reimplements flag.Usage
func Usage() {
	progname := os.Args[0]
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", progname)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Examples:
	%s -ports="80"
	%s -ports="23000-23010,http://example.com:80-81" -i=1m
	%s -ports="80,remoteapp:80" -vars="mem:memstats.Alloc,duration:Response.Mean,Counter"
	%s -ports="1234-1236" -vars="Goroutines" -self

For more details and docs, see README: http://github.com/divan/expvarmon
`, progname, progname, progname, progname)
}

// initConfiguration loads the configuration file.
func initConfiguration() {

	viper.SetConfigName("config")
	viper.AddConfigPath("$HOME/.expvarmon")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	// set default values
	viper.SetDefault("interval", defaults.interval)
	viper.SetDefault("ports", defaults.ports)
	viper.SetDefault("vars", defaults.vars)
	viper.SetDefault("dummy", defaults.dummy)
	viper.SetDefault("self", defaults.self)
	viper.SetDefault("endpoint", defaults.endpoint)
	viper.SetDefault("debug", defaults.debug)

	// bind viper config options to flags
	viper.BindPFlag("i", flag.Lookup("interval"))
	viper.BindPFlag("ports", flag.Lookup("ports"))
	viper.BindPFlag("vars", flag.Lookup("vars"))
	viper.BindPFlag("dummy", flag.Lookup("dummy"))
	viper.BindPFlag("self", flag.Lookup("self"))
	viper.BindPFlag("endpoint", flag.Lookup("endpoint"))
	viper.BindPFlag("debug", flag.Lookup("debug"))

	// assign viper-provided values to the global configuration variables
	interval = viper.GetDuration("interval")

	dummy = viper.GetBool("dummy")
	urls = viper.GetStringSlice("ports")
	varsArg = viper.GetStringSlice("vars")
	self = viper.GetBool("self")
	endpoint = viper.GetString("endpoint")

}
