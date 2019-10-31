package process

import (
	proto "github.com/golang/protobuf/proto"
	"transitrhythm/gtfs/realtime/server/transit_realtime"

	// "google.golang.org/grpc"

	"fmt"
)

var (
	expectedEntityLength = 1
	expectedEntityID     = "1"
	// expectedTripID       = "t0"

)

// Process -
func Process(data []byte, size int) {

	feed := transit_realtime.FeedMessage{}

	err := proto.Unmarshal(data, &feed)
	if err != nil {
		fmt.Printf("Error unmarshaling data: %s\n", err)
	}

	if len(feed.Entity) != expectedEntityLength {
		fmt.Printf("Expected entity length: %d, got: %d\n", expectedEntityLength, len(feed.Entity))
	}

	entity := feed.Entity[0]
	if *entity.Id != expectedEntityID {
		fmt.Printf("Expected entity id: %v, got: %v\n", expectedEntityID, entity.Id)
	}
}

// Transmit -
func Transmit() {

}
