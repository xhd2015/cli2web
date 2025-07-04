package run

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xhd2015/cli2web/config"
	"github.com/xhd2015/cli2web/schema"
	"github.com/xhd2015/less-gen/flags"
	"github.com/xhd2015/less-gen/netport"
	"golang.org/x/term"
)

const help = `
cli2web converts your CLI into web interface

Usage: cli2web --schema schema.json

Options:
  --schema <file>            path to the schema file
  --port <port>              port to serve the web interface on

Other commands:
  cli2web parse-schema <schema.json>    parse schema from json file
  cli2web parse-schema <dir>            parse schema from directory

The schema:
  cli2web example
`

//go:embed static/style.css
var styleCSS string

//go:embed static/script.js
var scriptJS string

func Main(args []string) error {
	return runArgs(args)
}

type RunOptions struct {
	Schema       []byte
	SchemaConfig *config.Schema
	Port         int
}

func Run(opts RunOptions) error {
	return runConfig(opts.Schema, opts.SchemaConfig, opts.Port)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func renderSidebar(cfg *config.Schema) string {
	var sb strings.Builder
	header := "Commands"
	if cfg.Name != "" {
		header = cfg.Name + " Commands"
	}
	sb.WriteString(`<div class="sidebar"><h2>` + html.EscapeString(header) + `</h2><ul class="tree">`)
	var renderCommands func([]*config.Command, string)
	renderCommands = func(commands []*config.Command, prefix string) {
		for _, cmd := range commands {
			path := prefix + "/" + cmd.Name
			sb.WriteString("<li>")
			if len(cmd.Commands) > 0 {
				sb.WriteString(fmt.Sprintf(`<span class="caret" data-path="%s">`+html.EscapeString(cmd.Name)+`</span>`, html.EscapeString(path)))
				sb.WriteString(`<ul class="nested">`)
				renderCommands(cmd.Commands, path)
				sb.WriteString(`</ul>`)
			} else {
				sb.WriteString(fmt.Sprintf(`<a href="%s">%s</a>: %s`,
					html.EscapeString(path), html.EscapeString(cmd.Name), html.EscapeString(cmd.Description)))
			}
			sb.WriteString("</li>")
		}
	}
	renderCommands(cfg.Commands, "")
	sb.WriteString(`</ul></div>`)
	return sb.String()
}

func findCommand(commands []*config.Command, pathParts []string) (*config.Command, bool) {
	if len(pathParts) == 0 {
		return &config.Command{}, false
	}
	for _, cmd := range commands {
		if cmd.Name == pathParts[0] {
			if len(pathParts) == 1 {
				return cmd, true
			}
			return findCommand(cmd.Commands, pathParts[1:])
		}
	}
	return &config.Command{}, false
}

func renderCommand(config *config.Schema, path string) string {
	pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	cmd, ok := findCommand(config.Commands, pathParts)
	if !ok {
		return "Command not found"
	}

	var sb strings.Builder
	commandName := strings.Join(pathParts, " ")
	if config.Name != "" {
		commandName = config.Name + " " + commandName
	}
	sb.WriteString(fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
		html.EscapeString(commandName), html.EscapeString(cmd.Description)))

	sb.WriteString(`<form id="command-form">`)
	// Render Arguments
	if len(cmd.Arguments) > 0 {
		sb.WriteString(`<h2>Arguments</h2>`)
		for _, arg := range cmd.Arguments {
			renderInput(&sb, "option", arg.Type, arg.Name, arg.Description, arg.Multiline, "arg-"+arg.Name, arg.Default)
		}
	}

	// Render Options
	if len(cmd.Options) > 0 {
		sb.WriteString(`<h2>Options</h2>`)
		for _, opt := range cmd.Options {
			renderInput(&sb, "option", opt.Type, opt.Flags, opt.Description, opt.Multiline, opt.Flags, opt.Default)
		}
	}
	sb.WriteString(`<button type="submit">Run</button></form>`)
	sb.WriteString(`<h2>Output</h2><pre id="output"></pre>`)
	sb.WriteString(`<h2>Examples</h2>`)
	sb.WriteString(`<ul>`)
	for _, ex := range cmd.Examples {
		sb.WriteString(fmt.Sprintf(`<li><div style="display:flex;flex-direction:column;"><div>%s</div> <pre><code>%s</code></pre></div></li>`, html.EscapeString(ex.Description), html.EscapeString(ex.Usage)))
	}
	sb.WriteString(`</ul>`)
	return sb.String()
}

