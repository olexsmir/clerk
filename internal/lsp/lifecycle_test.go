package lsp

import (
	"errors"
	"io"
	"os/exec"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestLifecycle_BeforeInitialize(t *testing.T) {
	lct := startLifecycleTest(t)
	// requests answer ServerNotInitialized
	if _, err := lct.srv.Hover(t.Context(), &protocol.HoverParams{}); !codeEq(err, jsonrpc2.ServerNotInitialized) {
		t.Fatalf("request before initialize: want -32002, got %v", err)
	}
	// notifications are dropped without error
	if err := lct.srv.DidOpen(t.Context(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri.File("a.journal"), LanguageID: "journal", Version: 1, Text: "2024-01-01 t\n"},
	}); err != nil {
		t.Fatalf("didOpen before initialize: %v", err)
	}
	// connection is still usable
	if _, err := lct.srv.Initialize(t.Context(), initParams); err != nil {
		t.Fatalf("initialize after dropped notification: %v", err)
	}
}

func TestLifecycle_InitializeMayOnlyBeSentOnce(t *testing.T) {
	lct := startLifecycleTest(t)
	if _, err := lct.srv.Initialize(t.Context(), initParams); err != nil {
		t.Fatalf("first initialize: %v", err)
	}
	if _, err := lct.srv.Initialize(t.Context(), initParams); !codeEq(err, jsonrpc2.InvalidRequest) {
		t.Fatalf("second initialize: want -32600, got %v", err)
	}
}

func TestLifecycle_RequestsServedAfterInitialize(t *testing.T) {
	lct := startLifecycleTest(t)
	if _, err := lct.srv.Initialize(t.Context(), initParams); err != nil {
		t.Fatal(err)
	}
	// a request in the running phase must reach the real handler: an unopened
	// document yields an empty semantic-tokens result, never a lifecycle error.
	if _, err := lct.srv.SemanticTokensFull(t.Context(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("nowhere.journal")},
	}); err != nil {
		t.Fatalf("request after initialize: %v", err)
	}
}

func TestLifecycle_ExitCodes(t *testing.T) {
	for tname, tc := range map[string]struct {
		wantCode       int
		shutdown, init bool
	}{
		"exit before initialize": {wantCode: 1},
		"exit after initialize":  {wantCode: 1, init: true},
		"exit after shutdown":    {wantCode: 0, init: true, shutdown: true},
	} {
		t.Run(tname, func(t *testing.T) {
			lct := startLifecycleTest(t)
			if tc.init {
				if _, err := lct.srv.Initialize(t.Context(), initParams); err != nil {
					t.Fatal(err)
				}
			}
			if tc.shutdown {
				if err := lct.srv.Shutdown(t.Context()); err != nil {
					t.Fatal(err)
				}
				// like gopls, the server trusts its client: a request after
				// shutdown is still served (a nonexistent document yields an
				// empty hover), never a lifecycle error.
				if _, err := lct.srv.Hover(t.Context(), &protocol.HoverParams{}); err != nil {
					t.Fatalf("request after shutdown: %v", err)
				}
			}
			if err := lct.srv.Exit(t.Context()); err != nil {
				t.Fatal(err)
			}
			var ee *ExitError
			if err := <-lct.errc; !errors.As(err, &ee) || ee.Code != tc.wantCode {
				t.Fatalf("want code %d, got %v", tc.wantCode, err)
			}
		})
	}
}

func TestLifecycle_ParentDeathExits(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a subprocess")
	}

	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	pid := int32(child.Process.Pid)
	child.Process.Kill()
	child.Wait() // reap so the immediate signal-0 probe sees ESRCH

	lct := startLifecycleTest(t)

	// initialize in the background: watchParent's immediate probe fires while
	// the request is still being handled, so its response is dropped and the
	// server exits before the call returns.
	go func() { lct.srv.Initialize(t.Context(), &protocol.InitializeParams{ProcessID: &pid}) }()

	select {
	case err := <-lct.errc:
		var ee *ExitError
		if !errors.As(err, &ee) || ee.Code != 1 {
			t.Fatalf("parent death: want code 1, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not exit after parent death")
	}
}

var initParams = &protocol.InitializeParams{
	WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{},
	Capabilities:                     protocol.ClientCapabilities{},
}

func codeEq(err error, code jsonrpc2.Code) bool {
	return errors.Is(err, jsonrpc2.NewError(code, ""))
}

type lifecycleTest struct {
	errc chan error
	srv  protocol.Server
}

func startLifecycleTest(t *testing.T) *lifecycleTest {
	t.Helper()
	s := newServer(t)
	lct := &lifecycleTest{errc: make(chan error, 1)}
	inR, inW := io.Pipe()   // client -> server
	outR, outW := io.Pipe() // server -> client

	go func() { lct.errc <- s.Run(t.Context(), inR, outW) }()
	t.Cleanup(func() {
		inW.Close()
		outR.Close()
	})

	cconn := jsonrpc2.NewConn(jsonrpc2.NewStream(readWriterCloser{Reader: outR, Writer: inW, Closer: outR}), jsonrpc2.WithCodec(lspCodec{}))
	lct.srv = protocol.ServerDispatcher(cconn)
	cconn.Go(t.Context(), protocol.Handlers(protocol.ClientHandler(&captureClient{}, jsonrpc2.MethodNotFoundHandler)))
	return lct
}
