package main

import (
	"flag"
	"fmt"
	"github.com/boivie/lovebeat-go/alert"
	"github.com/boivie/lovebeat-go/backend"
	"github.com/boivie/lovebeat-go/config"
	"github.com/boivie/lovebeat-go/dashboard"
	"github.com/boivie/lovebeat-go/httpapi"
	"github.com/boivie/lovebeat-go/service"
	"github.com/boivie/lovebeat-go/tcpapi"
	"github.com/boivie/lovebeat-go/udpapi"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
)

var log = logging.MustGetLogger("lovebeat")

const (
	VERSION                 = "0.1.0"
	MAX_UNPROCESSED_PACKETS = 1000
)

var (
	udpAddr     = flag.String("udp", ":8127", "UDP service address")
	tcpAddr     = flag.String("tcp", ":8127", "TCP service address")
	debug       = flag.Bool("debug", false, "print statistics sent to graphite")
	showVersion = flag.Bool("version", false, "print version string")
	workDir     = flag.String("workdir", "work", "working directory")
	cfgFile     = flag.String("config", "/etc/lovebeat.cfg", "configuration file")
)

var (
	signalchan = make(chan os.Signal, 1)
)

func signalHandler() {
	for {
		select {
		case sig := <-signalchan:
			fmt.Printf("!! Caught signal %d... shutting down\n", sig)
			return
		}
	}
}

func httpServer(port int16, svcs *service.Services) {
	rtr := mux.NewRouter()
	httpapi.Register(rtr, svcs.GetClient())
	dashboard.Register(rtr, svcs.GetClient())
	http.Handle("/", rtr)
	log.Info("HTTP server running on port %d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func getHostname() string {
	var hostname, err = os.Hostname()
	if err != nil {
		return fmt.Sprintf("unknown_%d", os.Getpid())
	}
	return strings.Split(hostname, ".")[0]
}

func main() {
	flag.Parse()

	var format = logging.MustStringFormatter("%{level} %{message}")
	logging.SetFormatter(format)
	if *debug {
		logging.SetLevel(logging.DEBUG, "lovebeat")
	} else {
		logging.SetLevel(logging.INFO, "lovebeat")
	}
	log.Debug("Debug logs enabled")

	if *showVersion {
		fmt.Printf("lovebeats v%s (built w/%s)\n", VERSION, runtime.Version())
		return
	}

	var cfg = config.ReadConfig(*cfgFile)

	var hostname = getHostname()
	log.Info("Lovebeat v%s started as host %s, PID %d", VERSION, hostname, os.Getpid())

	var be = backend.NewFileBackend(*workDir)
	var alerters = []alert.Alerter{alert.NewMailAlerter(&cfg.Mail)}
	var svcs = service.NewServices(be, alerters)

	signal.Notify(signalchan, syscall.SIGTERM)

	go svcs.Monitor()
	go httpServer(8080, svcs)
	go udpapi.Listener(*udpAddr, svcs.GetClient())
	go tcpapi.Listener(*tcpAddr, svcs.GetClient())

	// Ensure that the 'all' view exists
	svcs.GetClient().CreateOrUpdateView("all", "", "")

	log.Info("Ready to handle incoming connections")

	signalHandler()
}
