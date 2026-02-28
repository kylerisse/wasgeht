package server

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	"github.com/kylerisse/wasgeht/pkg/check"
	"github.com/kylerisse/wasgeht/pkg/host"
	"github.com/kylerisse/wasgeht/pkg/rrd"
)

// checkInstance pairs a check with its RRD file, metric definitions, and status tracker.
type checkInstance struct {
	check      check.Check
	rrdFile    *rrd.RRD
	metricDefs []check.MetricDef
	status     *check.Status
}

// worker periodically runs all enabled checks against the assigned host.
func (s *Server) worker(name string, h *host.Host) {
	defer s.wg.Done()

	// Add a random delay of 1-119 seconds before starting to reduce initial filesystem activity.
	startDelay := time.Duration(rand.Intn(119)+1) * time.Second
	s.logger.Infof("Worker for host %s will start in %v", name, startDelay)
	select {
	case <-time.After(startDelay):
		s.logger.Debugf("Worker for host %s starting", name)
	case <-s.done:
		s.logger.Infof("Worker for host %s received shutdown signal before starting", name)
		return
	}

	// Initialize all enabled checks and their RRD files.
	instances := s.initChecks(name, h)
	if len(instances) == 0 {
		s.logger.Warningf("Worker for host %s: no checks to run, exiting", name)
		return
	}

	// Run periodic checks every minute.
	for {
		select {
		case <-s.done:
			s.logger.Infof("Worker for host %s received shutdown signal.", name)
			return
		default:
			s.runChecks(name, instances)

			select {
			case <-time.After(time.Minute):
				// continue with the next iteration
			case <-s.done:
				s.logger.Infof("Worker for host %s received shutdown signal.", name)
				return
			}
		}
	}
}

// initChecks creates check instances and RRD files for all enabled checks on a host.
// Each check's factory receives the user-provided config directly; all required
// addressing information must be present in the config itself.
func (s *Server) initChecks(name string, h *host.Host) []checkInstance {
	instances := make([]checkInstance, 0, len(h.Checks))

	for checkType, cfg := range h.Checks {
		factoryCfg := copyConfig(cfg)

		chk, err := s.registry.Create(checkType, factoryCfg)
		if err != nil {
			s.logger.Errorf("Worker for host %s: failed to create %s check (%v)", name, checkType, err)
			continue
		}

		desc := chk.Describe()

		if len(desc.Metrics) == 0 {
			s.logger.Errorf("Worker for host %s: %s check declares no metrics", name, checkType)
			continue
		}

		rrdFile, err := rrd.NewRRD(name, s.rrdDir, s.graphDir, checkType, desc.Metrics, desc.Label, s.logger)
		if err != nil {
			s.logger.Errorf("Worker for host %s: failed to initialize RRD for %s check (%v)", name, checkType, err)
			continue
		}

		status := s.getOrCreateStatus(name, checkType)

		instances = append(instances, checkInstance{
			check:      chk,
			rrdFile:    rrdFile,
			metricDefs: desc.Metrics,
			status:     status,
		})
		s.logger.Infof("Worker for host %s: initialized %s check", name, checkType)
	}

	return instances
}

// runChecks executes all check instances for a host and updates their status and RRD files.
func (s *Server) runChecks(name string, instances []checkInstance) {
	for _, inst := range instances {
		result := inst.check.Run(context.Background())
		checkType := inst.check.Type()

		inst.status.SetResult(result)

		values := rrdValuesFromResult(result, inst.metricDefs)

		s.logger.Debugf("Worker for host %s [%s]: Updating RRD with values %v.", name, checkType, values)
		lastUpdate, err := inst.rrdFile.SafeUpdate(result.Timestamp, values)
		if err != nil {
			s.logger.Errorf("Worker for host %s [%s]: Failed to update RRD (%v)", name, checkType, err)
		} else {
			inst.status.SetLastUpdate(lastUpdate)
			s.logger.Debugf("Worker for host %s [%s]: RRD update successful.", name, checkType)
		}

		if result.Success {
			s.logger.Infof("Worker for host %s [%s]: check successful", name, checkType)
		} else {
			s.logger.Warningf("Worker for host %s [%s]: check failed (%v)", name, checkType, result.Err)
		}
	}
}

// copyConfig returns a shallow copy of the config map so that factories cannot
// mutate the original host config.
func copyConfig(cfg map[string]any) map[string]any {
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		out[k] = v
	}
	return out
}

// rrdValuesFromResult extracts metric values from a check.Result in the
// order declared by the metric definitions. Returns nil if no metrics are
// present (complete failure, skip RRD update). When only some metrics are
// present (partial failure), missing ones are returned as "U" so rrdtool
// records them as UNKNOWN, allowing surviving targets to continue graphing.
func rrdValuesFromResult(result check.Result, metrics []check.MetricDef) []string {
	if len(metrics) == 0 || len(result.Metrics) == 0 {
		return nil
	}
	vals := make([]string, len(metrics))
	for i, m := range metrics {
		if v, ok := result.Metrics[m.ResultKey]; ok {
			vals[i] = strconv.FormatInt(v, 10)
		} else {
			vals[i] = "U"
		}
	}
	return vals
}