func renderInput(sb *strings.Builder, wrapperClass string, inputType string, displayName string, description string, multiline bool, argName string, defaultValue string) {
	var wrapperStyle string
	if multiline {
		wrapperStyle = ` style="display:flex;flex-direction:column;"`
	}
	sb.WriteString(fmt.Sprintf(`<div class="%s"%s>`, wrapperClass, wrapperStyle))
	var descriptionHTML string
	if description != "" {
		descriptionHTML = " (" + html.EscapeString(description) + ")"
	}

	if inputType == "boolean" {
		sb.WriteString(fmt.Sprintf(`<label><input type="checkbox" name="%s"> %s%s</label>`,
			html.EscapeString(displayName), html.EscapeString(argName), descriptionHTML))
	} else {
		sb.WriteString(fmt.Sprintf(`<label>%s%s: </label>`,
			html.EscapeString(displayName), descriptionHTML))

		if multiline {
			sb.WriteString(fmt.Sprintf(`<textarea name="%s">%s</textarea>`,
				html.EscapeString(argName), html.EscapeString(defaultValue)))
		} else {
			sb.WriteString(fmt.Sprintf(`<input type="text" name="%s" value="%s">`,
				html.EscapeString(argName), html.EscapeString(defaultValue)))
		}
	}
	sb.WriteString(`</div>`)
}

func serveWs(w http.ResponseWriter, r *http.Request, config *config.Schema) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Websocket upgrade error:", err)
		return
	}

	_, p, err := conn.ReadMessage()
	if err != nil {
		log.Println("Websocket read message error:", err)
		conn.Close()
		return
	}

	var formData map[string]string
	if err := json.Unmarshal(p, &formData); err != nil {
		log.Println("JSON unmarshal error:", err)
		conn.Close()
		return
	}

	rawPathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws"), "/")
	var pathParts []string
	for _, part := range rawPathParts {
		if part != "" {
			pathParts = append(pathParts, part)
		}
	}

	cmd, ok := findCommand(config.Commands, pathParts)
	if !ok {
		log.Println("Command not found for path:", r.URL.Path, "Parsed parts:", pathParts)
		conn.Close()
		return
	}

	var args []string
	if config.Name != "" {
		args = append(args, config.Name)
	}
	args = append(args, pathParts...)

	// Add arguments
	for _, arg := range cmd.Arguments {
		if value := formData["arg-"+arg.Name]; value != "" {
			args = append(args, value)
		}
	}

	// Add options
	for _, opt := range cmd.Options {
		if opt.Type == "boolean" && formData[opt.Flags] == "on" {
			args = append(args, opt.Flags)
		} else if value, ok := formData[opt.Flags]; ok && value != "" && opt.Type == "string" {
			args = append(args, opt.Flags, value)
		}
	}

	log.Printf("Executing command: %v", args)
	cmdExec := exec.Command(args[0], args[1:]...)

	stdout, err := cmdExec.StdoutPipe()
	if err != nil {
		log.Println("StdoutPipe error:", err)
		conn.Close()
		return
	}
	stderr, err := cmdExec.StderrPipe()
	if err != nil {
		log.Println("StderrPipe error:", err)
		conn.Close()
		return
	}

	log.Println("Starting command execution...")
	if err := cmdExec.Start(); err != nil {
		log.Println("Command start error:", err)
		conn.Close()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2) // Wait for stdout and stderr goroutines

	go streamOutput(conn, stdout, "stdout", &wg)
	go streamOutput(conn, stderr, "stderr", &wg)

	log.Println("Waiting for command to finish...")
	if err := cmdExec.Wait(); err != nil {
		log.Println("Command wait error:", err)
	}
	log.Println("Command finished.")
	wg.Wait() // Wait for both streaming goroutines to finish
	conn.Close()
}

func streamOutput(conn *websocket.Conn, reader io.Reader, streamType string, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		log.Printf("[%s] Sending: %s", streamType, string(line))
		if err := conn.WriteMessage(websocket.TextMessage, append(line, '\n')); err != nil {
			log.Println("Websocket write message error:", err)
			return
		}
	}
	log.Printf("[%s] Stream finished.", streamType)
}

