package airthings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

type BLEClient struct {
	address      string
	timeout      time.Duration
	scanStopWait time.Duration
	logger       *slog.Logger
}

func NewBLEClient(address string, timeout time.Duration, logger *slog.Logger) *BLEClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &BLEClient{
		address:      address,
		timeout:      timeout,
		scanStopWait: 5 * time.Second,
		logger:       logger.With("component", "ble", "sensor_address", address),
	}
}

func (c *BLEClient) Read(ctx context.Context) (Reading, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	type result struct {
		reading Reading
		err     error
	}
	ch := make(chan result, 1)
	go func() {
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
	adapter := bluetooth.DefaultAdapter
	c.logger.Info("ble adapter enable start")
	if err := adapter.Enable(); err != nil {
		return Reading{}, fmt.Errorf("enable bluetooth adapter: %w", err)
	}
	c.logger.Info("ble adapter enabled", "powered", true)

	mac, err := bluetooth.ParseMAC(c.address)
	if err != nil {
		return Reading{}, fmt.Errorf("parse bluetooth address %q: %w", c.address, err)
	}

	address, err := c.discover(ctx, adapter, mac)
	if err != nil {
		return Reading{}, err
	}

	c.logger.Info("ble connection attempt")
	device, err := adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return Reading{}, fmt.Errorf("connect to airthings sensor %s: %w", c.address, err)
	}
	c.logger.Info("ble connected")
	defer func() {
		c.logger.Info("ble disconnect start")
		if err := device.Disconnect(); err != nil {
			c.logger.Warn("ble disconnect failed", "error", err)
			return
		}
		c.logger.Info("ble disconnect complete")
	}()

	charUUID, err := bluetooth.ParseUUID(MeasurementsCharacteristicUUID)
	if err != nil {
		return Reading{}, fmt.Errorf("parse measurements characteristic UUID: %w", err)
	}

	c.logger.Info("ble services resolve start")
	services, err := device.DiscoverServices(nil)
	if err != nil {
		return Reading{}, fmt.Errorf("discover services: %w", err)
	}
	c.logger.Info("ble services resolved", "service_count", len(services))
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
		c.logger.Info("ble characteristic read start", "uuid", MeasurementsCharacteristicUUID)
		n, err := chars[0].Read(buf)
		if err != nil {
			return Reading{}, fmt.Errorf("read measurements characteristic: %w", err)
		}
		c.logger.Info("ble characteristic read complete", "uuid", MeasurementsCharacteristicUUID, "bytes", n)
		reading, err := DecodeWavePlusPayload(buf[:n])
		if err != nil {
			return Reading{}, err
		}
		return reading, nil
	}

	return Reading{}, errors.New("measurements characteristic not found")
}

func (c *BLEClient) discover(ctx context.Context, adapter *bluetooth.Adapter, target bluetooth.MAC) (bluetooth.Address, error) {
	found := make(chan bluetooth.Address, 1)
	scanDone := make(chan error, 1)
	targetText := target.String()

	c.logger.Info("ble discovery start")
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

			c.logger.Info("ble target discovered", "address", discovered, "rssi", result.RSSI, "local_name", result.LocalName())
			select {
			case found <- result.Address:
			default:
			}
			if err := adapter.StopScan(); err != nil {
				c.logger.Warn("ble discovery stop request failed", "error", err)
			} else {
				c.logger.Info("ble discovery stop requested", "reason", "target_discovered")
			}
		})
		scanDone <- err
	}()

	select {
	case address := <-found:
		if err := c.waitScanStopped(ctx, scanDone); err != nil {
			return bluetooth.Address{}, err
		}
		return address, nil
	case err := <-scanDone:
		if err != nil {
			return bluetooth.Address{}, fmt.Errorf("discover airthings sensor %s: %w", c.address, err)
		}
		return bluetooth.Address{}, fmt.Errorf("discover airthings sensor %s: discovery stopped before target was found", c.address)
	case <-ctx.Done():
		if err := adapter.StopScan(); err != nil {
			c.logger.Warn("ble discovery stop request failed", "reason", "context_done", "error", err)
		} else {
			c.logger.Info("ble discovery stop requested", "reason", ctx.Err())
		}
		return bluetooth.Address{}, fmt.Errorf("discover airthings sensor %s: %w", c.address, ctx.Err())
	}
}

func (c *BLEClient) waitScanStopped(ctx context.Context, scanDone <-chan error) error {
	stopCtx := ctx
	var cancel context.CancelFunc
	if c.scanStopWait > 0 {
		stopCtx, cancel = context.WithTimeout(ctx, c.scanStopWait)
		defer cancel()
	}

	select {
	case err := <-scanDone:
		if err != nil {
			return fmt.Errorf("stop bluetooth discovery: %w", err)
		}
		c.logger.Info("ble discovery stopped")
		return nil
	case <-stopCtx.Done():
		return fmt.Errorf("stop bluetooth discovery: %w", stopCtx.Err())
	}
}
