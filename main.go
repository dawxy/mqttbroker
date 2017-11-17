package main

import (
	"flag"
	"net/http"
	"net/http/pprof"
	"strconv"

	"github.com/shouyingo/logwriter"
	"github.com/vizee/echo"
)

var (
	pprofAddr = ""
	optListen string
)

func onServiceStat(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		switch r.FormValue("key") {
		case "loglevel":
			n, _ := strconv.Atoi(r.FormValue("value"))
			if echo.LogLevel(n) <= echo.DebugLevel {
				echo.SetLevel(echo.LogLevel(n))
			}
			w.Write([]byte("ok"))
		}
	}
}

func startPprof() {
	mux := http.NewServeMux()
	mux.HandleFunc("/service/stat", onServiceStat)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	echo.Info("http pprof serving", echo.String("addr", pprofAddr))
	err := http.ListenAndServe(pprofAddr, mux)
	if err != nil {
		echo.Error("http pprof failed", echo.Errors("error", err))
	}
}

func main() {
	var (
		optLog   string
		optDebug bool
	)
	flag.BoolVar(&optDebug, "debug", false, "show debug")
	flag.StringVar(&optLog, "log", "", "path/to/roll.log")
	flag.StringVar(&pprofAddr, "pprof", ":0", "address:port")
	flag.StringVar(&optListen, "listen", ":1883", "address:port")
	flag.Parse()
	if optDebug {
		echo.SetLevel(echo.DebugLevel)
	}
	if optLog != "" {
		echo.SetOutput(logwriter.New(optLog, 256*1024*1024, 32))
	}
	if pprofAddr != "" {
		go startPprof()
	}
	Run()
}
