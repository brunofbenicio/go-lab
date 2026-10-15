# pubsub

In-process generic publish/subscribe broker with topics. Publishing never
blocks: a subscriber whose buffer is full simply misses the message, so one
slow consumer cannot stall the rest.

## Usage

```go
b := pubsub.New[Event]()
events, cancel := b.Subscribe("orders", 16)
defer cancel()

n := b.Publish("orders", Event{ID: 1}) // n = subscribers that received it
for e := range events {
    handle(e)
}
```

`cancel` closes the subscriber channel; `Close` closes all of them.

## Tests

```sh
go test -race ./pkg/pubsub/
```
