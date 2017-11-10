package gossip

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stderr, "", 0)
	logger.SetOutput(ioutil.Discard)
}

func debug(format string, args ...interface{}) {
	logger.Printf(fmt.Sprintf("[DEBUG] %s", format), args...)
}

func SetDebug(w io.Writer) {
	logger.SetOutput(w)
}
