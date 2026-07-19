package v20260719

import (
	"reflect"
	"testing"

	driver "github.com/bootun/legate-driver-sdk/go/driver"
)

func TestMultipartPreservesOrderUnknownPartsAndFiles(t *testing.T) {
	filename := "input.png"
	ref := driver.BlobRef{ID: 7, Size: 3}
	input := &driver.MultipartInput{Parts: []driver.MultipartInputPart{
		{Name: "future", Inline: []byte("one")},
		{Name: "image", Filename: &filename, ContentType: "image/png", Blob: &ref},
		{Name: "prompt", Inline: []byte("draw")},
		{Name: "model", Inline: []byte("public")},
		{Name: "future", Inline: []byte("two")},
	}}
	request, err := DecodeRequest(input)
	if err != nil {
		t.Fatal(err)
	}
	body, err := request.MultipartBody("vendor-image-v2")
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{body.Parts[0].Name, body.Parts[1].Name, body.Parts[2].Name, body.Parts[3].Name, body.Parts[4].Name};
		!reflect.DeepEqual(got, []string{"future", "image", "prompt", "model", "future"}) {
		t.Fatalf("part order = %v", got)
	}
	if string(body.Parts[3].Content.Inline) != "vendor-image-v2" || body.Parts[1].Content.Blob == nil {
		t.Fatalf("multipart body = %#v", body)
	}
}

func TestEditRequiresFileImage(t *testing.T) {
	_, err := DecodeRequest(&driver.MultipartInput{Parts: []driver.MultipartInputPart{
		{Name: "model", Inline: []byte("group")},
		{Name: "prompt", Inline: []byte("draw")},
		{Name: "image", Inline: []byte("not-a-file")},
	}})
	if err == nil {
		t.Fatal("inline image was accepted")
	}
}