func runArgs(args []string) error {
	var schemaPath string
	var port int

	origArgs := args
	args, err := flags.String("--schema", &schemaPath).
		Bool("--port", &port).
		Help("-h,--help", help).
		Parse(args)
	if err != nil {
		return err
	}

	if len(args) > 0 {
		cmd := origArgs[0]
		cmdArgs := origArgs[1:]
		switch cmd {
		case "parse-schema":
			return handleParseSchema(cmdArgs)
		case "example":
			return handleExample(cmdArgs)
		}
		return fmt.Errorf("unrecognized command: %s", cmd)
	}

	var configData []byte
	if schemaPath == "" {
		if IsStdinTTY() {
			return fmt.Errorf("requires --schema, try `cli2web --help`")
		}
		// read schema from stdin
		configData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading schema from stdin: %v", err)
		}
	} else {
		// Read and parse JSON schema
		configData, err = os.ReadFile(schemaPath)
		if err != nil {
			return fmt.Errorf("reading schema file: %v", err)
		}
	}

	return runConfig(configData, nil, port)
}

func runConfig(configData []byte, schemaConfig *config.Schema, port int) error {
	var config *config.Schema
	if schemaConfig != nil {
		config = schemaConfig
	} else {
		if err := json.Unmarshal(configData, &config); err != nil {
			return fmt.Errorf("parsing schema file: %v", err)
		}
	}

	// Serve static files
	// http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	renderPage := func(title, content string) string {
		return `<!DOCTYPE html><html><head><title>` + html.EscapeString(title) + `</title>` +
			// `<link rel="stylesheet" href="/static/style.css">` +
			`<style>` + styleCSS + `</style>` +
			`</head><body><div class="container">` +
			renderSidebar(config) +
			`<div class="main-content">` + content + `</div></div>` +
			// `<script src="/static/script.js"></script>` +
			`<script>` + scriptJS + `</script>` +
			`</body></html>`
	}

	// Home page and command pages
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// Command page
			pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
			title := strings.Join(pathParts, " ")
			if config.Name != "" {
				title = config.Name + " " + title
			}
			fmt.Fprint(w, renderPage(title, renderCommand(config, r.URL.Path)))
			return
		}

		// Home page
		webTitle := "CLI Web Interface"
		if config.Name != "" {
			webTitle = config.Name + " Web Interface"
		}
		content := `<h1>` + html.EscapeString(webTitle) + `</h1><p>Select a command from the sidebar to begin.</p>`
		fmt.Fprint(w, renderPage(webTitle, content))
	})

	http.HandleFunc("/ws/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r, config)
	})

	if port == 0 {
		listenPort, err := netport.FindListenablePort("", 7777, 100)
		if err != nil {
			return fmt.Errorf("finding listenable port: %v", err)
		}
		port = listenPort
	}

	// Start server
	listenAddr := fmt.Sprintf(":%d", port)
	url := "http://localhost" + listenAddr
	log.Printf("Starting server on %s", url)

	// Automatically open the browser after a short delay to ensure server is ready
	go func() {
		// wait for server to be ready for at most 1s
		netport.IsTCPAddrDialable(net.JoinHostPort("", strconv.Itoa(port)), 1*time.Second)

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		default:
			fmt.Printf("💡 Please manually open: %s\n", url)
			return
		}

		if err := cmd.Run(); err != nil {
			fmt.Printf("💡 Could not automatically open browser. Please manually open: %s\n", url)
		} else {
			fmt.Printf("🌐 Opening browser automatically...\n")
		}
	}()

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		return fmt.Errorf("server error: %v", err)
	}
	return nil
}

func IsStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func handleParseSchema(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("requires schema file or dir")
	}
	file := args[0]
	args = args[1:]
	if len(args) > 0 {
		return fmt.Errorf("unrecognized extra arguments: %s", strings.Join(args, ", "))
	}
	if file == "" {
		return fmt.Errorf("requires schema file or dir")
	}

	stat, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("reading schema file: %v", err)
	}
	if !stat.IsDir() {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading schema file: %v", err)
		}
		var schema *config.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			return fmt.Errorf("parsing schema file: %v", err)
		}
		fmt.Printf("validated\n")
		return nil
	}
	dir := file
	schema, err := schema.ParseSchemaFromDir(dir)
	if err != nil {
		return fmt.Errorf("parsing schema file: %v", err)
	}

	printSchema, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling schema: %v", err)
	}
	fmt.Println(string(printSchema))
	return nil
}

func handleExample(args []string) error {
	fmt.Printf("example not implemented yet")
	return nil
}
