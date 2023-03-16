package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/jc-lab/fully-go-remote/internal/cmd"
	"github.com/jc-lab/fully-go-remote/internal/protocol"
	"github.com/jc-lab/go-tls-psk"
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

func DoExec(flags *cmd.AppFlags) {
	pskConfig := tls.PSKConfig{
		GetIdentity: func() string {
			return "fgor-client"
		},
		GetKey: func(identity string) ([]byte, error) {
			return []byte(*flags.Token), nil
		},
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS10,
		MaxVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_PSK_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA384,
			tls.TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA,
		},
		InsecureSkipVerify: true,
		Extra:              pskConfig,
		Certificates:       []tls.Certificate{tls.Certificate{}},
	}
	_ = tlsConfig

	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				return tls.Dial(network, addr, tlsConfig)
			},
		},
	}

	f, err := os.OpenFile(flags.ExeFile, os.O_RDONLY, 0)
	if err != nil {
		log.Fatal(err)
		return
	}

	resp, err := func() ([]byte, error) {
		defer f.Close()
		req, err := http.NewRequest("POST", *flags.Connect+"/api/upload-and-run", f)
		if err != nil {
			return nil, err
		}

		jsonRunArgs, err := json.Marshal(flags.RunArgs)
		if err != nil {
			return nil, err
		}

		req.Header.Set(protocol.HEADER_ARGS, base64.StdEncoding.EncodeToString(jsonRunArgs))
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		buf, err := streamToByteArray(res.Body)
		if err != nil {
			return nil, err
		}

		if res.StatusCode != 200 {
			log.Println(res.Status)
			return nil, errors.New(string(buf))
		}

		return buf, err
	}()

	if err != nil {
		log.Fatal(err)
		return
	}

	log.Println(string(resp))
}

func streamToByteArray(stream io.Reader) ([]byte, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(stream); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
