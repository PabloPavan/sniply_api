package db

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	dbMetricsEnabled bool
	dbQueryDuration  metric.Float64Histogram
	dbQueryErrors    metric.Int64Counter
	dbTracer         trace.Tracer
)

func InitTelemetry(serviceName string) {
	dbTracer = otel.Tracer(serviceName + "/db")
	meter := otel.Meter(serviceName + "/db")

	var err error
	dbQueryDuration, err = meter.Float64Histogram(
		"sniply_db_query_duration_seconds",
		metric.WithDescription("Latencia das queries no banco de dados"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return
	}

	dbQueryErrors, err = meter.Int64Counter(
		"sniply_db_query_errors_total",
		metric.WithDescription("Erros de queries no banco de dados"),
	)
	if err != nil {
		return
	}

	dbMetricsEnabled = true
}

type instrumentedQueryer struct {
	q Queryer
}

func (i instrumentedQueryer) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	start := time.Now()
	ctx, span, op := startDBSpan(ctx, sql)
	tag, err := i.q.Exec(ctx, sql, arguments...)
	recordDBTelemetry(ctx, span, op, err, time.Since(start))
	return tag, err
}

func (i instrumentedQueryer) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	start := time.Now()
	ctx, span, op := startDBSpan(ctx, sql)
	rows, err := i.q.Query(ctx, sql, args...)
	if err != nil {
		recordDBTelemetry(ctx, span, op, err, time.Since(start))
		return rows, err
	}
	return &instrumentedRows{
		Rows:  rows,
		ctx:   ctx,
		op:    op,
		start: start,
		span:  span,
	}, nil
}

func (i instrumentedQueryer) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	start := time.Now()
	ctx, span, op := startDBSpan(ctx, sql)
	row := i.q.QueryRow(ctx, sql, args...)
	return &instrumentedRow{
		Row:   row,
		ctx:   ctx,
		op:    op,
		start: start,
		span:  span,
	}
}

type instrumentedRows struct {
	pgx.Rows
	ctx   context.Context
	op    string
	start time.Time
	span  trace.Span
	once  sync.Once
}

func (r *instrumentedRows) Close() {
	r.Rows.Close()
	r.record(r.Rows.Err())
}

func (r *instrumentedRows) record(err error) {
	r.once.Do(func() {
		recordDBTelemetry(r.ctx, r.span, r.op, err, time.Since(r.start))
	})
}

type instrumentedRow struct {
	pgx.Row
	ctx   context.Context
	op    string
	start time.Time
	span  trace.Span
	once  sync.Once
}

func (r *instrumentedRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	r.once.Do(func() {
		recordDBTelemetry(r.ctx, r.span, r.op, err, time.Since(r.start))
	})
	return err
}

func startDBSpan(ctx context.Context, sql string) (context.Context, trace.Span, string) {
	op := dbOperation(sql)
	tracer := dbTracer
	if tracer == nil {
		tracer = otel.Tracer("sniply-db")
	}
	ctx, span := tracer.Start(ctx, "DB "+op)
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", op),
	)
	return ctx, span, op
}

func recordDBTelemetry(ctx context.Context, span trace.Span, op string, err error, duration time.Duration) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "db_error")
	}
	span.End()

	if !dbMetricsEnabled {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", op),
		attribute.String("db.status", statusLabel(err)),
	}
	dbQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	if err != nil {
		dbQueryErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

func statusLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func dbOperation(sql string) string {
	fields := strings.Fields(strings.TrimSpace(sql))
	if len(fields) == 0 {
		return "unknown"
	}
	return strings.ToUpper(fields[0])
}
