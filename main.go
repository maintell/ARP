package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

var (
	configFiles Arg // "Config file for Application Reverse Proxy.", the option is customed type, parse in main
	configDir   string
	ver         = flag.Bool("version", false, "Show current version of ARP.")
	test        = flag.Bool("test", false, "Test config file only, without launching ARP server.")
	format      = flag.String("format", "json", "Format of input file.")

	/* We have to do this here because Golang's Test will also need to parse flag, before
	 * main func in this file is run.
	 */
	_ = func() error {

		flag.Var(&configFiles, "config", "Config file for ARP. Multiple assign is accepted (only json). Latter ones overrides the former ones.")
		flag.Var(&configFiles, "c", "Short alias of -config")
		flag.StringVar(&configDir, "confdir", "", "A dir with multiple json config")

		return nil
	}()
)

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func dirExists(file string) bool {
	if file == "" {
		return false
	}
	info, err := os.Stat(file)
	return err == nil && info.IsDir()
}

func readConfDir(dirPath string) {
	confs, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Fatalln(err)
	}
	for _, f := range confs {
		if strings.HasSuffix(f.Name(), ".json") {
			configFiles.Set(path.Join(dirPath, f.Name()))
		}
	}
}

func getConfigFilePath() (Arg, error) {
	if dirExists(configDir) {
		log.Println("Using confdir from arg:", configDir)
		readConfDir(configDir)
	} else {
		if envConfDir := GetConfDirPath(); dirExists(envConfDir) {
			log.Println("Using confdir from env:", envConfDir)
			readConfDir(envConfDir)
		}
	}

	if len(configFiles) > 0 {
		return configFiles, nil
	}

	if workingDir, err := os.Getwd(); err == nil {
		configFile := filepath.Join(workingDir, "config.json")
		if fileExists(configFile) {
			log.Println("Using default config: ", configFile)
			return Arg{configFile}, nil
		}
	}

	if configFile := GetConfigurationPath(); fileExists(configFile) {
		log.Println("Using config from env: ", configFile)
		return Arg{configFile}, nil
	}

	log.Println("Using config from STDIN")
	return Arg{"stdin:"}, nil
}

func GetConfigFormat() string {
	return "json"
}

func printVersion() {
	ver := VersionStatement()
	for _, s := range ver {
		fmt.Println(s)
	}
}

func StartApplicationServer() (Server, error) {
	configFiles, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}

	config, err := LoadConfig(GetConfigFormat(), configFiles[0], configFiles)
	if err != nil {
		return nil, newError("failed to read config files: [", configFiles.String(), "]").Base(err)
	}

	server, err := New(config)
	if err != nil {
		return nil, newError("failed to create server").Base(err)
	}

	return server, nil
}

func main() {

	flag.Parse()
	printVersion()
	if *ver {
		return
	}

	server, err := StartApplicationServer()
	if err != nil {
		fmt.Println(err)
		// Configuration error. Exit with a special value to prevent systemd from restarting.
		os.Exit(23)
	}

	if *test {
		fmt.Println("Configuration OK.")
		os.Exit(0)
	}

	if err := server.Start(); err != nil {
		fmt.Println("Failed to start", err)
		os.Exit(-1)
	}

	defer server.Close()

	// Explicitly triggering GC to remove garbage from config loading.
	runtime.GC()

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
		<-osSignals
	}
}
