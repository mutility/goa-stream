# goa-stream

[![CI](https://github.com/mutility/goa-stream/actions/workflows/build.yaml/badge.svg)](https://github.com/mutility/goa-stream/actions/workflows/build.yaml)

goa-stream is a helper package for implementing streaming support in `goa example` clients. To use it, perform the following steps.

1. Create your design, implement your service
2. Run `goa example $design` for your service
3. Integrate goa-stream into the resulting cmd/*/main.go as described below
4. Run your service
5. Run the example, e.g. `go run ./cmd/*-cli svc method -... < input.jsonl > output.jsonl`

## Integrating goa-stream

In main.go, add the following.

### Import goa-stream

```diff
+import stream "github.com/mutility/goa-stream"
```

### Optional: add flags

```diff
     flag.Usage = usage
+    stream.Flags(flag.CommandLine)
     flag.Parse()
```

Adding flags enables the following:

* `-stream-verbose` without either hardcoding or using general `-verbose`
* specifying files via `-stream-in` or `-stream-out` (otherwise os.Stdin and
  os.Stdout are used; `-` can be used to represent them)
* `-stream-strict` ensures your input doesn't attempt to specify fields that are
  not present in the stream's StreamingPayload

Stream verbose, which can also be specified in the stream.JSONL call, prints the
following characters, followed by the message from any non-EOF error received.

| rune | meaning |
|:-:|-|
|`^`|streaming payload sent|
|`v`|streaming result received|
|`?`|error converting input|
|`$`|EOF from input|
|`#`|EOF from Send or Recv|
|`!`|error from Send (not EOF)|

### Add streaming

```diff
     data, err := endpoint(context.Backgroud(), payload)
     if err != nil {
         fmt.Fprintln(os.Stderr, err.Error())
         os.Exit(1)
     }

+    stream.JSONL(data, debug)

     if data != nil {
```

Streaming reads [JSON Lines](https://jsonlines.org/) format input from
`os.Stdin` or a file named in `-stream-in`. It uses `json.Unmarshal` to convert
each successive object to the goa service type, and send as a streaming payload
to the server.

Passing `debug` here enables stream verbose when `debug` is true. If not using
`stream.Flags`, this parameter is the only place to enable stream verbose.

Simultaneously, streaming receives any streaming results from the server and
prints them out using `json.MarshalIndent`.

### Optional: use a signal.NotifyContext

```diff
+    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
+    defer cancel()

-    data, err := endpoint(context.Backgroud(), payload)
+    data, err := endpoint(ctx, payload)
```

(Note that signal.NotifyContext requires go 1.16, or you can backport its implementation.)

Using a `signal.NotifyContext` offers the ability to interrupt a stream, such as
the infinite stream provided by `yes '{}' | client ...`. Without this, an
infinite stream can have unexpected behavior on interrupt, such as appearing to
complete successfully but the server receives only a subset of the records sent.
