// +build darwin

package http

import (
	"fmt"
	"net"

	reusep "github.com/kavu/go_reuseport"
)

// Listener returns net.Listener object
func Listener(port int) (net.Listener, error) {
	return reusep.Listen("tcp", fmt.Sprintf(":%d", port))
}
