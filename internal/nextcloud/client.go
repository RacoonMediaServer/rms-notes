package nextcloud

import (
	"github.com/RacoonMediaServer/rms-notes/internal/vault"
	"github.com/studio-b12/gowebdav"
	"go-micro.dev/v4/logger"
	"os"
	"path/filepath"
)

type Client struct {
	c *gowebdav.Client
	l logger.Logger
}

type WebDAV struct {
	Root     string
	User     string
	Password string
}

func NewClient(config WebDAV) vault.Accessor {
	root := gowebdav.Join(config.Root, "files/"+config.User)
	return &Client{
		c: gowebdav.NewClient(root, config.User, config.Password),
		l: logger.Fields(map[string]interface{}{"from": "nextcloud"}),
	}
}

func (c *Client) Read(path string) ([]byte, error) {
	return c.c.Read(path)
}

func (c *Client) List(path string) ([]os.FileInfo, error) {
	return c.c.ReadDir(path)
}

func (c *Client) Write(path string, content []byte) error {
	return c.c.Write(path, content, 0644)
}

func (c *Client) Walk(root string, fn filepath.WalkFunc) error {
	err := c.walkDir(root, fn)
	if err == filepath.SkipDir || err == filepath.SkipAll {
		return nil
	}
	return err
}

func (c *Client) walkDir(root string, fn filepath.WalkFunc) error {
	files, err := c.List(root)
	if err != nil {
		return fn(root, nil, err)
	}
	for _, f := range files {
		next := filepath.Join(root, f.Name())
		err = fn(next, f, nil)
		if err != nil {
			if f.IsDir() && err == filepath.SkipDir {
				continue
			}
			return err
		}

		if f.IsDir() {
			err = c.walkDir(next, fn)
			if err != nil && err != filepath.SkipDir {
				return err
			}
		}
	}

	return nil
}
