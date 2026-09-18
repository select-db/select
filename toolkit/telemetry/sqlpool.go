package telemetry

import (
	"context"
	"database/sql"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ObserveSQLPool registers observable gauges for a database/sql pool under
// db.client.connection.*, labelled by pool name.
//
// wait_count and wait_duration are the ones to alert on: they are zero while the
// pool has headroom and non-zero the moment requests start queueing for a
// connection, which is the difference between a slow database and too few
// connections to reach it.
//
// A no-op when telemetry is off or db is nil.
func ObserveSQLPool(name string, db *sql.DB) error {
	if !Enabled() || db == nil {
		return nil
	}
	meter := Meter("github.com/selectDb/toolkit/telemetry")
	poolName := attribute.String("pool.name", name)

	open, err := meter.Int64ObservableUpDownCounter("db.client.connection.count",
		metric.WithDescription("Connections currently in the pool, by state"))
	if err != nil {
		return err
	}
	max, err := meter.Int64ObservableUpDownCounter("db.client.connection.max",
		metric.WithDescription("Configured maximum open connections"))
	if err != nil {
		return err
	}
	waitCount, err := meter.Int64ObservableCounter("db.client.connection.wait.count",
		metric.WithDescription("Connection requests that had to wait for a free connection"))
	if err != nil {
		return err
	}
	waitTime, err := meter.Float64ObservableCounter("db.client.connection.wait.duration",
		metric.WithDescription("Total time blocked waiting for a connection"),
		metric.WithUnit("s"))
	if err != nil {
		return err
	}
	closed, err := meter.Int64ObservableCounter("db.client.connection.closed",
		metric.WithDescription("Connections closed by a pool limit, by reason"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		s := db.Stats()
		o.ObserveInt64(open, int64(s.InUse), metric.WithAttributes(poolName, attribute.String("state", "used")))
		o.ObserveInt64(open, int64(s.Idle), metric.WithAttributes(poolName, attribute.String("state", "idle")))
		o.ObserveInt64(max, int64(s.MaxOpenConnections), metric.WithAttributes(poolName))
		o.ObserveInt64(waitCount, s.WaitCount, metric.WithAttributes(poolName))
		o.ObserveFloat64(waitTime, s.WaitDuration.Seconds(), metric.WithAttributes(poolName))
		o.ObserveInt64(closed, s.MaxIdleClosed, metric.WithAttributes(poolName, attribute.String("reason", "max_idle")))
		o.ObserveInt64(closed, s.MaxLifetimeClosed, metric.WithAttributes(poolName, attribute.String("reason", "max_lifetime")))
		o.ObserveInt64(closed, s.MaxIdleTimeClosed, metric.WithAttributes(poolName, attribute.String("reason", "max_idle_time")))
		return nil
	}, open, max, waitCount, waitTime, closed)
	return err
}
