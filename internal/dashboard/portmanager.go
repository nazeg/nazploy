package dashboard

import (
	"fmt"
	"net"
	"sync"
)

// PortManager manages port allocation across the system
type PortManager struct {
	mu   sync.Mutex
	used map[int]bool
}

func NewPortManager() *PortManager {
	return &PortManager{
		used: make(map[int]bool),
	}
}

func (pm *PortManager) Next() (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Scan for available ports in real-time
	for port := PortRangeStart; port <= PortRangeEnd; port++ {
		if pm.used[port] {
			continue
		}
		if !pm.isPortInUse(port) {
			pm.used[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", PortRangeStart, PortRangeEnd)
}

func (pm *PortManager) Release(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.used, port)
}

// isPortInUse checks if a port is actually occupied on the system
func (pm *PortManager) isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true // port is in use
	}
	ln.Close()
	return false
}
