# Schema Directory

This directory contains the JSON schema for MCP types as taken from the [official MCP specification repo](https://github.com/modelcontextprotocol/specification). E.g. for the latest version, see [schema/mcp-schema-2024-10-07.json](schema/mcp-schema-2024-10-07.json). Taken from [here](https://github.com/modelcontextprotocol/specification/blob/bb5fdd282a4d0793822a569f573ebc36804d38f8/schema/schema.json).

This schema is used to generate the types we use in this library and means that we adhere strictly to the spec.

## Generating types from a new schema

We use the [go-jsonschema](https://github.com/atombender/go-jsonschema) library to generate types from the schema. To update the types, run `go generate ./...` in this directory.