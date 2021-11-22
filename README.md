# goesl

[FreeSWITCH™](https://freeswitch.com/) Event Socket Library written in Go.

goesl was written base on:

- https://github.com/0x19/goesl
- https://github.com/percipia/eslgo

Before writing this library, I was still working quite smoothly with 0x19/goesl and also used percipia/eslgo but there were a few small problems that both of them could not solve, so I had to adjust tweak it a bit.

## Install

```
go get github.com/luandnh/goesl
```

## Examples

### Inbound ESL Client

```go
package main

import (
	"context"
	"fmt"
	"github.com/luandnh/goesl"
	"time"
    log "github.com/sirupsen/logrus"
)

func main() {
    client, err := goesl.NewClient("127.0.0.1", 8021, "ClueCon", 10)
	if err != nil {
		fmt.Println("Error connecting", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

    raw, err := client.Send("api sofia status")
	if err != nil {
		panic(err)
	}
	log.Info(string(raw.Body))

	time.Sleep(60 * time.Second)

    err = client.SendAsync("event json CHANNEL_CREATE CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE CHANNEL_PROGRESS_MEDIA")
	if err != nil {
		panic(err)
	}

    for {
		msg, err := client.ReadMessage()
		if err != nil {
			if !strings.Contains(err.Error(), "EOF") && err.Error() != "unexpected end of JSON input" {
				log.Error("Error while reading Freeswitch message : ", err)
			}
			continue
		}
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			log.Error("marshal json error : ",err)
		}
		log.Info(string(msgBytes))
	}
	conn.ExitAndClose()
}
```
