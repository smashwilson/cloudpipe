package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"

	log "github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"
)

// Context provides shared state among individual route handlers.
type Context struct {
	Settings
	Storage
}

// Settings contains configuration options loaded from the environment.
type Settings struct {
	Port         int
	LogLevel     string
	MongoURL     string
	AdminName    string
	AdminKey     string
	DockerHost   string
	DockerTLS    bool
	DockerCACert string
	DockerCert   string
	DockerKey    string
	Image        string
	Poll         int
	Web          bool
	Runner       bool
}

// NewContext loads the active configuration and applies any immediate, global settings like the
// logging level.
func NewContext() (*Context, error) {
	c := &Context{}

	if err := c.Load(); err != nil {
		return c, err
	}

	// Summarize the loaded settings.

	log.WithFields(log.Fields{
		"port":          c.Port,
		"logging level": c.LogLevel,
		"mongo URL":     c.MongoURL,
		"admin account": c.AdminName,
	}).Info("Initializing with loaded settings.")

	// Configure the logging level.

	level, err := log.ParseLevel(c.LogLevel)
	if err != nil {
		return c, err
	}
	log.SetLevel(level)

	// Connect to MongoDB.

	c.Storage, err = NewMongoStorage(c)
	if err != nil {
		return c, err
	}
	if err := c.Storage.Bootstrap(); err != nil {
		return c, err
	}

	return c, nil
}

// Load configuration settings from the environment, apply defaults, and validate them.
func (c *Context) Load() error {
	if err := envconfig.Process("RHO", &c.Settings); err != nil {
		return err
	}

	if c.Port == 0 {
		c.Port = 8000
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	if c.MongoURL == "" {
		c.MongoURL = "mongo"
	}

	if c.Poll == 0 {
		c.Poll = 500
	}

	if c.DockerHost == "" {
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			c.DockerHost = host
		} else {
			c.DockerHost = "unix:///var/run/docker.sock"
		}
	}

	certRoot := os.Getenv("DOCKER_CERT_PATH")
	if certRoot == "" {
		user, err := user.Current()
		if err != nil {
			return fmt.Errorf("Unable to read the current OS user: %v", err)
		}

		certRoot = path.Join(user.HomeDir, ".docker")
	}

	if c.DockerCACert == "" {
		c.DockerCACert = path.Join(certRoot, "ca.pem")
	}

	if c.DockerCert == "" {
		c.DockerCert = path.Join(certRoot, "cert.pem")
	}

	if c.DockerKey == "" {
		c.DockerKey = path.Join(certRoot, "key.pem")
	}

	if c.Image == "" {
		c.Image = "cloudpipe/runner-py2"
	}

	if _, err := log.ParseLevel(c.LogLevel); err != nil {
		return err
	}

	// If neither web nor runner are explicitly enabled, enable both.
	if !c.Web && !c.Runner {
		if os.Getenv("RHO_WEB") != "" && os.Getenv("RHO_RUNNER") != "" {
			return errors.New("You must enable either RHO_WEB or RHO_RUNNER!")
		}

		c.Web, c.Runner = true, true
	}

	return nil
}

// ListenAddr generates an address to bind the net/http server to based on the current settings.
func (c *Context) ListenAddr() string {
	return fmt.Sprintf(":%d", c.Port)
}
