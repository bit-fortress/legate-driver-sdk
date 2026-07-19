package driver

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestABIProtoContract(t *testing.T) {
	descriptorPath := filepath.Join(t.TempDir(), "driver.pb")
	command := exec.Command("protoc", "--proto_path=testdata", "--descriptor_set_out="+descriptorPath, "abi/v1/driver.proto")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile proto: %v\n%s", err, output)
	}
	encoded, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	var set descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(encoded, &set); err != nil {
		t.Fatal(err)
	}
	file := set.File[0]
	for _, forbidden := range []string{"TextPrepareRequest", "TextPrepareResponse", "CanonicalPayload", "CanonicalEventPayload", "RetryHint", "TextFeature"} {
		for _, message := range file.MessageType {
			if message.GetName() == forbidden {
				t.Fatalf("obsolete message %s remains", forbidden)
			}
		}
		for _, enum := range file.EnumType {
			if enum.GetName() == forbidden {
				t.Fatalf("obsolete enum %s remains", forbidden)
			}
		}
	}
	open := findMessage(t, file, "TextAttemptOpenSuccess")
	if len(open.OneofDecl) != 1 || open.OneofDecl[0].GetName() != "output" {
		t.Fatalf("TextAttemptOpenSuccess.output oneof missing")
	}
}

func findMessage(t *testing.T, file *descriptorpb.FileDescriptorProto, name string) *descriptorpb.DescriptorProto {
	t.Helper()
	for _, message := range file.MessageType {
		if message.GetName() == name {
			return message
		}
	}
	t.Fatalf("message %s missing", name)
	return nil
}
