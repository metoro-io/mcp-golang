package mcp_golang

// There are no arguments to the resource api

type ResourceResponse struct {
	Contents []*EmbeddedResource `json:"contents"`
}

func NewResourceResponse(contents ...*EmbeddedResource) *ResourceResponse {
	return &ResourceResponse{
		Contents: contents,
	}
}
