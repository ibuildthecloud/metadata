package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/rancher/metadata/content"
	"github.com/rancher/metadata/content/event"
	"context"
)

const (
	ContentText = 1
	ContentJSON = 2
)

var (
	VERSION string
	// A key to check for magic traversing of arrays by a string field in them
	// For example, given: { things: [ {name: 'asdf', stuff: 42}, {name: 'zxcv', stuff: 43} ] }
	// Both ../things/0/stuff and ../things/asdf/stuff will return 42 because 'asdf' matched the 'anme' field of one of the 'things'.
	MAGIC_ARRAY_KEYS = []string{"name", "uuid"}
)

// ServerConfig specifies the configuration for the metadata server
type ServerConfig struct {
	listen       string
	listenReload string
	enableXff    bool

	router       *mux.Router
	reloadRouter *mux.Router
	store        content.Store
	reloadChan   chan os.Signal
}

func main() {
	app := cli.NewApp()
	app.Action = appMain
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Debug",
		},
		cli.BoolFlag{
			Name:  "xff",
			Usage: "X-Forwarded-For header support",
		},
		cli.StringFlag{
			Name:  "listen",
			Value: ":80",
			Usage: "Address to listen to (TCP)",
		},
		cli.StringFlag{
			Name:  "listenReload",
			Value: "127.0.0.1:8112",
			Usage: "Address to listen to for reload requests (TCP)",
		},
		cli.StringFlag{
			Name:   "access-key",
			EnvVar: "CATTLE_ACCESS_KEY",
			Usage:  "Rancher access key",
		},
		cli.StringFlag{
			Name:   "secret-key",
			EnvVar: "CATTLE_SECRET_KEY",
			Usage:  "Rancher secret key",
		},
		cli.StringFlag{
			Name:   "url",
			EnvVar: "CATTLE_URL",
			Usage:  "Rancher URL",
		},
		cli.StringFlag{
			Name:  "log",
			Value: "",
			Usage: "Log file",
		},
	}

	app.Run(os.Args)
}

func appMain(ctx *cli.Context) error {
	if ctx.GlobalBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logFile := ctx.GlobalString("log")
	if logFile != "" {
		if output, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
			logrus.Fatalf("Failed to log to file %s: %v", logFile, err)
		} else {
			logrus.SetOutput(output)
		}
	}

	sc := NewServerConfig(
		ctx.GlobalString("answers"),
		ctx.GlobalString("listen"),
		ctx.GlobalString("listenReload"),
		ctx.GlobalBool("xff"),
	)

	// Start the server
	sc.Start()

		s := NewSubscriber(
			ctx.String("access-key"),
			os.Getenv("CATTLE_ACCESS_KEY"),
			os.Getenv("CATTLE_SECRET_KEY"),
			ctx.String("answers"),
			sc.SetAnswers)
		if err := s.Subscribe(); err != nil {
			logrus.Fatal("Failed to subscribe", err)
		}
	}

	go func() {
		logrus.Info(http.ListenAndServe(":6060", nil))
	}()

	// Run the server
	sc.RunServer()

	return nil
}

func NewServerConfig(listen, listenReload string, enableXff bool) *ServerConfig {
	sc := &ServerConfig{
		listen:          listen,
		listenReload:    listenReload,
		enableXff:       enableXff,
		router:          mux.NewRouter(),
		reloadRouter:    mux.NewRouter(),
		store:           event.NewMemoryStore(context.Background()),
	}

	return sc
}

func (sc *ServerConfig) Start() {
	logrus.Infof("Starting rancher-metadata %s", VERSION)
	// on the startup, read answers from file (if present)so there is no delay
	// in serving to the client till the delta update from subscriber
	if _, err := os.Stat(sc.answersFilePath); err == nil {
		if err = sc.loadAnswersFromFile(sc.answersFilePath); err != nil {
			logrus.Fatal("Failed loading data from file")
		}
	}
}

func (sc *ServerConfig) lookupAnswer(wait bool, oldValue, version string, ip string, path []string, maxWait time.Duration) (interface{}, bool) {
	if !wait {
		v := sc.answers()
		return v.Matching(version, ip, path)
	}

	if maxWait == time.Duration(0) {
		maxWait = time.Minute
	}

	if maxWait > 2*time.Minute {
		maxWait = 2 * time.Minute
	}

	start := time.Now()

	for {
		v := sc.answers()
		val, ok := v.Matching(version, ip, path)
		if time.Now().Sub(start) > maxWait {
			return val, ok
		}
		if ok && fmt.Sprint(val) != oldValue {
			return val, ok
		}

		sc.versionCond.L.Lock()
		sc.versionCond.Wait()
		sc.versionCond.L.Unlock()
	}
}

