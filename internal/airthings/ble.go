package airthings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

type BLEClient struct {
	address string
	options BLEOptions
	logger  *slog.Logger

	mu          sync.Mutex
	opMu        sync.Mutex
	initialized bool
	adapter     *bluetooth.Adapter
}

type BLEOptions struct {
	TotalTimeout      time.Duration
	DiscoveryTimeout  time.Duration
	ConnectTimeout    time.Duration
	ServicesTimeout   time.Duration
	ReadTimeout       time.Duration
	DisconnectTimeout time.Duration
}

func NewBLEClient(address string, options BLEOptions, logger *slog.Logger) *BLEClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &BLEClient{
		address: address,
		options: normalizeBLEOptions(options),
		logger:  logger.With("component", "ble", "sensor_address", address),
	}
}

func (c *BLEClient) Read(ctx context.Context) (Reading, error) {
	if c.options.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.options.TotalTimeout)
		defer cancel()
	}

	type result struct {
		reading Reading
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		c.opMu.Lock()
		defer c.opMu.Unlock()
		if err := ctx.Err(); err != nil {
			ch <- result{err: err}
			return
		}
		reading, err := c.readOnce(ctx)
		ch <- result{reading: reading, err: err}
	}()

	select {
	case <-ctx.Done():
		return Reading{}, ctx.Err()
	case res := <-ch:
		return res.reading, res.err
	}
}

func (c *BLEClient) readOnce(ctx context.Context) (Reading, error) {
	totalStart := time.Now()
	adapter, err := c.ensureAdapter()
	if err != nil {
		return Reading{}, fmt.Errorf("enable bluetooth adapter: %w", err)
	}

	mac, err := bluetooth.ParseMAC(c.address)
	if err != nil {
		return Reading{}, fmt.Errorf("parse bluetooth address %q: %w", c.address, err)
	}

	address, discoveryDuration, err := c.discover(ctx, adapter, mac)
	if err != nil {
		c.maybeResetAdapter(err)
		return Reading{}, err
	}

	connectStart := time.Now()
	c.logger.Debug("ble connection attempt")
	stopConnectWatch := c.watchPhase("connection", c.options.ConnectTimeout)
	device, err := adapter.Connect(address, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(clampBLEDuration(c.options.ConnectTimeout)),
	})
	stopConnectWatch()
	if err != nil {
		c.maybeResetAdapter(err)
		return Reading{}, fmt.Errorf("connect to airthings sensor %s: %w", c.address, err)
	}
	connectDuration := time.Since(connectStart)
	c.logger.Debug("ble connected", "duration", connectDuration)
	defer func() {
		disconnectStart := time.Now()
		c.logger.Debug("ble disconnect start")
		stopDisconnectWatch := c.watchPhase("disconnect", c.options.DisconnectTimeout)
		if err := device.Disconnect(); err != nil {
			stopDisconnectWatch()
			c.logger.Warn("ble disconnect failed", "error", err, "duration", time.Since(disconnectStart))
			return
		}
		stopDisconnectWatch()
		c.logger.Debug("ble disconnect complete", "duration", time.Since(disconnectStart))
	}()
	if err := ctx.Err(); err != nil {
		return Reading{}, err
	}

	charUUID, err := bluetooth.ParseUUID(MeasurementsCharacteristicUUID)
	if err != nil {
		return Reading{}, fmt.Errorf("parse measurements characteristic UUID: %w", err)
	}

	servicesStart := time.Now()
	c.logger.Debug("ble services resolve start")
	stopServicesWatch := c.watchPhase("services_resolve", c.options.ServicesTimeout)
	services, err := device.DiscoverServices(nil)
	stopServicesWatch()
	if err != nil {
		return Reading{}, fmt.Errorf("discover services: %w", err)
	}
	servicesDuration := time.Since(servicesStart)
	c.logger.Debug("ble services resolved", "service_count", len(services), "duration", servicesDuration)
	if err := ctx.Err(); err != nil {
		return Reading{}, err
	}
	for _, service := range services {
		chars, err := service.DiscoverCharacteristics([]bluetooth.UUID{charUUID})
		if err != nil {
			c.logger.Debug("ble characteristic discovery skipped service", "error", err)
			continue
		}
		if len(chars) == 0 {
			continue
		}
		buf := make([]byte, 32)
		readStart := time.Now()
		c.logger.Debug("ble characteristic read start", "uuid", MeasurementsCharacteristicUUID)
		stopReadWatch := c.watchPhase("characteristic_read", c.options.ReadTimeout)
		n, err := chars[0].Read(buf)
		stopReadWatch()
		if err != nil {
			return Reading{}, fmt.Errorf("read measurements characteristic: %w", err)
		}
		readDuration := time.Since(readStart)
		c.logger.Debug("ble characteristic read complete", "uuid", MeasurementsCharacteristicUUID, "bytes", n, "duration", readDuration)
		reading, err := DecodeWavePlusPayload(buf[:n])
		if err != nil {
			return Reading{}, err
		}
		c.logger.Info(
			"ble sensor read complete",
			"discovery_duration", discoveryDuration,
			"connection_duration", connectDuration,
			"services_duration", servicesDuration,
			"read_duration", readDuration,
			"total_duration", time.Since(totalStart),
		)
		return reading, nil
	}

	return Reading{}, errors.New("measurements characteristic not found")
}

