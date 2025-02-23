package main

import (
	"fmt"

	"github.com/RacoonMediaServer/rms-notes/internal/config"
	"github.com/RacoonMediaServer/rms-notes/internal/db"
	notesService "github.com/RacoonMediaServer/rms-notes/internal/service"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"github.com/RacoonMediaServer/rms-packages/pkg/service/servicemgr"
	"github.com/urfave/cli/v2"
	"go-micro.dev/v4"
	"go-micro.dev/v4/logger"

	// Plugins
	_ "github.com/go-micro/plugins/v4/registry/etcd"
)

var Version = "v0.0.0"

const serviceName = "rms-notes"

func main() {
	logger.Infof("%s %s", serviceName, Version)
	defer logger.Info("DONE.")

	useDebug := false

	service := micro.NewService(
		micro.Name(serviceName),
		micro.Version(Version),
		micro.Flags(
			&cli.BoolFlag{
				Name:        "verbose",
				Aliases:     []string{"debug"},
				Usage:       "debug log level",
				Value:       false,
				Destination: &useDebug,
			},
		),
	)

	service.Init(
		micro.Action(func(context *cli.Context) error {
			configFile := fmt.Sprintf("/etc/rms/%s.json", serviceName)
			if context.IsSet("config") {
				configFile = context.String("config")
			}
			return config.Load(configFile)
		}),
	)

	cfg := config.Config()

	if useDebug || cfg.Debug.Verbose {
		_ = logger.Init(logger.WithLevel(logger.DebugLevel))
	}

	_ = servicemgr.NewServiceFactory(service)

	database, err := db.Connect(cfg.Database)
	if err != nil {
		logger.Fatalf("Connect to database failed: %s", err)
	}

	ns, err := notesService.New(database, service, cfg.Async)
	if err != nil {
		logger.Fatalf("Create service failed: %s", err)
	}

	// регистрируем хендлеры
	if err := rms_notes.RegisterRmsNotesHandler(service.Server(), ns); err != nil {
		logger.Fatalf("Register service failed: %s", err)
	}

	if err := service.Run(); err != nil {
		logger.Fatalf("Run service failed: %s", err)
	}
}