func (sc *ServerConfig) watchSignals() {
	signal.Notify(sc.reloadChan, syscall.SIGHUP)

	go func() {
		for range sc.reloadChan {
			sc.reload()
		}
	}()
}

func (sc *ServerConfig) watchReloadHttp() {
	sc.reloadRouter.HandleFunc("/favicon.ico", http.NotFound)
	sc.reloadRouter.HandleFunc("/v1/reload", sc.httpReload).Methods("POST")

	logrus.Info("Listening for Reload on ", sc.listenReload)
	go http.ListenAndServe(sc.listenReload, sc.reloadRouter)
}

func (sc *ServerConfig) RunServer() {
	sc.watchReloadSignals()
	sc.watchReloadHttp()

	sc.router.HandleFunc("/favicon.ico", http.NotFound)
	sc.router.HandleFunc("/", sc.root).
		Methods("GET", "HEAD").
		Name("Root")

	sc.router.HandleFunc("/{version}", sc.metadata).
		Methods("GET", "HEAD").
		Name("Version")

	sc.router.HandleFunc("/{version}/{key:.*}", sc.metadata).
		Queries("wait", "true", "value", "{oldValue}").
		Methods("GET", "HEAD").
		Name("Wait")

	sc.router.HandleFunc("/{version}/{key:.*}", sc.metadata).
		Methods("GET", "HEAD").
		Name("Metadata")

	logrus.Info("Listening on ", sc.listen)
	logrus.Fatal(http.ListenAndServe(sc.listen, sc.router))
}

func (sc *ServerConfig) httpReload(w http.ResponseWriter, req *http.Request) {
	logrus.Debugf("Received HTTP reload request")
	if err := sc.reload(); err == nil {
		io.WriteString(w, "OK")
	} else {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
	}
}

func contentType(req *http.Request) int {
	str := httputil.NegotiateContentType(req, []string{
		"text/plain",
		"application/json",
	}, "text/plain")

	if strings.Contains(str, "json") {
		return ContentJSON
	} else {
		return ContentText
	}
}

func (sc *ServerConfig) root(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	logrus.WithFields(logrus.Fields{"client": sc.requestIp(req), "version": "root"}).Debugf("OK: %s", "/")

	answers := sc.answers()

	m := make(map[string]interface{})
	for _, k := range answers.Versions() {
		url, err := sc.router.Get("Version").URL("version", k)
		if err == nil {
			m[k] = (*url).String()
		} else {
			logrus.Warn("Error: ", err.Error())
		}
	}

	// If latest isn't in the list, pretend it is
	_, ok := m["latest"]
	if !ok {
		url, err := sc.router.Get("Version").URL("version", "latest")
		if err == nil {
			m["latest"] = (*url).String()
		} else {
			logrus.Warn("Error: ", err.Error())
		}
	}

	respondSuccess(w, req, m)
}

func (sc *ServerConfig) metadata(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	vars := mux.Vars(req)
	clientIp := sc.requestIp(req)

	version := vars["version"]
	wait := mux.CurrentRoute(req).GetName() == "Wait"
	oldValue := vars["oldValue"]
	maxWait, _ := strconv.Atoi(req.URL.Query().Get("maxWait"))

	answers := sc.answers()
	_, ok := answers[version]
	if !ok {
		// If a `latest` key is not provided, pick the ASCII-betically highest version and call it that.
		if version == "latest" {
			version = ""
			for _, k := range answers.Versions() {
				if k > version {
					version = k
				}
			}

			logrus.Debugf("Picked %s for latest version because none provided", version)
		} else {
			respondError(w, req, "Invalid version", http.StatusNotFound)
			return
		}
	}

	path := strings.TrimRight(req.URL.EscapedPath()[1:], "/")
	pathSegments := strings.Split(path, "/")[1:]
	displayKey := ""
	var err error
	for i := 0; err == nil && i < len(pathSegments); i++ {
		displayKey += "/" + pathSegments[i]
		pathSegments[i], err = url.QueryUnescape(pathSegments[i])
	}

	if err != nil {
		respondError(w, req, err.Error(), http.StatusBadRequest)
		return
	}

	logrus.WithFields(logrus.Fields{
		"version":  version,
		"client":   clientIp,
		"wait":     wait,
		"oldValue": oldValue,
		"maxWait":  maxWait}).Debugf("Searching for: %s", displayKey)
	val, ok := sc.lookupAnswer(wait, oldValue, version, clientIp, pathSegments, time.Duration(maxWait)*time.Second)

	if ok {
		logrus.WithFields(logrus.Fields{"version": version, "client": clientIp}).Debugf("OK: %s", displayKey)
		respondSuccess(w, req, val)
	} else {
		logrus.WithFields(logrus.Fields{"version": version, "client": clientIp}).Infof("Error: %s", displayKey)
		respondError(w, req, "Not found", http.StatusNotFound)
	}
}

