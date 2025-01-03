package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"reflect"

	"fmt"
	htmpl "html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitfield/script"
	"github.com/gin-gonic/gin"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/paymentintent"
)

const KB = 1024
const MB = 1024 * KB

//go:embed index.html
var indexHTML []byte

//go:embed complete.html
var completeHTML []byte

//go:embed checkout.css
var checkoutCSS []byte

var menvfile = os.Getenv("MENV")

type FileAsset struct {
	Name  string     //file name
	Data  []byte     // file contents or compiled data
	Mod   time.Time  //modification time of source file
	Built time.Time  //built time or time source file was read
	Mu    sync.Mutex //read / write lock
	Cmp   bool       // should compile the file
	Tiny  bool       // should compile with tinygo
}

var htmlFiles = []FileAsset{
	{Name: "index.html", Data: indexHTML, Built: time.Now()},
	{Name: "complete.html", Data: completeHTML, Built: time.Now()},
	{Name: "public/checkout.css", Data: checkoutCSS, Built: time.Now()},
}

var jsFiles = []FileAsset{
	{Name: runtime.GOROOT() + "/misc/wasm/wasm_exec.js"},
	{Name: strings.TrimSuffix(runtime.GOROOT(), "go") + "tinygo" + "/targets/wasm_exec.js"},
}

var wasmFiles = []FileAsset{
	{Name: "checkout_wasm.go", Cmp: true, Tiny: true},
	// {Name: "complete_wasm.go", Cmp: true, Tiny: true},
}

func readFile(files []FileAsset, i int) (ret []byte) {
	if i <= len(files)-1 && i >= 0 {
		files[i].Mu.Lock()
		ret = files[i].Data
		files[i].Mu.Unlock()
	}
	return ret
}

type item struct {
	Id     string
	Amount int64
	Qty    int64
}

type FlagVars struct {
	Teststripekey bool
	WebPort       int
	StripelivePK  string
	StripeliveSK  string
	StripetestPK  string
	StripetestSK  string
	StripeSK      string
	StripePK      string
}

var f = FlagVars{}

var (
	// Hardcoded array of valid shorthand characters, excluding "h"
	shorthandChars = []rune("abcdefgijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	nextShortIndex = 0 // Index for the next shorthand flag
)

// Get the next available shorthand flag
func getNextShortFlag() string {
	if nextShortIndex >= len(shorthandChars) {
		return ""
	}
	short := shorthandChars[nextShortIndex]
	nextShortIndex++
	return string(short)
}

var a = true
var b = false

func addStringFlag(cmd *cobra.Command, f interface{}, fieldPtr *string, description string) {
	cmd.Flags().StringVarP(fieldPtr, ccc(fieldPtr, f, b), getNextShortFlag(), scriptExecString(fmt.Sprintf("${%s%s}", ccc(fieldPtr, f, a), func(s string) string {
		if s != "" {
			s = "-" + s
		}
		return s
	}(*fieldPtr))), fmt.Sprintf("%s env: %s\033[0m\n\r", description, ccc(fieldPtr, f, a)))
}
func addBoolFlag(cmd *cobra.Command, f interface{}, fieldPtr *bool, description string) {
	cmd.Flags().BoolVarP(fieldPtr, ccc(fieldPtr, f, b), getNextShortFlag(), scriptExecBool(fmt.Sprintf("${%s%s}", ccc(fieldPtr, f, a), func(b bool) string {
		return "-" + strconv.FormatBool(b)
	}(*fieldPtr))), fmt.Sprintf("%s env: %s\033[0m\n\r", description, ccc(fieldPtr, f, a)))
}
func addIntFlag(cmd *cobra.Command, f interface{}, fieldPtr *int, description string) {
	cmd.Flags().IntVarP(fieldPtr, ccc(fieldPtr, f, b), getNextShortFlag(), scriptExecInt(fmt.Sprintf("${%s%s}", ccc(fieldPtr, f, a), func(i int) string {
		return fmt.Sprintf("-%d", i)
	}(*fieldPtr))), fmt.Sprintf("%s env: %s\033[0m\n\r", description, ccc(fieldPtr, f, a)))
}

// change case
func ccc(val interface{}, strct interface{}, upper bool) string {
	v := reflect.ValueOf(strct)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("uc: second argument must be a pointer to a struct")
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.CanAddr() && field.Addr().Interface() == val {
			if upper {
				return strings.ToUpper(v.Type().Field(i).Name)
			}
			return strings.ToLower(v.Type().Field(i).Name)
		}
	}
	return ""
}

