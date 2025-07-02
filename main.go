package main

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
	"github.com/xhd2015/less-gen/flags"
	"github.com/xhd2015/less-gen/netport"
	"golang.org/x/term"
)

//go:embed static/style.css
var styleCSS string

//go:embed static/script.js
var scriptJS string

// CLIConfig represents the JSON schema
type CLIConfig struct {
	Root     string    `json:"root"`
	Commands []Command `json:"commands"`
}

type Command struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Commands    []Command  `json:"commands"`
	Examples    []Example  `json:"examples"`
	Options     []Option   `json:"options"`
	Arguments   []Argument `json:"arguments"`
	Output      Output     `json:"output"`
}

type Argument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Multiline   bool   `json:"multiline"`
}

type Example struct {
	Usage       string `json:"usage"`
	Description string `json:"description"`
}

type Option struct {
	Flags       string `json:"flags"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Multiline   bool   `json:"multiline"`
}

type Output struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func renderSidebar(config CLIConfig) string {
	var sb strings.Builder
	header := "Commands"
	if config.Root != "" {
		header = config.Root + " Commands"
	}
	sb.WriteString(`<div class="sidebar"><h2>` + html.EscapeString(header) + `</h2><ul class="tree">`)
	var renderCommands func([]Command, string)
	renderCommands = func(commands []Command, prefix string) {
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
	renderCommands(config.Commands, "")
	sb.WriteString(`</ul></div>`)
	return sb.String()
}

func findCommand(commands []Command, pathParts []string) (Command, bool) {
	if len(pathParts) == 0 {
		return Command{}, false
	}
	for _, cmd := range commands {
		if cmd.Name == pathParts[0] {
			if len(pathParts) == 1 {
				return cmd, true
			}
			return findCommand(cmd.Commands, pathParts[1:])
		}
	}
	return Command{}, false
}

func renderCommand(config CLIConfig, path string) string {
	pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	cmd, ok := findCommand(config.Commands, pathParts)
	if !ok {
		return "Command not found"
	}

	var sb strings.Builder
	commandName := strings.Join(pathParts, " ")
	if config.Root != "" {
		commandName = config.Root + " " + commandName
	}
	sb.WriteString(fmt.Sprintf(`<h1>%s</h1><p>%s</p>`,
		html.EscapeString(commandName), html.EscapeString(cmd.Description)))

	sb.WriteString(`<form id="command-form">`)
	// Render Arguments
	if len(cmd.Arguments) > 0 {
		sb.WriteString(`<h2>Arguments</h2>`)
		for _, arg := range cmd.Arguments {
			sb.WriteString(`<div class="option">`)
			if arg.Multiline {
				sb.WriteString(fmt.Sprintf(`<label>%s (%s): <textarea name="arg-%s">%s</textarea></label>`,
					html.EscapeString(arg.Name), html.EscapeString(arg.Description), html.EscapeString(arg.Name), html.EscapeString(arg.Default)))
			} else {
				sb.WriteString(fmt.Sprintf(`<label>%s (%s): <input type="text" name="arg-%s" value="%s"></label>`,
					html.EscapeString(arg.Name), html.EscapeString(arg.Description), html.EscapeString(arg.Name), html.EscapeString(arg.Default)))
			}
			sb.WriteString(`</div>`)
		}
	}

	// Render Options
	if len(cmd.Options) > 0 {
		sb.WriteString(`<h2>Options</h2>`)
		for _, opt := range cmd.Options {
			sb.WriteString(`<div class="option">`)
			if opt.Type == "boolean" {
				sb.WriteString(fmt.Sprintf(`<label><input type="checkbox" name="%s"> %s (%s)</label>`,
					html.EscapeString(opt.Flags), html.EscapeString(opt.Flags), html.EscapeString(opt.Description)))
			} else if opt.Multiline {
				sb.WriteString(fmt.Sprintf(`<label>%s (%s): <textarea name="%s">%s</textarea></label>`,
					html.EscapeString(opt.Flags), html.EscapeString(opt.Description), html.EscapeString(opt.Flags), html.EscapeString(opt.Default)))
			} else {
				sb.WriteString(fmt.Sprintf(`<label>%s (%s): <input type="text" name="%s" value="%s"></label>`,
					html.EscapeString(opt.Flags), html.EscapeString(opt.Description), html.EscapeString(opt.Flags), html.EscapeString(opt.Default)))
			}
			sb.WriteString(`</div>`)
		}
	}
	sb.WriteString(`<button type="submit">Run</button></form>`)
	sb.WriteString(`<h2>Output</h2><pre id="output"></pre>`)
	sb.WriteString(`<h2>Examples</h2><ul>`)
	for _, ex := range cmd.Examples {
		sb.WriteString(fmt.Sprintf(`<li>%s: %s</li>`, html.EscapeString(ex.Usage), html.EscapeString(ex.Description)))
	}
	sb.WriteString(`</ul>`)
	return sb.String()
}

func serveWs(w http.ResponseWriter, r *http.Request, config CLIConfig) {
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
	if config.Root != "" {
		args = append(args, config.Root)
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

const help = `
cli2web converts your CLI into web interface

Usage: cli2web --schema schema.json

Options:
  --schema <file>        path to the schema file
  --port <port>          port to serve the web interface on

The schema:
  cli2web example
`

func main() {
	var schemaPath string
	var port int
	args, err := flags.String("--schema", &schemaPath).
		Bool("--port", &port).
		Help("-h,--help", help).
		Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
		return
	}

	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "unrecognized extra arguments: %s", strings.Join(args, ", "))
		os.Exit(1)
		return
	}

	var configData []byte

	if schemaPath == "" {
		if IsStdinTTY() {
			fmt.Fprintln(os.Stderr, "requires --schema")
			os.Exit(1)
			return
		}
		// read schema from stdin
		configData, err = io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Error reading schema from stdin: %v", err)
		}
	} else {
		// Read and parse JSON schema
		configData, err = os.ReadFile(schemaPath)
		if err != nil {
			log.Fatalf("Error reading schema file: %v", err)
		}
	}

	var config CLIConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatalf("Error parsing schema file: %v", err)
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
			if config.Root != "" {
				title = config.Root + " " + title
			}
			fmt.Fprint(w, renderPage(title, renderCommand(config, r.URL.Path)))
			return
		}

		// Home page
		webTitle := "CLI Web Interface"
		if config.Root != "" {
			webTitle = config.Root + " Web Interface"
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
			fmt.Fprintf(os.Stderr, "Error finding listenable port: %v\n", err)
			os.Exit(1)
			return
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
		log.Fatalf("Server error: %v", err)
	}
}

func IsStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