func (c *BLEClient) discover(ctx context.Context, adapter *bluetooth.Adapter, target bluetooth.MAC) (bluetooth.Address, time.Duration, error) {
	found := make(chan bluetooth.Address, 1)
	scanDone := make(chan error, 1)
	targetText := target.String()
	start := time.Now()
	discoveryCtx, cancel := context.WithTimeout(ctx, c.options.DiscoveryTimeout)
	defer cancel()

	c.logger.Debug("ble discovery start")
	go func() {
		err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			discovered := result.Address.String()
			c.logger.Debug(
				"ble advertisement observed",
				"address", discovered,
				"rssi", result.RSSI,
				"local_name", result.LocalName(),
			)
			if !strings.EqualFold(discovered, targetText) {
				return
			}

			c.logger.Info("ble target discovered", "address", discovered, "rssi", result.RSSI, "local_name", result.LocalName(), "duration", time.Since(start))
			select {
			case found <- result.Address:
			default:
			}
			if err := adapter.StopScan(); err != nil {
				c.logger.Warn("ble discovery stop request failed", "error", err)
			} else {
				c.logger.Debug("ble discovery stop requested", "reason", "target_discovered")
			}
		})
		scanDone <- err
	}()

	select {
	case address := <-found:
		if err := c.waitScanStopped(discoveryCtx, scanDone); err != nil {
			return bluetooth.Address{}, 0, err
		}
		return address, time.Since(start), nil
	case err := <-scanDone:
		if err != nil {
			return bluetooth.Address{}, 0, fmt.Errorf("discover airthings sensor %s: %w", c.address, err)
		}
		return bluetooth.Address{}, 0, fmt.Errorf("discover airthings sensor %s: discovery stopped before target was found", c.address)
	case <-discoveryCtx.Done():
		if err := adapter.StopScan(); err != nil {
			c.logger.Warn("ble discovery stop request failed", "reason", "context_done", "error", err)
		} else {
			c.logger.Debug("ble discovery stop requested", "reason", discoveryCtx.Err())
		}
		return bluetooth.Address{}, 0, fmt.Errorf("discover airthings sensor %s: %w", c.address, discoveryCtx.Err())
	}
}

func (c *BLEClient) waitScanStopped(ctx context.Context, scanDone <-chan error) error {
	select {
	case err := <-scanDone:
		if err != nil {
			return fmt.Errorf("stop bluetooth discovery: %w", err)
		}
		c.logger.Debug("ble discovery stopped")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop bluetooth discovery: %w", ctx.Err())
	}
}

func (c *BLEClient) ensureAdapter() (*bluetooth.Adapter, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.adapter == nil {
		c.adapter = bluetooth.DefaultAdapter
	}
	if c.initialized {
		return c.adapter, nil
	}

	start := time.Now()
	c.logger.Info("ble adapter enable start")
	if err := c.adapter.Enable(); err != nil {
		return nil, err
	}
	c.initialized = true
	c.logger.Info("ble adapter enabled", "powered", true, "duration", time.Since(start))
	return c.adapter, nil
}

func (c *BLEClient) maybeResetAdapter(err error) {
	if err == nil || !isAdapterRecoveryError(err) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.initialized {
		c.logger.Warn("ble adapter will be re-enabled on next read", "error", err)
	}
	c.initialized = false
}

func isAdapterRecoveryError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "adapter") ||
		strings.Contains(msg, "not powered") ||
		strings.Contains(msg, "powered off") ||
		strings.Contains(msg, "no such object") ||
		strings.Contains(msg, "unknownobject")
}

func normalizeBLEOptions(options BLEOptions) BLEOptions {
	if options.DiscoveryTimeout <= 0 {
		options.DiscoveryTimeout = 20 * time.Second
	}
	if options.ConnectTimeout <= 0 {
		options.ConnectTimeout = 20 * time.Second
	}
	if options.ServicesTimeout <= 0 {
		options.ServicesTimeout = 15 * time.Second
	}
	if options.ReadTimeout <= 0 {
		options.ReadTimeout = 5 * time.Second
	}
	if options.DisconnectTimeout <= 0 {
		options.DisconnectTimeout = 5 * time.Second
	}
	if options.TotalTimeout <= 0 {
		options.TotalTimeout = options.DiscoveryTimeout + options.ConnectTimeout + options.ServicesTimeout + options.ReadTimeout + options.DisconnectTimeout + 5*time.Second
	}
	return options
}

func (c *BLEClient) watchPhase(phase string, timeout time.Duration) func() {
	if timeout <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			c.logger.Warn("ble phase exceeded configured timeout", "phase", phase, "timeout", timeout)
		}
	}()
	return func() {
		close(done)
	}
}

func clampBLEDuration(duration time.Duration) time.Duration {
	const maxBLEDuration = time.Duration(^uint16(0)) * 625 * time.Microsecond
	if duration > maxBLEDuration {
		return maxBLEDuration
	}
	return duration
}