func respondError(w http.ResponseWriter, req *http.Request, msg string, statusCode int) {
	obj := make(map[string]interface{})
	obj["message"] = msg
	obj["type"] = "error"
	obj["code"] = statusCode

	switch contentType(req) {
	case ContentText:
		http.Error(w, msg, statusCode)
	case ContentJSON:
		bytes, err := json.Marshal(obj)
		if err == nil {
			http.Error(w, string(bytes), statusCode)
		} else {
			http.Error(w, "{\"type\": \"error\", \"message\": \"JSON marshal error\"}", http.StatusInternalServerError)
		}
	}
}

func respondSuccess(w http.ResponseWriter, req *http.Request, val interface{}) {
	switch contentType(req) {
	case ContentText:
		respondText(w, val)
	case ContentJSON:
		respondJSON(w, req, val)
	}
}

func respondText(w http.ResponseWriter, val interface{}) {
	if val == nil {
		fmt.Fprint(w, "")
		return
	}

	switch v := val.(type) {
	case string, json.Number:
		fmt.Fprint(w, v)
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64:
		fmt.Fprintf(w, "%d", v)
	case float64:
		// The default format has extra trailing zeros
		str := strings.TrimRight(fmt.Sprintf("%f", v), "0")
		str = strings.TrimRight(str, ".")
		fmt.Fprint(w, str)
	case bool:
		if v {
			fmt.Fprint(w, "true")
		} else {
			fmt.Fprint(w, "false")
		}
	case map[string]interface{}:
		out := make([]string, len(v))
		i := 0
		for k, vv := range v {
			_, isMap := vv.(map[string]interface{})
			_, isArray := vv.([]interface{})
			if isMap || isArray {
				out[i] = fmt.Sprintf("%s/\n", url.QueryEscape(k))
			} else {
				out[i] = fmt.Sprintf("%s\n", url.QueryEscape(k))
			}
			i++
		}

		sort.Strings(out)
		for _, vv := range out {
			fmt.Fprint(w, vv)
		}
	case []interface{}:
	outer:
		for k, vv := range v {
			vvMap, isMap := vv.(map[string]interface{})
			_, isArray := vv.([]interface{})

			if isMap {
				// If the child is a map and has a "name" property, show index=name ("0=foo")
				for _, magicKey := range MAGIC_ARRAY_KEYS {
					name, ok := vvMap[magicKey]
					if ok {
						fmt.Fprintf(w, "%d=%s\n", k, url.QueryEscape(name.(string)))
						continue outer
					}
				}
			}

			if isMap || isArray {
				// If the child is a map or array, show index ("0/")
				fmt.Fprintf(w, "%d/\n", k)
			} else {
				// Otherwise, show index ("0" )
				fmt.Fprintf(w, "%d\n", k)
			}
		}
	default:
		http.Error(w, "Value is of a type I don't know how to handle", http.StatusInternalServerError)
	}
}

func respondJSON(w http.ResponseWriter, req *http.Request, val interface{}) {
	if err := json.NewEncoder(w).Encode(val); err != nil {
		respondError(w, req, "Error serializing to JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

func (sc *ServerConfig) requestIp(req *http.Request) string {
	if sc.enableXff {
		clientIp := req.Header.Get("X-Forwarded-For")
		if len(clientIp) > 0 {
			return clientIp
		}
	}

	clientIp, _, _ := net.SplitHostPort(req.RemoteAddr)
	return clientIp
}
