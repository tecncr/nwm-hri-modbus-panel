package modbus

import (
	"context"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
)

type fakeLink struct {
	response []byte
	request  []byte
	expected int
	err      error
}

func (f *fakeLink) Exchange(_ context.Context, request []byte, expected int) ([]byte, error) {
	f.request, f.expected = append([]byte(nil), request...), expected
	return append([]byte(nil), f.response...), f.err
}

func bytesFromHex(value string) []byte {
	result, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return result
}

func TestManualReadAddressExample(t *testing.T) {
	link := &fakeLink{response: bytesFromHex("1803020018a58c")}
	values, err := NewClient(link).ReadRegisters(context.Background(), 0x18, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []uint16{0x18}) {
		t.Fatalf("values = %#v", values)
	}
	if got := hex.EncodeToString(link.request); got != "1803000000018603" {
		t.Fatalf("request = %s", got)
	}
}

func TestManualWriteExamples(t *testing.T) {
	t.Run("write address", func(t *testing.T) {
		link := &fakeLink{response: bytesFromHex("180600000017cbcd")}
		if err := NewClient(link).WriteSingle(context.Background(), 0x18, 0, 0x17); err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(link.request); got != "180600000017cbcd" {
			t.Fatalf("request = %s", got)
		}
	})
	t.Run("write baselines", func(t *testing.T) {
		link := &fakeLink{response: bytesFromHex("1710001200046339")}
		values := []uint16{0x8776, 0x6554, 0x4332, 0x2110}
		if err := NewClient(link).WriteMultiple(context.Background(), 0x17, 0x12, values); err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(link.request); got != "171000120004088776655443322110d3e3" {
			t.Fatalf("request = %s", got)
		}
	})
}

func TestBroadcastDoesNotWaitForResponse(t *testing.T) {
	link := &fakeLink{}
	if err := NewClient(link).WriteSingle(context.Background(), 0, 0, 0x18); err != nil {
		t.Fatal(err)
	}
	if link.expected != 0 {
		t.Fatalf("expected response length = %d", link.expected)
	}
}

func TestReadRejectsBadCRCAndException(t *testing.T) {
	tests := []struct {
		name string
		resp []byte
		want error
	}{
		{"crc", bytesFromHex("18030200180000"), ErrInvalidCRC},
		{"exception", AppendCRC([]byte{0x18, 0x83, 0x02}), nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(&fakeLink{response: test.resp}).ReadRegisters(context.Background(), 0x18, 0, 1)
			if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	c := NewClient(&fakeLink{})
	if _, err := c.ReadRegisters(context.Background(), 0, 0, 1); err == nil {
		t.Fatal("broadcast read should fail")
	}
	if _, err := c.ReadRegisters(context.Background(), 1, 0, 126); err == nil {
		t.Fatal("oversized read should fail")
	}
	if err := c.WriteMultiple(context.Background(), 1, 0, nil); err == nil {
		t.Fatal("empty multiple write should fail")
	}
}

func TestCRCUsesModbusLowByteFirst(t *testing.T) {
	if got := hex.EncodeToString(AppendCRC(bytesFromHex("180300000001"))); got != "1803000000018603" {
		t.Fatalf("frame = %s", got)
	}
}
