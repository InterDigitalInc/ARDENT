package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/hot"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/infra"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/sanity"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/stack"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/http/rest"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/orchestrator"
	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/storage/mysql"
)

const (
	// default log level
	defaultLogLevel = "ERROR"
	// default http rest ip
	defaultRestIp = "localhost"
	// default http rest port
	defaultRestPort = 8080
	// default heat directory path
	defaultHeatDirPath = "../../heat"
	// infra-descriptor-definitions.yml file path
	defaultDescDefFilePath = "../../util/infra-descriptor-definitions.yml"
)

func main() {
	// pass following in command line arguments :
	// -mysqlusr=<required>
	// -mysqlpwd=<required>
	// -mysqlserverip=<required>
	// -dbname=<required>
	// -restip=<optional>
	// -restport=<optional>
	// -loglevel=<optional>
	// -logfile=<optional>
	// -heatdir=<optional>
	// -descdefinition=<optional>
	// e.g - ./ardent -mysqlusr=user -mysqlpwd=password -restip=192.168.121.12 -restport=8080

	mysqlUsr := flag.String("mysqlusr", "", "MySQL Username")
	mysqlPwd := flag.String("mysqlpwd", "", "MySQL Password")
	mysqlServerIp := flag.String("mysqlserverip", "", "MySQL Server IP Address")
	storeName := flag.String("dbname", "", "Database Name")
	restIp := flag.String("restip", defaultRestIp, "HTTP Server IP")
	restPort := flag.Int("restport", defaultRestPort, "HTTP Server Port")
	logLevel := flag.String("loglevel", defaultLogLevel, "Log Level [DEBUG|INFO|ERROR]")
	logFilePath := flag.String("logfile", "", "Log file path")
	heatDirPath := flag.String("heatdir", defaultHeatDirPath, "heat directory path")
	descDefFilePath := flag.String("descdefinition", defaultDescDefFilePath, "Descriptor definition file path")
	flag.Parse()

	if *mysqlUsr == "" || *mysqlPwd == "" || *mysqlServerIp == "" || *storeName == "" {
		log.Fatalf("\nUsage: ./ardent -mysqlusr=<required> -mysqlpwd=<required> -mysqlserverip=<required> -dbname=<required>\n" +
			"                -restip=<optional> -restport=<optional> -loglevel=<optional[DEBUG|INFO|ERROR]>\n" +
			"                -logfile=<optional> -heatdir=<optional> -descdefinition=<optional>")
	}

	logger := logrus.New()
	if *logFilePath == "" {
		logger.SetOutput(os.Stdout)
	} else {
		f, err := os.OpenFile(*logFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			log.Fatalf("Error in opening Log file. %v", err)
		}
		defer f.Close()
		logger.SetOutput(f)
	}

	switch *logLevel {
	case "DEBUG":
		logger.SetLevel(logrus.DebugLevel)
	case "INFO":
		logger.SetLevel(logrus.InfoLevel)
	case "ERROR":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.Errorf("Invalid loglevel")
		return
	}

	logger.Infof("Starting ARDENT!!")

	// exit, if any of the function call fails.

	// get db handler
	logger.Debugf("Initializing Storage Service")
	s, err := mysql.NewStorage(logger, *mysqlUsr, *mysqlPwd, *mysqlServerIp, *storeName)
	if err != nil {
		logger.Errorf("Error in initializing Store. %v", err)
		return
	}
	logger.Infof("Storage Service initialized successfully!")

	// initiate sanity service
	logger.Debugf("Initializing Sanity Service")
	sanity, err := sanity.NewService(logger, s, *heatDirPath)
	if err != nil {
		logger.Errorf("Error in initializing Sanity Service. %v", err)
		return
	}
	logger.Infof("Sanity Service initialized successfully!")

	// initiate hot service
	logger.Debugf("Initializing Hot Service")
	hot, err := hot.NewService(logger, s, sanity, *heatDirPath)
	if err != nil {
		logger.Errorf("Error in initializing Hot Service. %v", err)
		return
	}
	logger.Infof("Hot Service initialized successfully!")

	// initiate stack service
	logger.Debugf("Initializing Stack Service")
	stack, err := stack.NewService(logger, s)
	if err != nil {
		logger.Errorf("Error in initializing Stack Service. %v", err)
		return
	}
	logger.Infof("Stack Service initialized successfully!")

	// initiate infra service
	logger.Debugf("Initializing Infra Service")
	infra, err := infra.NewService(logger, s, hot, sanity, stack, *descDefFilePath)
	if err != nil {
		logger.Errorf("Error in initializing Infra Service. %v", err)
		return
	}
	logger.Infof("Infra Service initialized successfully!")

	// initiate orchestrator
	logger.Debugf("Initializing Orchestrator")
	err = orchestrator.Intialize(logger)
	if err != nil {
		logger.Errorf("Error in initializing Orchestrator. %v", err)
		return
	}
	logger.Infof("Orchestrator initialized successfully!")

	// initiate mux router
	logger.Infof("Initializing HTTP REST Service")
	router, err := rest.Handler(logger, infra, hot, stack, sanity)
	if err != nil {
		logger.Errorf("Error in initializing HTTP REST Service. %v", err)
		return
	}

	logger.Debugf("Starting HTTP REST Server at Port: %d", *restPort)
	if err := http.ListenAndServe(*restIp+":"+strconv.Itoa(*restPort), router); err != nil {
		logger.Errorf("Failure in starting HTTP REST Server at port %d. %v", *restPort, err)
		return
	}
}
