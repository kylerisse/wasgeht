package server

import (
	"context"
	"math/rand"
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

// worker periodically runs all enabled checks against the assigned host
func (s *Server) worker(name string, h *host.Host) {
	defer s.wg.Done()

	// Add a random delay of 1-59 seconds before starting to reduce initial filesystem activity
	startDelay := time.Duration(rand.Intn(59)+1) * time.Second
	s.logger.Infof("Worker for host %s will start in %v", name, startDelay)
	select {
	case <-time.After(startDelay):
		s.logger.Debugf("Worker for host %s starting", name)
	case <-s.done:
		s.logger.Infof("Worker for host %s received shutdown signal before starting", name)
		return
	}

	// Resolve target: use explicit address or fall back to hostname
	target := h.Address
	if target == "" {
		target = name
	}

	// Initialize all enabled checks and their RRD files
	instances := s.initChecks(name, h, target)
	if len(instances) == 0 {
		s.logger.Warningf("Worker for host %s: no checks to run, exiting", name)
		return
	}

	// Run periodic checks every minute
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
func (s *Server) initChecks(name string, h *host.Host, target string) []checkInstance {
	enabledChecks := h.EnabledChecks()
	instances := make([]checkInstance, 0, len(enabledChecks))

	for checkType, cfg := range enabledChecks {
		// Look up the descriptor to learn what metrics this check produces
		desc, err := s.registry.Describe(checkType)
		if err != nil {
			s.logger.Errorf("Worker for host %s: no descriptor for %s check (%v)", name, checkType, err)
			continue
		}

		if len(desc.Metrics) == 0 {
			s.logger.Errorf("Worker for host %s: %s check declares no metrics", name, checkType)
			continue
		}

		// Build the config for the factory: inject target, copy user config
		factoryCfg := buildFactoryConfig(cfg, target)

		chk, err := s.registry.Create(checkType, factoryCfg)
		if err != nil {
			s.logger.Errorf("Worker for host %s: failed to create %s check (%v)", name, checkType, err)
			continue
		}

		// Initialize RRD file for this check.
		// Currently one RRD file per check type using the first metric's DSName.
		// Multi-DS RRD support is future work.
		metric := desc.Metrics[0]
		rrdFile, err := rrd.NewRRD(name, s.rrdDir, s.graphDir, checkType, metric.DSName, metric.Label, metric.Unit, metric.Scale, s.logger)
		if err != nil {
			s.logger.Errorf("Worker for host %s: failed to initialize RRD for %s check (%v)", name, checkType, err)
			continue
		}

		// Get or create the status tracker for this host/check pair
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

		// Update the check status
		inst.status.SetResult(result)

		// Build RRD update from result metrics using the descriptor
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

// buildFactoryConfig creates a config map for a check factory by copying the
// user-provided config and injecting the resolved target address.
func buildFactoryConfig(cfg map[string]any, target string) map[string]any {
	factoryCfg := make(map[string]any, len(cfg)+1)
	for k, v := range cfg {
		factoryCfg[k] = v
	}
	factoryCfg["target"] = target
	return factoryCfg
}

// rrdValuesFromResult extracts metric values from a check.Result in the
// order declared by the metric definitions. Returns an empty slice if the
// check failed or no declared metrics are present.
func rrdValuesFromResult(result check.Result, metrics []check.MetricDef) []float64 {
	if !result.Success {
		return []float64{}
	}
	var vals []float64
	for _, m := range metrics {
		if v, ok := result.Metrics[m.ResultKey]; ok {
			vals = append(vals, v)
		}
	}
	return vals
}
