package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

func remoteReadHandler(db *sql.DB) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var buf []byte
		body, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			log.Printf("there was an error reading the body: %s\n", err)
			return
		}

		buf, err = snappy.Decode(buf, body)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			log.Printf("could not snappy decode request: %s\n", err)
			return
		}

		rw := &prompb.ReadRequest{}
		err = proto.Unmarshal(buf, rw)
		if err != nil {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			log.Printf("could not proto unmarhsall request: %s\n", err)
			return
		}

		rr := &prompb.ReadResponse{}
		for _, q := range rw.GetQueries() {
			r, err := getData(db, q)
			if err != nil {
				writer.WriteHeader(http.StatusServiceUnavailable)
				log.Printf("there was an error querying data: %s", err)
				return
			}

			rr.Results = append(rr.Results, r)
		}

		mrr, err := rr.Marshal()
		if err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			log.Printf("could not marshal read response: %s", err)
			return
		}

		log.Printf("there were %d results", len(rr.GetResults()))
		encoded := snappy.Encode(nil, mrr)
		writer.WriteHeader(http.StatusOK)
		writer.Header().Add("Content-Type", "application/x-protobuf")
		_, err = writer.Write(encoded)
		if err != nil {
			log.Fatalf("there was an error writing the response: %s", err)
		}
	}
}

func getData(db *sql.DB, q *prompb.Query) (*prompb.QueryResult, error) {
	sqlQuery, sqlValues, err := constructSqlQuery(q)
	log.Printf("sql query: %s, %#v", sqlQuery, sqlValues)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(sqlQuery, sqlValues...)
	//rows, err := db.Query(sqlQuery)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	qr, err := constructQueryResult(rows)
	if err != nil {
		return nil, err
	}

	return qr, nil
}

func constructQueryResult(rows *sql.Rows) (*prompb.QueryResult, error) {
	qr := &prompb.QueryResult{}
	for rows.Next() {
		ts := &prompb.TimeSeries{}
		var name, dims, value, timestamp string
		var unmarshalledDims map[string]string
		err := rows.Scan(&name, &dims, &value, &timestamp)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(dims), &unmarshalledDims)
		if err != nil {
			return nil, err
		}

		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}

		t, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}

		ts.Samples = append(ts.Samples, prompb.Sample{
			Value:     v,
			Timestamp: t,
		})

		ts.Labels = append(ts.Labels, prompb.Label{
			Name:  "__name__",
			Value: name,
		})

		for k, v := range unmarshalledDims {
			ts.Labels = append(ts.Labels, prompb.Label{
				Name:  k,
				Value: v,
			})
		}

		qr.Timeseries = append(qr.Timeseries, ts)
	}

	log.Printf("number of results: %d", len(qr.Timeseries))
	err := rows.Err()
	if err != nil {
		return nil, err
	}

	return qr, nil
}

// This is awful but it's for getting this thing going
// - awful because I'm building the conditions straight from values
// passed in the query which is ripe for SQL injection but...
// this isn't a project that should be used in prod, so I'll let it go
func constructSqlQuery(q *prompb.Query) (string, []any, error) {
	timeStart := q.GetStartTimestampMs()
	timeEnd := q.GetEndTimestampMs()
	sql := `select * from samples where `
	sqlMatchers := []string{}
	sqlValues := []any{}
	for _, m := range q.GetMatchers() {
		sqlValues = append(sqlValues, m.GetValue())
		name := m.GetName()
		matcher, err := getMatcher(m.GetType())
		if err != nil {
			return "", []any{}, err
		}

		if name == "__name__" {
			sqlMatchers = append(sqlMatchers, "name "+string(matcher)+" ?")
			continue
		} else {
			sqlMatchers = append(sqlMatchers, "json_extract(dimensions, '$."+name+"') "+matcher+" ?")
		}
	}

	sql += strings.Join(sqlMatchers, " and ")
	sql += " and timestamp between ? and ? and value is not null"

	sqlValues = append(sqlValues, timeStart)
	sqlValues = append(sqlValues, timeEnd)

	return sql, sqlValues, nil
}

func getMatcher(m prompb.LabelMatcher_Type) (string, error) {
	switch m {
	case prompb.LabelMatcher_EQ:
		return "=", nil
	case prompb.LabelMatcher_NEQ:
		return "!=", nil
	case prompb.LabelMatcher_RE:
		return "regexp", nil
	case prompb.LabelMatcher_NRE:
		return "not regexp", nil
	default:
		return "", errors.New("unknown type")
	}
}