func init() {
	stripe.EnableTelemetry = false
	runCmd.Flags().SortFlags = false
	addBoolFlag(runCmd, &f, &f.Teststripekey, "use stripe test api keys instead of live key")
	addStringFlag(runCmd, &f, &f.StripeliveSK, "stripe live api sk")
	addStringFlag(runCmd, &f, &f.StripelivePK, "stripe live api pk")
	addStringFlag(runCmd, &f, &f.StripetestSK, "stripe test api sk")
	addStringFlag(runCmd, &f, &f.StripetestPK, "stripe test api pk")
	addIntFlag(runCmd, &f, &f.WebPort, "port to serve on")
}
func main() {
	_, err = script.Exec(`go help`).Bytes()
	if err != nil {
		log.Fatal("error on golang invocation: ", err)
	}
	Execute()
}

// Execute executes root CLI command.
func Execute() {
	cc.Init(&cc.Config{
		RootCmd:         runCmd,
		Headings:        cc.HiBlue + cc.Bold,
		Commands:        cc.HiBlue + cc.Bold,
		CmdShortDescr:   cc.HiBlue,
		Example:         cc.HiBlue + cc.Italic,
		ExecName:        cc.HiBlue + cc.Bold,
		Flags:           cc.HiBlue + cc.Bold,
		FlagsDescr:      cc.HiBlue,
		NoExtraNewlines: true,
		NoBottomNewline: true,
	})
	if err := runCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
}

