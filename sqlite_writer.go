package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

func NewWriter(db *sql.DB) *Writer {
	return &Writer{
		db:     db,
		rwChan: make(chan *prompb.WriteRequest),
	}
}

type Writer struct {
	db             *sql.DB
	rwChan         chan *prompb.WriteRequest
	samplesWritten uint64
	mu             sync.Mutex
}

func (w *Writer) HandleRemoteWrite(rw *prompb.WriteRequest) {
	w.rwChan <- rw
}

func (w *Writer) Start() {
	go func() {
		t := time.NewTicker(time.Second)
		for {
			select {
			case <-t.C:
				recordsWritten := atomic.LoadUint64(&w.samplesWritten)
				if recordsWritten > 0 {
					log.Printf("records written: %d", recordsWritten)
					atomic.SwapUint64(&w.samplesWritten, 0)
				}
			}
		}
	}()

	for {
		select {
		case rw := <-w.rwChan:
			w.mu.Lock()
			err := w.saveRemoteWrite(rw)
			if err != nil {
				log.Printf("there was an error saving the remote write: %s", err)
			} else {
				atomic.AddUint64(&w.samplesWritten, 1)
			}

			w.mu.Unlock()
		}
	}
}

func (w *Writer) saveRemoteWrite(rw *prompb.WriteRequest) error {
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}

	for _, ts := range rw.GetTimeseries() {
		err = insertTs(ts, tx)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func insertTs(ts prompb.TimeSeries, tx *sql.Tx) error {
	name := ""
	labels := map[string]string{}
	for _, l := range ts.GetLabels() {
		if l.GetName() == "__name__" {
			name = l.GetValue()
			continue
		}

		labels[l.GetName()] = l.GetValue()
	}

	jsonMap, err := json.Marshal(labels)
	if err != nil {
		return err
	}

	for _, s := range ts.GetSamples() {
		_, err := tx.Exec(
			"INSERT INTO samples(name, dimensions, value, timestamp) VALUES (?, ?, ?, ?)",
			name,
			jsonMap,
			s.V(),
			s.T(),
		)

		if err != nil {
			return err
		}
	}

	return nil
}
