package systemd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	dbus "github.com/godbus/dbus/v5"
	"github.com/simple_runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
)

var (
	isRunningSystemdOnce sync.Once
	isRunningSystemd     bool
)

func IsRunningSystemd() bool {
	isRunningSystemdOnce.Do(func() {
		fi, err := os.Lstat("/run/systemd/system")
		isRunningSystemd = err == nil && fi.IsDir()
	})
	return isRunningSystemd
}

func newProp(name string, units interface{}) systemdDbus.Property {
	return systemdDbus.Property{
		Name:  name,
		Value: dbus.MakeVariant(units),
	}
}

func getUnitName(c *configs.Cgroup) string {
	return c.Name
}

func resetFailedUnit(cm *dbusConnManager, name string) {
	err := cm.retryOnDisconnect(func(c *systemdDbus.Conn) error {
		return c.ResetFailedUnitContext(context.TODO(), name)
	})
	if err != nil {
		logrus.Warnf("unable to reset failed unit: %v", err)
	}
}

func startUnit(cm *dbusConnManager, unitName string, properties []systemdDbus.Property) error {
	statusChan := make(chan string, 1)
	err := cm.retryOnDisconnect(func(c *systemdDbus.Conn) error {
		_, err := c.StartTransientUnitContext(context.TODO(), unitName, "replace", properties, statusChan)
		return err
	})
	if err == nil {
		timeout := time.NewTimer(30 * time.Second)
		defer timeout.Stop()

		select {
		case s := <-statusChan:
			close(statusChan)
			// Please refer to https://pkg.go.dev/github.com/coreos/go-systemd/v22/dbus#Conn.StartUnit
			if s != "done" {
				resetFailedUnit(cm, unitName)
				return fmt.Errorf("error creating systemd unit `%s`: got `%s`", unitName, s)
			}
		case <-timeout.C:
			resetFailedUnit(cm, unitName)
			return errors.New("Timeout waiting for systemd to create " + unitName)
		}
	} else if !isUnitExists(err) {
		return err
	}
	return nil
}

// isDbusError returns true if the error is a specific dbus error.
func isDbusError(err error, name string) bool {
	if err != nil {
		var derr dbus.Error
		if errors.As(err, &derr) {
			return strings.Contains(derr.Name, name)
		}
	}
	return false
}

// isUnitExists returns true if the error is that a systemd unit already exists.
func isUnitExists(err error) bool {
	return isDbusError(err, "org.freedesktop.systemd1.UnitExists")
}
