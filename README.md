# yomo-socketio-adaptor

![Edge-Mesh](https://github.com/yomorun/yomo-socketio-adapter/blob/main/edge-mesh.jpg)

![YoMo](https://github.com/yomorun/yomo-socketio-adapter/blob/main/adapter.png)

### Prerequisites

[Install Go](https://golang.org/doc/install)

### 1. Install CLI

```bash
$ go install github.com/yomorun/cli/yomo@latest
```

See [YoMo CLI](https://github.com/yomorun/cli#installing) for details.

### 2. Run YoMo-Zipper

```bash
$ yomo serve -c example/workflow.yaml

Using config file: example/workflow.yaml
ℹ️   Found 0 flows in zipper config
ℹ️   Found 1 sinks in zipper config
ℹ️   Sink 1: Socket.io
ℹ️   Running YoMo Serverless...
2021/06/07 22:33:11 ✅ Listening on localhost:9000
```

### 3. Run Socket.io Server and YoMo Socket.io adapter

```bash
$ REGION=CN MACROMETA_API_KEY=your-macrometa-apikey go run main.go
2021/06/07 22:33:18 Connecting to zipper localhost:9000 ...
2021/06/07 22:33:18 ✅ Connected to zipper localhost:9000
2021/06/07 22:33:18 Starting socket.io server...
```

This example uses [Macrometa KV Datastore](https://www.macrometa.com/products/nosql/kv) to store the global state, you can signup a macrometa and create a K/V collection (`VHQ`) for testing. Then you can generate a new [API key](https://gdn.paas.macrometa.io/#accounts/apikeys) in your Macrometa account and set it to the environment variable `MACROMETA_API_KEY`.

### 4. Run the frontend project

See [yomo-app-gather-town-example](https://github.com/yomorun/yomo-app-gather-town-example) for details.
