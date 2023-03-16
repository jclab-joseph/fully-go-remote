package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jc-lab/fully-go-remote/internal/cmd"
	"github.com/jc-lab/fully-go-remote/internal/protocol"
	"github.com/jc-lab/go-tls-psk"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

type AppCtx struct {
	flags *cmd.AppFlags

	mutex   sync.Mutex
	process *os.Process
}

func DoServer(flags *cmd.AppFlags) {
	ctx := &AppCtx{
		flags: flags,
		mutex: sync.Mutex{},
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

func (ctx *AppCtx) runAndDebug(dlvArgs []string, f string, exeArgs []string) error {
	sessionId := uuid.NewString()

	func() {
		// Kill old process
		ctx.mutex.Lock()
		defer ctx.mutex.Unlock()

		if ctx.process != nil {
			ctx.process.Signal(os.Kill)
			ctx.process.Wait()
			ctx.process = nil
		}
	}()

	args := []string{"exec", "--headless", "--accept-multiclient", "--api-version=2", "--listen", *ctx.flags.DelveListenAddress}
	args = append(args, dlvArgs...)
	args = append(args, f)
	if len(exeArgs) > 0 {
		args = append(args, "--")
		args = append(args, exeArgs...)
	}

	command := exec.Command("dlv", args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	log.Printf("session[%s] starting\n", sessionId)
	err := command.Start()
	if err != nil {
		return err
	}

	ctx.process = command.Process

	go func() {
		state, _ := command.Process.Wait()
		ctx.mutex.Lock()
		ctx.process = nil
		ctx.mutex.Unlock()
		exitCode := -1
		if state != nil {
			exitCode = state.ExitCode()
		}
		log.Printf("session[%s] exited. code=%d\n", sessionId, exitCode)
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

func parseBase64Args(input string) ([]string, error) {
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return nil, err
	}
	var args []string
	if err = json.Unmarshal(decoded, &args); err != nil {
		return nil, err
	}

	return args, nil
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

	dlvArgs, _ := parseBase64Args(req.Header.Get(protocol.HEADER_DLV_ARGS))
	runArgs, _ := parseBase64Args(req.Header.Get(protocol.HEADER_ARGS))

	os.Chmod(f.Name(), 0700)
	err = ctx.runAndDebug(dlvArgs, f.Name(), runArgs)
	if err != nil {
		_ = os.Remove(f.Name())
		log.Println("run failed: ", err)
		httpWriteErr(w, err)
		return
	}

	w.WriteHeader(200)
	_, _ = w.Write([]byte("{}"))
}
