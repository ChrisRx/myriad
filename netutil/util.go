package netutil

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func ParseAddr(addr string) (string, int) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		panic(errors.Errorf("invalid addr: %#v", addr))
	}
	port, _ := strconv.Atoi(parts[1])
	return parts[0], port
}
