# Pagination Example

BEWARE: As of 2024-12-13, it looks like Claude does not support pagination yet

This example demonstrates how to use pagination in mcp-golang for listing tools, prompts, and resources.

## Overview

The server is configured with a pagination limit of 2 items per page and registers:
- 5 tools (3 hello tools and 2 bye tools)
- 5 prompts (3 greeting prompts and 2 farewell prompts)
- 5 resources (text files)

## Expected Behavior

1. First page requests will return 2 items and a nextCursor
2. Using the nextCursor will fetch the next 2 items
3. The last page will have no nextCursor
4. Items are returned in alphabetical order by name (for tools and prompts) or URI (for resources)

## Implementation Details

- Uses the `WithPaginationLimit` option to enable pagination
- Demonstrates cursor-based pagination for all three types: tools, prompts, and resources
- Shows how to handle multiple pages of results
- Includes examples of proper error handling
