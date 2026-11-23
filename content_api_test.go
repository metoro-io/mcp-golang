package mcp_golang

import (
	"encoding/json"
	"testing"
)

// A tool result may carry every content type of MCP spec 2025-06-18 plus types
// this library does not know yet. One unknown block must not fail the whole
// response: the client would lose the text blocks that came with it.
func TestToolResponseUnmarshalAllContentTypes(t *testing.T) {
	raw := `{"content":[
		{"type":"text","text":"42 rows"},
		{"type":"resource_link","uri":"file:///results/q1.csv","name":"q1.csv","title":"Query result","description":"full result set","mimeType":"text/csv","size":2048},
		{"type":"image","data":"aW1n","mimeType":"image/png"},
		{"type":"audio","data":"YXVk","mimeType":"audio/wav"},
		{"type":"resource","resource":{"uri":"mem://a","mimeType":"text/plain","text":"embedded text"}},
		{"type":"resource","resource":{"uri":"mem://b","mimeType":"application/pdf","blob":"cGRm"}},
		{"type":"hologram","payload":{"x":1}}
	]}`

	var resp ToolResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(resp.Content) != 7 {
		t.Fatalf("want 7 content blocks, got %d", len(resp.Content))
	}

	c := resp.Content
	if c[0].Type != ContentTypeText || c[0].TextContent.Text != "42 rows" {
		t.Errorf("text block wrong: %+v", c[0])
	}

	link := c[1].ResourceLinkContent
	if c[1].Type != ContentTypeResourceLink || link == nil {
		t.Fatalf("resource_link block wrong: %+v", c[1])
	}
	if link.Uri != "file:///results/q1.csv" || link.Name != "q1.csv" ||
		link.Title == nil || *link.Title != "Query result" ||
		link.Description == nil || *link.Description != "full result set" ||
		link.MimeType == nil || *link.MimeType != "text/csv" ||
		link.Size == nil || *link.Size != 2048 {
		t.Errorf("resource_link fields wrong: %+v", link)
	}

	if c[2].Type != ContentTypeImage || c[2].ImageContent == nil ||
		c[2].ImageContent.Data != "aW1n" || c[2].ImageContent.MimeType != "image/png" {
		t.Errorf("image block wrong: %+v", c[2])
	}
	if c[3].Type != ContentTypeAudio || c[3].AudioContent == nil ||
		c[3].AudioContent.Data != "YXVk" || c[3].AudioContent.MimeType != "audio/wav" {
		t.Errorf("audio block wrong: %+v", c[3])
	}

	er := c[4].EmbeddedResource
	if c[4].Type != ContentTypeEmbeddedResource || er == nil || er.TextResourceContents == nil ||
		er.TextResourceContents.Text != "embedded text" || er.TextResourceContents.Uri != "mem://a" {
		t.Errorf("text resource block wrong: %+v", c[4])
	}
	eb := c[5].EmbeddedResource
	if c[5].Type != ContentTypeEmbeddedResource || eb == nil || eb.BlobResourceContents == nil ||
		eb.BlobResourceContents.Blob != "cGRm" || eb.BlobResourceContents.Uri != "mem://b" {
		t.Errorf("blob resource block wrong: %+v", c[5])
	}

	if c[6].Type != ContentType("hologram") || len(c[6].Raw) == 0 {
		t.Errorf("unknown block must keep its type and raw JSON: %+v", c[6])
	}
}

// An empty text resource is valid content; it used to fail the unmarshal.
func TestEmbeddedResourceEmptyText(t *testing.T) {
	var er EmbeddedResource
	if err := json.Unmarshal([]byte(`{"uri":"mem://empty","text":""}`), &er); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if er.TextResourceContents == nil || er.TextResourceContents.Uri != "mem://empty" {
		t.Errorf("want empty text resource, got %+v", er)
	}
}

// New types must round-trip so a server built on this library can send them.
func TestContentMarshalRoundTrip(t *testing.T) {
	title := "Report"
	in := []*Content{
		NewTextContent("hi"),
		NewImageContent("aW1n", "image/png"),
		NewAudioContent("YXVk", "audio/wav"),
		NewResourceLinkContent("file:///r.txt", "r.txt", &title, nil, nil, nil),
		NewTextResourceContent("mem://a", "t", "text/plain"),
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out []*Content
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal failed: %v (%s)", err, b)
	}
	if out[2].AudioContent == nil || out[2].AudioContent.Data != "YXVk" {
		t.Errorf("audio lost: %s", b)
	}
	if out[3].ResourceLinkContent == nil || out[3].ResourceLinkContent.Uri != "file:///r.txt" ||
		*out[3].ResourceLinkContent.Title != "Report" {
		t.Errorf("resource_link lost: %s", b)
	}
	if out[4].EmbeddedResource == nil || out[4].EmbeddedResource.TextResourceContents.Text != "t" {
		t.Errorf("resource lost: %s", b)
	}
}
