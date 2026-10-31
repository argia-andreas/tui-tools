package main

import (
	"bytes"
	"testing"
)

func TestFixtureBytesAndPositions(t *testing.T) {
	for _, topic := range topics {
		for p := int32(0); p < 3; p++ {
			first := fixture(topic, p, 0)
			if first.Topic != topic || first.Partition != p || !bytes.Contains(first.Value, []byte(`"status":"created"`)) {
				t.Fatal("fixture identity/JSON changed")
			}
			if len(first.Headers) != 4 || first.Headers[0].Key != "trace" || first.Headers[1].Key != "trace" || first.Headers[2].Value != nil || first.Headers[3].Value == nil {
				t.Fatal("ordered duplicate/null-empty headers lost")
			}
			null, empty := fixture(topic, p, 1), fixture(topic, p, 2)
			if null.Key != nil || null.Value != nil || empty.Key == nil || empty.Value == nil || len(empty.Value) != 0 {
				t.Fatal("null and empty conflated")
			}
			if got := fixture(topic, p, 3); !bytes.Equal(got.Value, []byte{0, 255, 128, 10, 27}) {
				t.Fatal("binary fixture changed")
			}
			if !bytes.Contains(fixture(topic, p, 4).Value, []byte{27, ']'}) {
				t.Fatal("hostile terminal fixture removed")
			}
		}
	}
	if !fixture(topics[0], 2, 0).Timestamp.After(fixture(topics[0], 0, 2).Timestamp) {
		t.Fatal("asymmetric partition timestamps lost")
	}
}
