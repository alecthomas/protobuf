package main

import (
	"fmt"
	"log"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	pb "google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	pbBytes, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	fds := &pb.FileDescriptorSet{}
	if err := proto.Unmarshal(pbBytes, fds); err != nil {
		log.Fatal(err)
	}

	m := protojson.MarshalOptions{Multiline: true, AllowPartial: true}
	b, err := m.Marshal(fds)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))
}
