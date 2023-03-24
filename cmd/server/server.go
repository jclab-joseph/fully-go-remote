package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jc-lab/fully-go-remote/internal/cmd"
	"github.com/jc-lab/fully-go-remote/internal/protocol"
	"github.com/jc-lab/go-tls-psk"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
)

type RunCtx struct {
	process   *os.Process
	cleanupCh chan bool
}

type AppCtx struct {
	flags *cmd.AppFlags

	mutex sync.Mutex
	runs  map[string]*RunCtx
}

func DoServer(flags *cmd.AppFlags) {
	ctx := &AppCtx{
		flags: flags,
		mutex: sync.Mutex{},
		runs:  make(map[string]*RunCtx),
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

func (runCtx *RunCtx) KillIfRunning() {
	oldProcess := runCtx.process
	runCtx.process = nil
	if oldProcess != nil {
		oldProcess.Signal(os.Kill)
		_ = <-runCtx.cleanupCh
	}
}

func (ctx *AppCtx) prepareRunCtx(name string) *RunCtx {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	runCtx := ctx.runs[name]
	if runCtx == nil {
		runCtx = &RunCtx{}
		ctx.runs[name] = runCtx
	} else {
		runCtx.KillIfRunning()
		close(runCtx.cleanupCh)
	}
	runCtx.cleanupCh = make(chan bool, 1)
	return runCtx
}

func (ctx *AppCtx) waitProcessAndDelete(runCtx *RunCtx, sessionId string, f string) {
	state, _ := runCtx.process.Wait()
	runCtx.process = nil
	exitCode := -1
	if state != nil {
		exitCode = state.ExitCode()
	}
	log.Printf("session[%s] exited. code=%d\n", sessionId, exitCode)
	_ = os.Remove(f)
	runCtx.cleanupCh <- true
}

func (ctx *AppCtx) runGoAndDebug(runCtx *RunCtx, dlvArgs []string, f string, exeArgs []string) error {
	sessionId := uuid.NewString()

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
	command.Dir = *ctx.flags.WorkingDirectory

	log.Printf("session[%s] starting\n", sessionId)
	err := command.Start()
	if err != nil {
		return err
	}

	runCtx.process = command.Process

	go ctx.waitProcessAndDelete(runCtx, sessionId, f)

	return nil
}

func (ctx *AppCtx) runJavaAndDebug(runCtx *RunCtx, jvmArgs []string, f string, exeArgs []string) error {
	sessionId := uuid.NewString()

	args := []string{}
	args = append(args, jvmArgs...)
	args = append(args, "-jar", f)
	if len(exeArgs) > 0 {
		args = append(args, exeArgs...)
	}

	command := exec.Command("java", args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Dir = *ctx.flags.WorkingDirectory

	log.Printf("session[%s] starting\n", sessionId)
	err := command.Start()
	if err != nil {
		return err
	}

	runCtx.process = command.Process

	go ctx.waitProcessAndDelete(runCtx, sessionId, f)

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

	var err error

	programType := req.Header.Get(protocol.HEADER_TYPE)
	if programType == "" {
		programType = "go"
	}

	// Kill and delete old file
	runCtx := ctx.prepareRunCtx(programType)

	programName := req.Header.Get(protocol.HEADER_NAME)
	var f *os.File
	tempFile := ""

	extName := path.Ext(programName)
	if extName == "" {
		extName = ".exe"
	} else {
		programName = strings.TrimSuffix(programName, extName)
	}

	if programName != "" {
		for i := 0; i < 10; i++ {
			tempName := fmt.Sprintf("fgr-%s-%d%s", programName, i, extName)
			tempFile = path.Join(os.TempDir(), tempName)
			if f, err = os.Create(tempFile); err != nil {
				log.Println(err)
			}
			if err == nil {
				break
			}
		}
	}
	if f == nil {
		f, err = os.CreateTemp("", "fgr*."+extName)
	}
	log.Print("save to ", f.Name())
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

	runArgs, _ := parseBase64Args(req.Header.Get(protocol.HEADER_ARGS))

	os.Chmod(f.Name(), 0700)
	if programType == "go" {
		dlvArgs, _ := parseBase64Args(req.Header.Get(protocol.HEADER_DLV_ARGS))
		err = ctx.runGoAndDebug(runCtx, dlvArgs, f.Name(), runArgs)
	} else if programType == "java" {
		jvmArgs, _ := parseBase64Args(req.Header.Get(protocol.HEADER_JVM_ARGS))
		fullyJvmArgs := []string{"-agentlib:" + *ctx.flags.JavaAgentLib}
		fullyJvmArgs = append(fullyJvmArgs, jvmArgs...)
		err = ctx.runJavaAndDebug(runCtx, fullyJvmArgs, f.Name(), runArgs)
	} else {
		err = errors.New("unknown type: " + programType)
	}
	if err != nil {
		_ = os.Remove(f.Name())
		log.Println("run failed: ", err)
		httpWriteErr(w, err)
		return
	}

	w.WriteHeader(200)
	_, _ = w.Write([]byte("{}"))
}
