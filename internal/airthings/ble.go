package airthings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"tinygo.org/x/bluetooth"
)

type BLEClient struct {
	address string
	timeout time.Duration
}

func NewBLEClient(address string, timeout time.Duration) *BLEClient {
	return &BLEClient{address: address, timeout: timeout}
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
		reading, err := c.readOnce()
		ch <- result{reading: reading, err: err}
	}()

	select {
	case <-ctx.Done():
		return Reading{}, ctx.Err()
	case res := <-ch:
		return res.reading, res.err
	}
}

func (c *BLEClient) readOnce() (Reading, error) {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return Reading{}, fmt.Errorf("enable bluetooth adapter: %w", err)
	}

	mac, err := bluetooth.ParseMAC(c.address)
	if err != nil {
		return Reading{}, fmt.Errorf("parse bluetooth address %q: %w", c.address, err)
	}
	device, err := adapter.Connect(bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: mac}}, bluetooth.ConnectionParams{})
	if err != nil {
		return Reading{}, fmt.Errorf("connect to airthings sensor %s: %w", c.address, err)
	}
	defer device.Disconnect()

	charUUID, err := bluetooth.ParseUUID(MeasurementsCharacteristicUUID)
	if err != nil {
		return Reading{}, fmt.Errorf("parse measurements characteristic UUID: %w", err)
	}

	services, err := device.DiscoverServices(nil)
	if err != nil {
		return Reading{}, fmt.Errorf("discover services: %w", err)
	}
	for _, service := range services {
		chars, err := service.DiscoverCharacteristics([]bluetooth.UUID{charUUID})
		if err != nil {
			continue
		}
		if len(chars) == 0 {
			continue
		}
		buf := make([]byte, 32)
		n, err := chars[0].Read(buf)
		if err != nil {
			return Reading{}, fmt.Errorf("read measurements characteristic: %w", err)
		}
		reading, err := DecodeWavePlusPayload(buf[:n])
		if err != nil {
			return Reading{}, err
		}
		return reading, nil
	}

	return Reading{}, errors.New("measurements characteristic not found")
}
