package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type HTTPExample struct {
	BaseURL  string          `json:"baseurl"`
	Request  RequestExample  `json:"request"`
	Response ResponseExample `json:"response"`
}

type RequestExample struct {
	Method   string          `json:"method"`
	Endpoint string          `json:"endpoint"`
	Headers  any             `json:"headers"`
	Body     json.RawMessage `json:"body"`
}

type ResponseExample struct {
	StatusCode int             `json:"status_code"`
	Body       map[string]any  `json:"body"`
	RawBody    json.RawMessage `json:"-"`
}

type FieldConfig struct {
	Name   string      `json:"name"`
	Type   string      `json:"type"`
	OfType string      `json:"ofType,omitempty"`
	Args   []ArgConfig `json:"args,omitempty"`
}

type TypeConfig struct {
	Name   string        `json:"name"`
	Fields []FieldConfig `json:"fields"`
}

type ArgConfig struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	OfType string `json:"ofType,omitempty"`
}

type QueryConfig struct {
	Name   string        `json:"name"`
	Fields []FieldConfig `json:"fields"`
}

type SchemaConfig struct {
	Types []TypeConfig `json:"types"`
	Query QueryConfig  `json:"query"`
}

type ConnectorsConfig struct {
	Connectors []ConnectorConfig `json:"connectors"`
}

type ConnectorConfig struct {
	Field         string         `json:"field"`
	Adapter       string         `json:"adapter"`
	AdapterConfig map[string]any `json:"adapterConfig"`
	KeyPattern    string         `json:"keyPattern"`
	Optional      bool           `json:"optional,omitempty"`
	TimeoutMS     int            `json:"timeoutMs,omitempty"`
	Retries       int            `json:"retries,omitempty"`
}

type ServiceConfig struct {
	Version      string `json:"version"`
	Schema       string `json:"schema"`
	Connectors   string `json:"connectors"`
	Mock         string `json:"mock,omitempty"`
	Route        string `json:"route"`
	Pretty       bool   `json:"pretty"`
	GraphiQL     bool   `json:"graphiql"`
	AllowPartial bool   `json:"allow_partial"`
}

type GeneratorOptions struct {
	Input        string
	OutputDir    string
	Field        string
	TypeName     string
	QueryName    string
	Adapter      string
	BaseURL      string
	Endpoint     string
	Method       string
	KeyPattern   string
	TimeoutMS    int
	Retries      int
	Optional     bool
	Pretty       bool
	GraphiQL     bool
	AllowPartial bool
	TUI          bool

	RedisEndpoint string
	RedisPassword string

	AWSRegion       string
	AWSTable        string
	AWSBucket       string
	AWSAccessKeyID  string
	AWSSecretAccess string

	RDSDriver     string
	RDSDSN        string
	RDSQuery      string
	RDSResultMode string
}

func main() {
	opts := parseFlags()
	example, err := loadExample(opts.Input)
	exitOnErr(err)

	fillDefaults(&opts, example)
	if opts.TUI {
		opts = runTUI(opts, example)
	}

	schema, connectors, service, err := generate(example, opts)
	exitOnErr(err)
	exitOnErr(writeOutputs(opts.OutputDir, schema, connectors, service))

	fmt.Printf("Generated files in %s\n", opts.OutputDir)
	fmt.Printf("- schema.json\n- connectors.json\n- service.json\n")
}

