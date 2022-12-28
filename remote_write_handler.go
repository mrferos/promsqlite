package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

func remoteWriterHandler(rwWriter *Writer) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var buf []byte
		body, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte("could not read body"))
			log.Print("could not read body")
			return
		}

		buf, err = snappy.Decode(buf, body)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(fmt.Sprintf("could not decode body: %s", err)))
			log.Print("could not decode body: %s", err)
			return
		}

		rw := &prompb.WriteRequest{}
		err = proto.Unmarshal(buf, rw)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(fmt.Sprintf("could not unmarhsal data to prometheus WriteRequest: %s", err)))
			log.Print("could not unmarhsal data to prometheus WriteRequest: %s", err)
			return
		}

		rwWriter.HandleRemoteWrite(rw)
		writer.WriteHeader(http.StatusOK)
	}
}
