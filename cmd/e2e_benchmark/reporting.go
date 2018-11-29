package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
)

type reporter interface {
	onTick(start time.Time, states []*workerState)
	onFinish(start time.Time, states []*workerState, err error) error
}

type jsonReporter struct {
	w io.Writer
}

func (r jsonReporter) onTick(start time.Time, states []*workerState) {
}

func (r jsonReporter) onFinish(start time.Time, states []*workerState, err error) error {
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	bytes, err := json.Marshal(&benchmarkReport{
		Start:        start,
		Error:        errText,
		Succeeded:    err == nil,
		WorkerStates: states,
	})
	if err != nil {
		return err
	}
	_, err = r.w.Write(bytes)
	return err
}

type textReporter struct {
	printer     *statusPrinter
	interactive bool
}

func (r textReporter) onTick(start time.Time, states []*workerState) {
	if r.interactive {
		r.printer.print(states, start, true)
	}
}

func (r textReporter) onFinish(start time.Time, states []*workerState, err error) error {
	fmt.Fprintf(r.printer.out, "---\nexecution total time: %v\n", time.Since(start))
	if err != nil {
		fmt.Fprintf(r.printer.out, "benchmark failed: %s\n", err)
		return nil
	}
	fmt.Fprintln(r.printer.out, "workers state:")
	r.printer.print(states, start, false)
	fmt.Fprintln(r.printer.out)
	fmt.Fprintln(r.printer.out, "average timings:")
	timings := computePhaseTimingAverages(start, states)
	for _, t := range timings {
		fmt.Fprintf(r.printer.out, "%s: %v\n", t.name, t.duration)
	}
	fmt.Fprintln(r.printer.out, "benchmark succeeded")
	return nil
}

func buildReporter(out io.Writer, format string) (reporter, error) {
	switch format {
	case "json":
		return jsonReporter{w: out}, nil
	case "report":
		return textReporter{interactive: false, printer: &statusPrinter{out: out}}, nil
	case "interactive":
		return textReporter{interactive: true, printer: &statusPrinter{out: out}}, nil
	}
	return nil, errors.Errorf("unsupported format %s", format)
}

func reportBenchStatus(ctx context.Context, out io.Writer, finishedC chan error, start time.Time, format string, states []*workerState) error {
	ticker := time.NewTicker(time.Second * 1)
	reporter, err := buildReporter(out, format)
	if err != nil {
		return err
	}
	for {
		select {
		case err := <-finishedC:
			if e := reporter.onFinish(start, states, err); e != nil {
				panic(errors.Wrap(e, "failed to print report"))
			}
			return err
		case <-ticker.C:
			reporter.onTick(start, states)
		case <-ctx.Done():
			return errors.New("benchmark timeout")
		}
	}
}
