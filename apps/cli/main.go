package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/RacoonMediaServer/rms-notes/internal/folder"
	"github.com/RacoonMediaServer/rms-notes/internal/nextcloud"
	"github.com/RacoonMediaServer/rms-notes/internal/obsidian"
	"github.com/RacoonMediaServer/rms-notes/internal/vault"
	"go-micro.dev/v4/logger"
)

func main() {
	addr := flag.String("remote", "", "Address")
	user := flag.String("user", "", "User")
	pwd := flag.String("password", "", "Password")
	dir := flag.String("directory", "", "Vault directory")
	flag.Parse()

	_ = logger.Init(logger.WithLevel(logger.DebugLevel))

	params := nextcloud.WebDAV{
		Root:     *addr,
		User:     *user,
		Password: *pwd,
	}

	var conn vault.Accessor
	if strings.HasPrefix(*addr, "http") {
		conn = nextcloud.NewClient(params)
	} else {
		conn = folder.NewAccessor()
		*dir = filepath.Join(*addr, *dir)
	}

	cli := obsidian.NewVault(context.Background(), *dir, conn, nil)
	if err := cli.Refresh(obsidian.Scheduled); err != nil {
		panic(err)
	}
	cli.StartWatchingChanges()

	for {
		tasks := cli.GetTasks()
		for _, t := range tasks {
			fmt.Println(t.Hash(), t.String())
		}
		<-time.After(30 * time.Second)
	}
}