func parseFlags() GeneratorOptions {
	var opts GeneratorOptions
	var tui bool
	flag.StringVar(&opts.Input, "input", "pagamentos.json", "input JSON file with request/response example")
	flag.StringVar(&opts.OutputDir, "out", "generated", "output directory")
	flag.StringVar(&opts.Field, "field", "", "GraphQL query field and connector field")
	flag.StringVar(&opts.TypeName, "type", "", "GraphQL output type name")
	flag.StringVar(&opts.QueryName, "query-name", "Query", "GraphQL root query type name")
	flag.StringVar(&opts.Adapter, "adapter", "", "connector adapter: rest, redis, dynamodb, s3, rds")
	flag.StringVar(&opts.BaseURL, "base-url", "", "REST base URL")
	flag.StringVar(&opts.Endpoint, "endpoint", "", "REST endpoint/key pattern")
	flag.StringVar(&opts.Method, "method", "", "REST method: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
	flag.StringVar(&opts.KeyPattern, "key-pattern", "", "connector keyPattern")
	flag.IntVar(&opts.TimeoutMS, "timeout-ms", 1000, "connector timeout in milliseconds")
	flag.IntVar(&opts.Retries, "retries", 1, "connector retries")
	flag.BoolVar(&opts.Optional, "optional", false, "mark connector as optional")
	flag.BoolVar(&opts.Pretty, "pretty", true, "generate pretty GraphQL responses in service.json")
	flag.BoolVar(&opts.GraphiQL, "graphiql", true, "enable GraphiQL in service.json")
	flag.BoolVar(&opts.AllowPartial, "allow-partial", false, "allow partial GraphQL responses in service.json")
	flag.BoolVar(&tui, "tui", false, "open interactive text UI before generating")

	flag.StringVar(&opts.RedisEndpoint, "redis-endpoint", "localhost:6379", "Redis endpoint")
	flag.StringVar(&opts.RedisPassword, "redis-password", "", "Redis password")
	flag.StringVar(&opts.AWSRegion, "aws-region", "us-east-1", "AWS region for DynamoDB/S3")
	flag.StringVar(&opts.AWSTable, "dynamodb-table", "", "DynamoDB table")
	flag.StringVar(&opts.AWSBucket, "s3-bucket", "", "S3 bucket")
	flag.StringVar(&opts.AWSAccessKeyID, "aws-access-key-id", "", "AWS access key id")
	flag.StringVar(&opts.AWSSecretAccess, "aws-secret-access-key", "", "AWS secret access key")
	flag.StringVar(&opts.RDSDriver, "rds-driver", "postgres", "RDS SQL driver: postgres or mysql")
	flag.StringVar(&opts.RDSDSN, "rds-dsn", "", "RDS DSN")
	flag.StringVar(&opts.RDSQuery, "rds-query", "", "RDS query; may use {args}")
	flag.StringVar(&opts.RDSResultMode, "rds-result-mode", "one", "RDS resultMode: one or many")
	flag.Parse()
	opts.TUI = tui
	return opts
}

func loadExample(path string) (HTTPExample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HTTPExample{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var example HTTPExample
	if err := decoder.Decode(&example); err != nil {
		return HTTPExample{}, err
	}
	if example.Response.Body == nil {
		return HTTPExample{}, errors.New("response.body is required")
	}
	return example, nil
}

func fillDefaults(opts *GeneratorOptions, example HTTPExample) {
	if opts.Adapter == "" {
		opts.Adapter = "rest"
	}
	if opts.Field == "" {
		opts.Field = inferFieldName(example.Request.Endpoint)
	}
	if opts.TypeName == "" {
		opts.TypeName = toExportedName(opts.Field)
	}
	if opts.BaseURL == "" {
		opts.BaseURL = example.BaseURL
	}
	if opts.BaseURL == "" {
		opts.BaseURL = "http://localhost:8080"
	}
	if opts.Endpoint == "" {
		opts.Endpoint = example.Request.Endpoint
	}
	if opts.Method == "" {
		opts.Method = strings.ToUpper(strings.TrimSpace(example.Request.Method))
	}
	if opts.Method == "" {
		opts.Method = http.MethodGet
	}
	if opts.KeyPattern == "" {
		opts.KeyPattern = opts.Endpoint
	}
	if opts.RDSQuery == "" {
		opts.RDSQuery = "select * from " + snakeCase(opts.Field) + " where id = '{id}'"
	}
}

func runTUI(opts GeneratorOptions, example HTTPExample) GeneratorOptions {
	in := bufio.NewReader(os.Stdin)
	fmt.Println("create-schema TUI")
	fmt.Println("=================")
	fmt.Println("Press Enter to keep the value shown in brackets.")
	fmt.Println()

	opts.OutputDir = prompt(in, "Output directory", opts.OutputDir)
	opts.Field = prompt(in, "GraphQL field / connector field", opts.Field)
	opts.TypeName = prompt(in, "GraphQL output type", opts.TypeName)
	opts.QueryName = prompt(in, "GraphQL query type", opts.QueryName)
	opts.Adapter = promptChoice(in, "Adapter", []string{"rest", "redis", "dynamodb", "s3", "rds"}, opts.Adapter)
	opts.KeyPattern = prompt(in, "Key pattern", opts.KeyPattern)
	opts.TimeoutMS = promptInt(in, "Timeout ms", opts.TimeoutMS)
	opts.Retries = promptInt(in, "Retries", opts.Retries)
	opts.Optional = promptBool(in, "Optional connector", opts.Optional)
	opts.Pretty = promptBool(in, "Enable pretty responses", opts.Pretty)
	opts.GraphiQL = promptBool(in, "Enable GraphiQL", opts.GraphiQL)
	opts.AllowPartial = promptBool(in, "Allow partial responses", opts.AllowPartial)

	switch opts.Adapter {
	case "rest":
		opts.BaseURL = prompt(in, "REST baseUrl", opts.BaseURL)
		opts.Endpoint = prompt(in, "REST endpoint", opts.Endpoint)
		opts.Method = promptChoice(in, "REST method", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}, opts.Method)
	case "redis":
		opts.RedisEndpoint = prompt(in, "Redis endpoint", opts.RedisEndpoint)
		opts.RedisPassword = prompt(in, "Redis password", opts.RedisPassword)
	case "dynamodb":
		opts.AWSRegion = prompt(in, "DynamoDB region", opts.AWSRegion)
		opts.AWSTable = prompt(in, "DynamoDB table", opts.AWSTable)
		opts.AWSAccessKeyID = prompt(in, "AWS access key id", opts.AWSAccessKeyID)
		opts.AWSSecretAccess = prompt(in, "AWS secret access key", opts.AWSSecretAccess)
	case "s3":
		opts.AWSRegion = prompt(in, "S3 region", opts.AWSRegion)
		opts.AWSBucket = prompt(in, "S3 bucket", opts.AWSBucket)
		opts.AWSAccessKeyID = prompt(in, "AWS access key id", opts.AWSAccessKeyID)
		opts.AWSSecretAccess = prompt(in, "AWS secret access key", opts.AWSSecretAccess)
	case "rds":
		opts.RDSDriver = promptChoice(in, "RDS driver", []string{"postgres", "mysql"}, opts.RDSDriver)
		opts.RDSDSN = prompt(in, "RDS DSN", opts.RDSDSN)
		opts.RDSQuery = prompt(in, "RDS query", opts.RDSQuery)
		opts.RDSResultMode = promptChoice(in, "RDS result mode", []string{"one", "many"}, opts.RDSResultMode)
	}

	fmt.Println()
	fmt.Printf("Detected response fields: %s\n", strings.Join(sortedKeys(example.Response.Body), ", "))
	fmt.Println()
	return opts
}

func generate(example HTTPExample, opts GeneratorOptions) (SchemaConfig, ConnectorsConfig, ServiceConfig, error) {
	if err := validateOptions(opts); err != nil {
		return SchemaConfig{}, ConnectorsConfig{}, ServiceConfig{}, err
	}
	types := make([]TypeConfig, 0)
	root := inferType(opts.TypeName, example.Response.Body, &types)
	args := argsFromPattern(opts.KeyPattern)
	if len(args) == 0 && opts.Adapter == "rest" {
		args = argsFromPattern(opts.Endpoint)
	}

	schema := SchemaConfig{
		Types: types,
		Query: QueryConfig{
			Name: opts.QueryName,
			Fields: []FieldConfig{
				{
					Name:   opts.Field,
					Type:   "Object",
					OfType: root.Name,
					Args:   args,
				},
			},
		},
	}

	connectors := ConnectorsConfig{
		Connectors: []ConnectorConfig{
			buildConnector(example, opts),
		},
	}

	service := ServiceConfig{
		Version:      "1",
		Schema:       "local:schema.json",
		Connectors:   "local:connectors.json",
		Route:        "/graphql",
		Pretty:       opts.Pretty,
		GraphiQL:     opts.GraphiQL,
		AllowPartial: opts.AllowPartial,
	}
	return schema, connectors, service, nil
}

func validateOptions(opts GeneratorOptions) error {
	switch opts.Adapter {
	case "rest", "redis", "dynamodb", "s3", "rds":
	default:
		return fmt.Errorf("unsupported adapter %q", opts.Adapter)
	}
	if opts.Field == "" {
		return errors.New("field is required")
	}
	if opts.TypeName == "" {
		return errors.New("type is required")
	}
	if opts.KeyPattern == "" {
		return errors.New("key-pattern is required")
	}
	if opts.TimeoutMS < 0 || opts.Retries < 0 {
		return errors.New("timeout-ms and retries cannot be negative")
	}
	if opts.Adapter == "rest" {
		if opts.BaseURL == "" {
			return errors.New("rest base-url is required")
		}
		if !validMethod(opts.Method) {
			return fmt.Errorf("unsupported REST method %q", opts.Method)
		}
	}
	return nil
}

func buildConnector(example HTTPExample, opts GeneratorOptions) ConnectorConfig {
	conn := ConnectorConfig{
		Field:      opts.Field,
		Adapter:    opts.Adapter,
		KeyPattern: opts.KeyPattern,
		Optional:   opts.Optional,
		TimeoutMS:  opts.TimeoutMS,
		Retries:    opts.Retries,
	}
	switch opts.Adapter {
	case "rest":
		conn.AdapterConfig = map[string]any{
			"baseUrl":  opts.BaseURL,
			"endpoint": opts.KeyPattern,
			"method":   strings.ToUpper(opts.Method),
			"headers":  normalizeHeaders(example.Request.Headers),
		}
		if body := stringBody(example.Request.Body); body != "" && strings.ToUpper(opts.Method) != http.MethodGet {
			conn.AdapterConfig["body"] = body
		}
	case "redis":
		conn.AdapterConfig = map[string]any{
			"endpoint": opts.RedisEndpoint,
			"password": opts.RedisPassword,
		}
	case "dynamodb":
		conn.AdapterConfig = map[string]any{
			"region":          opts.AWSRegion,
			"table":           opts.AWSTable,
			"accessKeyId":     opts.AWSAccessKeyID,
			"secretAccessKey": opts.AWSSecretAccess,
		}
	case "s3":
		conn.AdapterConfig = map[string]any{
			"region":          opts.AWSRegion,
			"bucket":          opts.AWSBucket,
			"accessKeyId":     opts.AWSAccessKeyID,
			"secretAccessKey": opts.AWSSecretAccess,
		}
	case "rds":
		conn.AdapterConfig = map[string]any{
			"driverName": opts.RDSDriver,
			"dsn":        opts.RDSDSN,
			"query":      opts.RDSQuery,
			"resultMode": opts.RDSResultMode,
		}
		conn.KeyPattern = opts.RDSQuery
	}
	return conn
}

func inferType(name string, values map[string]any, types *[]TypeConfig) TypeConfig {
	fields := make([]FieldConfig, 0, len(values))
	for _, key := range sortedKeys(values) {
		fields = append(fields, inferField(name, key, values[key], types))
	}
	t := TypeConfig{Name: toExportedName(name), Fields: fields}
	*types = append(*types, t)
	return t
}

func inferField(parent, name string, value any, types *[]TypeConfig) FieldConfig {
	switch v := value.(type) {
	case bool:
		return FieldConfig{Name: name, Type: "Boolean"}
	case string:
		return FieldConfig{Name: name, Type: "String"}
	case json.Number:
		if _, err := v.Int64(); err == nil && !strings.Contains(v.String(), ".") {
			return FieldConfig{Name: name, Type: "Int"}
		}
		return FieldConfig{Name: name, Type: "Float"}
	case float64:
		if v == float64(int64(v)) {
			return FieldConfig{Name: name, Type: "Int"}
		}
		return FieldConfig{Name: name, Type: "Float"}
	case map[string]any:
		child := toExportedName(parent + "_" + name)
		inferType(child, v, types)
		return FieldConfig{Name: name, Type: "Object", OfType: child}
	case []any:
		if len(v) == 0 || v[0] == nil {
			return FieldConfig{Name: name, Type: "List", OfType: "String"}
		}
		item := inferField(parent, singular(name), v[0], types)
		if item.Type == "Object" {
			return FieldConfig{Name: name, Type: "List", OfType: item.OfType}
		}
		return FieldConfig{Name: name, Type: "List", OfType: item.Type}
	case nil:
		return FieldConfig{Name: name, Type: "String"}
	default:
		return FieldConfig{Name: name, Type: "String"}
	}
}

func writeOutputs(dir string, schema SchemaConfig, connectors ConnectorsConfig, service ServiceConfig) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	files := map[string]any{
		"schema.json":     schema,
		"connectors.json": connectors,
		"service.json":    service,
	}
	for name, value := range files {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}

func prompt(in *bufio.Reader, label, current string) string {
	fmt.Printf("%s [%s]: ", label, current)
	value := readLine(in)
	if value == "" {
		return current
	}
	return value
}

func promptChoice(in *bufio.Reader, label string, choices []string, current string) string {
	for {
		fmt.Printf("%s %v [%s]: ", label, choices, current)
		value := readLine(in)
		if value == "" {
			return current
		}
		for _, choice := range choices {
			if strings.EqualFold(value, choice) {
				return choice
			}
		}
		fmt.Println("Invalid choice.")
	}
}

func promptInt(in *bufio.Reader, label string, current int) int {
	for {
		fmt.Printf("%s [%d]: ", label, current)
		value := readLine(in)
		if value == "" {
			return current
		}
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
		fmt.Println("Invalid number.")
	}
}

func promptBool(in *bufio.Reader, label string, current bool) bool {
	for {
		fmt.Printf("%s [%t]: ", label, current)
		value := strings.ToLower(readLine(in))
		if value == "" {
			return current
		}
		switch value {
		case "y", "yes", "s", "sim", "true", "1":
			return true
		case "n", "no", "nao", "não", "false", "0":
			return false
		default:
			fmt.Println("Invalid boolean.")
		}
	}
}

func readLine(in *bufio.Reader) string {
	value, _ := in.ReadString('\n')
	return strings.TrimSpace(value)
}

func inferFieldName(endpoint string) string {
	parts := strings.Split(strings.Trim(endpoint, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.Trim(parts[i], "{} ")
		if part == "" || strings.Contains(part, "-") || hasDigit(part) {
			continue
		}
		return lowerCamel(part)
	}
	return "dataSource"
}

func argsFromPattern(pattern string) []ArgConfig {
	seen := map[string]bool{}
	args := []ArgConfig{}
	for {
		start := strings.Index(pattern, "{")
		end := strings.Index(pattern, "}")
		if start == -1 || end == -1 || end < start {
			break
		}
		name := strings.TrimSpace(pattern[start+1 : end])
		if name != "" && !seen[name] {
			seen[name] = true
			args = append(args, ArgConfig{Name: name, Type: "NonNull", OfType: "String"})
		}
		pattern = pattern[end+1:]
	}
	return args
}

func normalizeHeaders(value any) map[string]string {
	headers := map[string]string{}
	switch v := value.(type) {
	case map[string]any:
		for k, val := range v {
			headers[k] = fmt.Sprintf("%v", val)
		}
	case map[string]string:
		return v
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := fmt.Sprintf("%v", first(m, "name", "key"))
			val := fmt.Sprintf("%v", first(m, "value", "val"))
			if name != "" && val != "" {
				headers[name] = val
			}
		}
	}
	return headers
}

func first(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return ""
}

func stringBody(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err == nil {
		return buf.String()
	}
	return trimmed
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toExportedName(value string) string {
	parts := splitWords(value)
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	if len(parts) == 0 {
		return "Generated"
	}
	return strings.Join(parts, "")
}

func lowerCamel(value string) string {
	exported := toExportedName(value)
	if exported == "" {
		return "dataSource"
	}
	return strings.ToLower(exported[:1]) + exported[1:]
}

func snakeCase(value string) string {
	parts := splitWords(value)
	return strings.ToLower(strings.Join(parts, "_"))
}

func splitWords(value string) []string {
	value = strings.TrimSpace(value)
	parts := []string{}
	var current []rune
	for _, r := range value {
		if r == '_' || r == '-' || r == '/' || r == ' ' {
			if len(current) > 0 {
				parts = append(parts, strings.ToLower(string(current)))
				current = nil
			}
			continue
		}
		if unicode.IsUpper(r) && len(current) > 0 {
			parts = append(parts, strings.ToLower(string(current)))
			current = []rune{unicode.ToLower(r)}
			continue
		}
		current = append(current, unicode.ToLower(r))
	}
	if len(current) > 0 {
		parts = append(parts, strings.ToLower(string(current)))
	}
	return parts
}

func singular(value string) string {
	value = strings.TrimSuffix(value, "s")
	if value == "" {
		return "item"
	}
	return value
}

func hasDigit(value string) bool {
	for _, r := range value {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func validMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func exitOnErr(err error) {
	if err == nil {
		return
	}
	if !errors.Is(err, flag.ErrHelp) && !errors.Is(err, io.EOF) {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(1)
}
