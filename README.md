# wssvc-go

Command line arguments when launching server

- `--serve ws://0.0.0.0:8081` the address to start the server. `ws://` and `wss://` are supported
- `--cert --ca --pkey` file path of wss, must be provided when protocol is `wss://`
- `--file-root` specify the root of files for reading, default is `.` (current directory)

```
$ wssvc --serve ws://:8888 --file-root /tmp
```

Server String
- `ws://:8080` is same as `ws://0.0.0.0:8080`, which listens on `*.*:8080` or say `0.0.0.0:8080`, which allows connection to TCP 8080 port from anywhere.
- `ws://localhost:8080` is same as `ws://127.0.0.1:8080`, which listens on `127.0.0.1:8080`, which allows connection to TCP 8080 port from local host. Connections from locations other than localhost is not permitted.
- `wss://` has the same behavior with `ws://`

Client side can control server side behavior using the following commands by sending text messages
- `ping me` asking server to send a Ping message to client
- `pong me` asking server to send a Pong message to client
- `disconnect me` asking server to send a Close message to client
- `start ping me` asking server to start to ping client forever every a few seconds
- `stop ping me` asking server to stop to ping client
- `send me file://test.docx` asking server to send a file under fileroot to client
