package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/jc-lab/fully-go-remote/internal/cmd"
	"github.com/jc-lab/fully-go-remote/internal/protocol"
	"github.com/jc-lab/go-tls-psk"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

type AppCtx struct {
	flags *cmd.AppFlags
}

func DoServer(flags *cmd.AppFlags) {
	ctx := &AppCtx{
		flags: flags,
	}

	pskConfig := tls.PSKConfig{
		GetIdentity: func() string {
			return "fgor-server"
		},
		GetKey: func(identity string) ([]byte, error) {
			if identity == "fgor-client" {
				return []byte(*flags.Token), nil
			}
			return nil, errors.New("INVALID IDENTITY: " + identity)
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

	router := &http.ServeMux{}
	router.HandleFunc("/api/upload-and-run", ctx.uploadAndRun)

	log.Println("Listen tls/" + *flags.ServerListenAddress)
	log.Println("Delve will listen on " + *flags.DelveListenAddress)

	listener, err := tls.Listen("tcp", *flags.ServerListenAddress, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err := http.Serve(listener, router); err != nil {
		log.Fatal(err)
	}
}

func (ctx *AppCtx) runAndDebug(f string, exeArgs []string) error {
	args := []string{"exec", "--headless", "--accept-multiclient", "--api-version=2", "--listen", *ctx.flags.DelveListenAddress, f}
	if len(exeArgs) > 0 {
		args = append(args, exeArgs...)
	}

	cmd := exec.Command("dlv", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Println("cmd error: ", err)
		}
		log.Println("session exited")
		_ = os.Remove(f)
	}()

	return nil
}

func httpWriteJson(w http.ResponseWriter, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func httpWriteErr(w http.ResponseWriter, err error) {
	w.WriteHeader(500)
	if err := httpWriteJson(w, &protocol.ErrorResponse{
		Message: err.Error(),
	}); err != nil {
		log.Println(err)
	}
}

func (ctx *AppCtx) uploadAndRun(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpWriteErr(w, errors.New("invalid method: "+req.Method))
		return
	}

	f, err := os.CreateTemp("", "fgr*.exe")
	if err != nil {
		log.Println("uploadAndRun failed: ", err)
		httpWriteErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	_, err = io.Copy(f, req.Body)
	_ = f.Close()

	if err != nil {
		log.Print("uploadAndRun failed: ", err)
		httpWriteErr(w, err)
		return
	}

	var args []string
	headerArgs := req.Header.Get(protocol.HEADER_ARGS)
	if len(headerArgs) > 0 {
		decoded, err := base64.StdEncoding.DecodeString(headerArgs)
		if err != nil {
			log.Println("base64 decode failed: ", err)
			httpWriteErr(w, err)
			return
		}
		if err = json.Unmarshal(decoded, &args); err != nil {
			log.Println("arguments decode failed: ", err)
			httpWriteErr(w, err)
			return
		}
	}

	os.Chmod(f.Name(), 0700)
	err = ctx.runAndDebug(f.Name(), args)
	if err != nil {
		_ = os.Remove(f.Name())
		log.Println("run failed: ", err)
		httpWriteErr(w, err)
		return
	}

	w.WriteHeader(200)
	_, _ = w.Write([]byte("{}"))
}