var wasmData []byte
var tmpl *htmpl.Template
var err error
var ldFlags string
var runCmd = &cobra.Command{
	Use:   "srv",
	Short: "stripe test server for webassembly",
	Run: func(_ *cobra.Command, _ []string) {
		f.StripeSK = f.StripeliveSK
		f.StripePK = f.StripelivePK
		if f.Teststripekey {
			f.StripeSK = f.StripetestSK
			f.StripePK = f.StripetestPK
		}
		stripe.Key = f.StripeSK
		ldFlags = `-ldflags="-X 'main.stripePK=` + f.StripePK + `'"`
		r1 := gin.New()
		r1.Use(gin.Recovery())
		r1.Use(loggingMiddleware())
		r1.GET("/", func(c *gin.Context) {
			var h htmlTemplateData
			c.Writer.Header().Set("Server", "")
			c.Writer.Header().Set("Content-Type", "text/html;charset=utf-8")
			c.Writer.Header().Set("Transfer-Encoding", "chunked")
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Flush()

			tmpl, err = htmpl.New("index").Parse(string(readFile(htmlFiles, 0)))
			if err != nil {
				msg := fmt.Sprintf("Error parsing html template indexHTML:\n%s\n%v\n", readFile(htmlFiles, 0), err)
				log.Println(msg)
				c.Writer.Write(htmlErr(msg))
				c.Writer.Flush()
				return
			}

			h.WasmExecJs = htmpl.JS(readFile(jsFiles, 0))
			wasmFile := 0
			if wasmFiles[wasmFile].Tiny {
				h.WasmExecJs = htmpl.JS(readFile(jsFiles, 1))
			}

			h.WasmBase64 = base64.StdEncoding.EncodeToString(readFile(wasmFiles, wasmFile))
			tmplData := map[string]interface{}{
				"Page": h,
			}
			var result bytes.Buffer
			err = tmpl.Execute(&result, tmplData)
			if err != nil {
				msg := fmt.Sprintf("Could not execute html template %v\n", err)
				log.Println(msg)
				c.Writer.Write(htmlErr(msg))
				c.Writer.Flush()
				return
			}
			c.Writer.Write(result.Bytes())
			c.Writer.Flush()
		})

		r1.GET("/complete", func(c *gin.Context) {
			var h htmlTemplateData
			c.Writer.Header().Set("Server", "")
			c.Writer.Header().Set("Content-Type", "text/html;charset=utf-8")
			c.Writer.Header().Set("Transfer-Encoding", "chunked")
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Flush()

			tmpl, err = htmpl.New("index").Parse(string(readFile(htmlFiles, 1)))
			if err != nil {
				msg := fmt.Sprintf("Error parsing html template indexHTML:\n%s\n%v\n", readFile(htmlFiles, 1), err)
				log.Println(msg)
				c.Writer.Write(htmlErr(msg))
				c.Writer.Flush()
				return
			}

			h.Css = htmpl.CSS(readFile(htmlFiles, 2))
			h.CssName = "checkout.css"

			h.WasmExecJs = htmpl.JS(readFile(jsFiles, 0))
			wasmFile := 0
			if wasmFiles[wasmFile].Tiny {
				h.WasmExecJs = htmpl.JS(readFile(jsFiles, 1))
			}

			h.WasmBase64 = base64.StdEncoding.EncodeToString(readFile(wasmFiles, wasmFile))
			tmplData := map[string]interface{}{
				"Page": h,
			}
			var result bytes.Buffer
			err = tmpl.Execute(&result, tmplData)
			if err != nil {
				msg := fmt.Sprintf("Could not execute html template %v\n", err)
				log.Println(msg)
				c.Writer.Write(htmlErr(msg))
				c.Writer.Flush()
				return
			}
			c.Writer.Write(result.Bytes())
			c.Writer.Flush()
		})

		r1.GET("/order/:piid", func(c *gin.Context) {
			c.Writer.Header().Set("Server", "")
			c.Writer.Header().Set("Content-Type", "application/json;charset=utf-8")
			c.Writer.Header().Set("Transfer-Encoding", "chunked")
			piid := c.Param("piid")
			order, err := script.File("orders/" + piid + ".json").Bytes()
			if err != nil {
				c.Writer.WriteHeader(http.StatusNotFound)
				c.Writer.Flush()
				return
			}
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Flush()
			c.Writer.Write(order)
			c.Writer.Flush()
		})

		r1.POST("/create-payment-intent", func(c *gin.Context) {
			rawBody, err := c.GetRawData()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
				log.Printf("Failed to read raw request body: %v", err)
				return
			}
			log.Printf("Raw request body: %s", string(rawBody))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
			var req struct {
				Items []item `json:"items"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				log.Printf("Failed to bind JSON: %v", err)
				return
			}
			total := int64(0)
			for _, item := range req.Items {
				total += item.Amount
			}
			params := &stripe.PaymentIntentParams{
				Amount:   stripe.Int64(total),
				Currency: stripe.String(string(stripe.CurrencyUSD)),
				//						        AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
				//						            Enabled: stripe.Bool(false),
				//						        },
			}
			pi, err := paymentintent.New(params)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				log.Printf("Failed to create PaymentIntent: %v", err)
				return
			}
			log.Printf("Created PaymentIntent with ClientSecret: %v", pi.ClientSecret)
			c.JSON(http.StatusOK, struct {
				ClientSecret   string `json:"clientSecret"`
				DpmCheckerLink string `json:"dpmCheckerLink"`
			}{
				ClientSecret:   pi.ClientSecret,
				DpmCheckerLink: fmt.Sprintf("https://dashboard.stripe.com/settings/payment_methods/review?transaction_id=%s", pi.ID),
			})
		})

		r1.POST("/submit-order", func(c *gin.Context) {
			var requestData struct {
				LocalStorageData map[string]interface{} `json:"localStorageData"`
				PaymentIntentId  string                 `json:"paymentIntentId"`
			}

			if err := c.ShouldBindJSON(&requestData); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
				return
			}

			log.Printf("Received order data: %+v", requestData.LocalStorageData)
			log.Printf("Received payment intent ID: %s", requestData.PaymentIntentId)

			paymentIntent, err := paymentintent.Get(requestData.PaymentIntentId, nil)
			if err != nil {
				log.Printf("Error retrieving payment intent: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to verify payment"})
				return
			}

			if paymentIntent.Status != stripe.PaymentIntentStatusSucceeded {
				log.Printf("Payment was not successful, status: %s", paymentIntent.Status)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Payment not successful"})
				return
			}

			ordersDir := "./orders"
			if err := os.MkdirAll(ordersDir, os.ModePerm); err != nil {
				log.Printf("Error creating orders directory: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to save order"})
				return
			}

			filePath := filepath.Join(ordersDir, fmt.Sprintf("%s.json", requestData.PaymentIntentId))

			data, err := json.MarshalIndent(requestData.LocalStorageData, "", "  ")
			if err != nil {
				log.Printf("Error marshalling data: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to save order"})
				return
			}

			if err := os.WriteFile(filePath, data, 0644); err != nil {
				log.Printf("Error writing data to file: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to save order"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Order submitted successfully"})
		})
		wg := new(sync.WaitGroup)
		wg.Add(1)
		go func() {
			fmt.Printf("listening on http://127.0.0.1:%d using gin router\n", f.WebPort)
			r1.Run(fmt.Sprintf(":%d", f.WebPort))
			wg.Done()
		}()
		initJSFiles()
		initFiles()
		go func() {
			for range time.Tick(time.Second) {
				initHTMLFiles()
				initFiles()
			}
		}()
		wg.Wait()
	},
}

func initJSFiles() {
	for i, _ := range jsFiles {
		jsFiles[i].Data, err = script.File(jsFiles[i].Name).Bytes()
		if err != nil {
			log.Fatal("Could not read file: ", jsFiles[i].Name, jsFiles[i].Data, err)
		}
	}
}

func initHTMLFiles() {
	for i, _ := range htmlFiles {
		fileInfo, err := os.Stat(htmlFiles[i].Name)
		if err != nil {
			log.Printf("Error accessing file %s: %v", htmlFiles[i].Name, err)
			htmlFiles[i].Mod = time.Now()
			continue
		}
		htmlFiles[i].Mod = fileInfo.ModTime()
		if htmlFiles[i].Mod.After(htmlFiles[i].Built) || htmlFiles[i].Data == nil {
			log.Println("reading html file", htmlFiles[i].Name)
			htmlFiles[i].Mu.Lock()
			htmlFiles[i].Data, err = script.File(htmlFiles[i].Name).Bytes()
			if err != nil {
				log.Printf("Failed to read html file %s:\n%s\n%v\n", htmlFiles[i].Name, string(htmlFiles[i].Data), err)
				continue
			}
			log.Println("read html file", htmlFiles[i].Name)
			htmlFiles[i].Built = htmlFiles[i].Mod
			htmlFiles[i].Mu.Unlock()
		}
	}
}

func initFiles() {
	for i, _ := range wasmFiles {
		fileInfo, err := os.Stat(wasmFiles[i].Name)
		if err != nil {
			log.Printf("Error accessing file %s: %v", wasmFiles[i].Name, err)
			wasmFiles[i].Mod = time.Now()
			continue
		}
		wasmFiles[i].Mod = fileInfo.ModTime()
		if (wasmFiles[i].Mod.After(wasmFiles[i].Built) || wasmFiles[i].Data == nil) && wasmFiles[i].Cmp {
			wasmFiles[i].Mu.Lock()
			var compileCmd string
			startTime := time.Now()
			buildWith := "go build"
			if wasmFiles[i].Tiny {
				buildWith = "tinygo build -target=wasm --no-debug"
			}
			log.Println("compiling wasm binary", func() string {
				if wasmFiles[i].Tiny {
					return "with tinygo"
				}
				return ""
			}())
			compileCmd = fmt.Sprintf(`bash -c 'GOOS=js GOARCH=wasm %s %s -o /dev/stdout %s'`, buildWith, ldFlags, wasmFiles[i].Name)
			fmt.Println(compileCmd)
			wasmFiles[i].Data, err = script.Exec(compileCmd).Bytes()
			if err != nil {
				log.Printf("Failed to compile wasm file %s:\n%s\n%v\n", wasmFiles[i].Name, string(wasmFiles[i].Data), err)
			} else {
				log.Printf("wasm binary size: %s\n", func() string {
					binarySize := len(wasmFiles[i].Data)
					if binarySize >= MB {
						return fmt.Sprintf("%.2f MB", float64(binarySize)/MB)
					} else if binarySize >= KB {
						return fmt.Sprintf("%.2f KB", float64(binarySize)/KB)
					}
					return fmt.Sprintf("%d bytes", binarySize)
				}())
				log.Printf("compile time: %v\n", time.Since(startTime))
			}
			wasmFiles[i].Built = wasmFiles[i].Mod
			wasmFiles[i].Mu.Unlock()
		}
	}
}

type GinHandler struct{ Router *gin.Engine }

func (h *GinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.Router.ServeHTTP(w, r) }
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCodeBackgroundColor := getBackgroundColor(statusCode)
		methodColor := getMethodColor(method)
		fmt.Printf("[GIN] | %s |%s %3d %s| %13v | %15s | %72s |%s %-7s %s %s\n", time.Now().Format("2006/01/02 - 15:04:05"), statusCodeBackgroundColor, statusCode, resetColor(), latency, c.ClientIP(), c.Request.RemoteAddr, methodColor, method, resetColor(), path)
	}
}

func htmlErr(msg string) []byte {
	return []byte(fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Error</title></head><body style='background-color: black; color: white;'><div>%s</div></body></html>`, strings.ReplaceAll(msg, "\n", "<br>")))
}

func getBackgroundColor(statusCode int) string {
	switch {
	case statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices:
		return green
	case statusCode >= http.StatusMultipleChoices && statusCode < http.StatusBadRequest:
		return white
	case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
		return yellow
	default:
		return red
	}
}
func getMethodColor(method string) string {
	switch method {
	case http.MethodGet:
		return blue
	case http.MethodPost:
		return cyan
	case http.MethodPut:
		return yellow
	case http.MethodDelete:
		return red
	case http.MethodPatch:
		return green
	case http.MethodHead:
		return magenta
	case http.MethodOptions:
		return white
	default:
		return reset
	}
}
func resetColor() string { return reset }

type consoleColorModeValue int

var consoleColorMode = autoColor

const (
	autoColor consoleColorModeValue = iota
	disableColor
	forceColor
)
const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

type htmlTemplateData struct {
	Title      string
	WasmExecJs htmpl.JS
	WasmBase64 string
	Css        htmpl.CSS
	CssName    string
	// Css        []htmpl.CSS
	// CssName    []string
	// Script     []htmpl.JS
	// ScriptName []string
}

func scriptExecString(s string) string {
	z, err := script.Exec(fmt.Sprintf(`bash -c 'MENV=%s ; if [[ $MENV != "" ]] && [[ -f $MENV ]] ; then source $MENV ; fi ; printf "%s"'`, menvfile, s)).String()
	if err == nil {
		return strings.TrimSpace(z)
	}
	return ""
}

func scriptExecBool(s string) bool {
	z, err := script.Exec(fmt.Sprintf(`bash -c 'MENV=%s ; if [[ $MENV != "" ]] && [[ -f $MENV ]] ; then source $MENV ; fi ; printf "%s"'`, menvfile, s)).String()
	if err == nil {
		b, err := strconv.ParseBool(z)
		if err == nil {
			return b
		}
	}
	return false
}

func scriptExecArray(s string) string {
	y, err := script.Exec(fmt.Sprintf(`bash -c 'MENV=%s ; if [[ $MENV != "" ]] && [[ -f $MENV ]] ; then source $MENV ; fi ; for _i in %s ; do echo "$_i" ; done'`, menvfile, s)).Slice()
	if err == nil {
		return strings.Join(y, ",")
	}
	return ""
}

func scriptExecInt(s string) int {
	z, err := script.Exec(fmt.Sprintf(`bash -c 'MENV=%s ; if [[ $MENV != "" ]] && [[ -f $MENV ]] ; then source $MENV ; fi ; printf "%s"'`, menvfile, s)).String()
	if err == nil {
		if z == "" {
			return 0
		}
		i, err := strconv.Atoi(z)
		if err == nil {
			return i
		}
	}
	return 0
}

const help = "Usage:\r\n" +
	"  {{.UseLine}}{{if .HasAvailableSubCommands}}{{end}} {{if gt (len .Aliases) 0}}\r\n\r\n" +
	"{{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}\r\n\r\n" +
	"Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand)}}\r\n  " +
	"{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}\r\n\r\n" +
	"Flags:\r\n" +
	"{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}\r\n\r\n" +
	"Global Flags:\r\n" +
	"{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}\r\n\r\n"
