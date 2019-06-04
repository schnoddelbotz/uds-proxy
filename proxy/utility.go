package proxy

import (
	"fmt"
	"io/ioutil"
	"os"
)

func sigHandler(c chan os.Signal, env *Instance) {
	for sig := range c {
		println()
		env.Shutdown(sig)
		os.Exit(0)
	}
}

func writePidFile(pidFilePath string) error {
	if pidFilePath == "" {
		return nil
	}
	data := []byte(fmt.Sprintf("%d", os.Getpid()))
	return ioutil.WriteFile(pidFilePath, data, 0644)
}
