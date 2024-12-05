package graph

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kylerisse/wasgeht/pkg/shell"
)

// Graph represents a graphical representation of metrics over a specified time length.
// It encapsulates the necessary configurations and handles the rendering of the graph
// using rrdtool.
type Graph struct {
	// UrlPath is the filesystem path where the generated graph image will be stored.
	// It should be a valid path with appropriate write permissions.
	UrlPath string `json:"url"`

	// expiration indicates the time after which the graph is considered stale
	// and should be regenerated. It helps in managing cache invalidation.
	expiration time.Time

	// mutex ensures thread-safe operations on the Graph instance.
	// It prevents race conditions during concurrent access.
	mutex *sync.RWMutex

	// ttl (Time-To-Live) defines the duration after which the graph expires.
	// It must be greater than 1 minute and less than 1 day.
	ttl time.Duration

	// rrdtoolCmd holds the rrdtool command and arguments for generating the
	// graph. These arguments should be validated before use.
	rrdtoolCmd []string
}

// NewGraph initializes a new Graph instance with the provided file path, rrdtool arguments,
// and TTL (Time-To-Live) duration. It performs input validation to ensure that the file path
// is valid, rrdtool arguments are provided, and TTL is within the acceptable range.
//
// Parameters:
//   - filePath: The filesystem path where the graph image will be saved. Must be non-empty and
//     the directory part must exist and be a directory.
//   - rrdtoolCmd: A slice of strings representing the rrdtool command and arguments.
//     Must contain at least one argument.
//   - ttl: The duration after which the graph expires. Must be greater than 1 minute and less than 1 day.
//
// Returns:
// - A pointer to the initialized Graph instance.
// - An error if any validation fails.
func NewGraph(filePath string, rrdtoolCmd []string, ttl time.Duration) (*Graph, error) {

	if strings.TrimSpace(filePath) == "" {
		return nil, errors.New("filePath cannot be empty")
	}

	dir := filepath.Dir(filePath)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, errors.New("filePath contains non-directory")
	}

	if len(rrdtoolCmd) < 2 {
		return nil, errors.New("rrdtoolCmd cannot be empty")
	}

	if ttl < (1*time.Minute) || ttl > (24*time.Hour) {
		return nil, errors.New("duration must be >= 1 min and <= 1 day")
	}

	return &Graph{
		UrlPath:    filePath,
		expiration: time.Now(),
		rrdtoolCmd: rrdtoolCmd,
		mutex:      &sync.RWMutex{},
	}, nil
}

// UpdateIfExpired checks whether the graph has expired based on the current time and
// the Expiration field. If the graph is expired, it triggers a redraw by calling the draw method.
//
// Returns:
// - An error if the draw operation fails.
// - nil if the graph is not expired or the draw operation succeeds.
func (g *Graph) UpdateIfExpired() error {
	if g.isExpired() {
		return g.draw()
	}
	return nil
}

// draw generates the graph by executing the rrdtool command with the provided arguments.
// It ensures thread-safe execution using a mutex to prevent concurrent modifications.
// Upon successful execution, it updates the Expiration time based on the TTL.
//
// Returns:
// - An error if the rrdtool command fails to execute or if any other issue arises.
func (g *Graph) draw() error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	_, err := shell.RunCommand(g.rrdtoolCmd, (5 * time.Second))
	if err != nil {
		return err
	}
	g.expiration = time.Now().Add(g.ttl)
	return nil
}

// isExpired checks whether the graph has expired based on the current time and the Expiration field.
//
// Returns:
// - true if the current time is after the Expiration time, indicating the graph is stale.
// - false otherwise.
//
// Note:
// - This method acquires a read lock to ensure thread-safe access to the expiration field.
func (g *Graph) isExpired() bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return time.Now().After(g.expiration)
}
