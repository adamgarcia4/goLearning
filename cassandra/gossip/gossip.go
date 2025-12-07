package gossip

import (
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/proto"

	pbproto "hello/gossip/proto"
)

func Start() {
	fmt.Println("Gossipping")

	test := pbproto.HeartbeatRequest{
		NodeId:    "1",
		Timestamp: time.Now().Unix(),
	}

	bytes, err := proto.Marshal(&test)
	if err != nil {
		log.Fatalf("marshal error: %v", err)
	}

	fmt.Printf("Bytes: %x\n", bytes)

	var test2 pbproto.HeartbeatRequest
	if err := proto.Unmarshal(bytes, &test2); err != nil {
		log.Fatalf("unmarshal error: %v", err)
	}

	fmt.Printf("Test2: %+v\n", &test2)
}
